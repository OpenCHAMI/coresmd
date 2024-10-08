package api_cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coredhcp/coredhcp/handler"
	"github.com/coredhcp/coredhcp/logger"
	"github.com/coredhcp/coredhcp/plugins"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

var log = logger.GetLogger("plugins/api_cache")

// Plugin wraps the api_cache plugin information.
var Plugin = plugins.Plugin{
	Name:   "api_cache",
	Setup6: setup6,
	Setup4: setup4,
}

var (
	baseURL     string
	accessToken string
	bootScript  string

	cache      Cache
	cacheMutex sync.RWMutex
)

type Cache struct {
	EthernetInterfaces []EthernetInterface
	Components         []Component
	LastUpdated        time.Time
}

type EthernetInterface struct {
	MACAddress  string `json:"MACAddress"`
	ComponentID string `json:"ComponentID"`
	Type        string `json:"Type"`
	Description string `json:"Description"`
	IPAddresses []struct {
		IPAddress string `json:"IPAddress"`
	} `json:"IPAddresses"`
}

type Component struct {
	ID   string `json:"ID"`
	NID  string `json:"NID"`
	Type string `json:"Type"`
}

func setup6(args ...string) (handler.Handler6, error) {
	return nil, errors.New("api_cache does not support DHCPv6")
}

func setup4(args ...string) (handler.Handler4, error) {
	if len(args) != 3 {
		return nil, errors.New("need base URL, access token, and boot script URL as arguments")
	}
	baseURL = args[0]
	accessToken = args[1]
	bootScript = args[2]

	err := refreshCache()
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			err := refreshCache()
			if err != nil {
				log.Printf("Error refreshing cache: %v", err)
			}
		}
	}()

	log.Infof("api_cache plugin initialized with baseURL: %s", baseURL)
	return Handler4, nil
}

func refreshCache() error {
	eiData, err := getAPI(fmt.Sprintf("%s/hsm/v2/Inventory/EthernetInterfaces", baseURL))
	if err != nil {
		return err
	}
	componentData, err := getAPI(fmt.Sprintf("%s/hsm/v2/State/Components", baseURL))
	if err != nil {
		return err
	}

	var ethernetInterfaces []EthernetInterface
	err = json.Unmarshal(eiData, &ethernetInterfaces)
	if err != nil {
		return err
	}

	var components struct {
		Components []Component `json:"Components"`
	}
	err = json.Unmarshal(componentData, &components)
	if err != nil {
		return err
	}

	cacheMutex.Lock()
	cache = Cache{
		EthernetInterfaces: ethernetInterfaces,
		Components:         components.Components,
		LastUpdated:        time.Now(),
	}
	cacheMutex.Unlock()
	log.Printf("Cache updated with %d EthernetInterfaces and %d Components", len(ethernetInterfaces), len(components.Components))
	return nil
}

func getAPI(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Handler4 handles DHCPv4 packets for the api_cache plugin
func Handler4(req, resp *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	for _, ei := range cache.EthernetInterfaces {
		if ei.MACAddress == req.ClientHWAddr.String() {
			for _, component := range cache.Components {
				if component.ID == ei.ComponentID {
					resp.YourIPAddr = net.ParseIP(ei.IPAddresses[0].IPAddress)
					resp.Options.Update(dhcpv4.OptBootFileName(fmt.Sprintf("%s/boot/v1/bootscript?mac=%s", bootScript, ei.MACAddress)))
					return resp, false
				}
			}
		}
	}
	return resp, true
}
