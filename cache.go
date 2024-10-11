package coresmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Cache struct {
	Client      *SmdClient
	Duration    time.Duration
	LastUpdated time.Time
	Mutex       sync.RWMutex

	RedfishEndpoints   map[string]RedfishEndpoint
	EthernetInterfaces map[string]EthernetInterface
	Components         map[string]Component
}

func NewCache(duration string, client *SmdClient) (*Cache, error) {
	cacheDuration, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache duration: %w", err)
	}
	if client == nil {
		return nil, errors.New("new Client needs non-nil SmdClient")
	}

	c := &Cache{
		Client:   client,
		Duration: cacheDuration,
	}

	return c, nil
}

func (c *Cache) Refresh() error {
	log.Info("initiating cache refresh")

	if c == nil {
		return fmt.Errorf("cache is nil")
	}

	// Fetch data
	log.Debug("fetching EthernetInterfaces")
	ethIfaceData, err := c.Client.APIGet("/hsm/v2/Inventory/EthernetInterfaces")
	if err != nil {
		return fmt.Errorf("failed to fetch EthernetInterfaces from SMD: %w", err)
	}
	log.Debug("EthernetInterfaces: " + string(ethIfaceData))
	log.Debug("fetching Components")
	compsData, err := c.Client.APIGet("/hsm/v2/State/Components")
	if err != nil {
		return fmt.Errorf("failed to fetch Components from SMD: %w", err)
	}
	log.Debug("Components: " + string(compsData))
	log.Debug("fetching RedfishEndpoints")
	rfeData, err := c.Client.APIGetToken("/hsm/v2/Inventory/RedfishEndpoints")
	if err != nil {
		return fmt.Errorf("failed to fetch RedfishEndpoints from SMD: %w", err)
	}
	log.Debug("RedfishEndpoints: " + string(rfeData))

	// Unmarshal it
	log.Debug("unmarshaling EthernetInterfaces")
	var ethIfaceSlice []EthernetInterface
	err = json.Unmarshal(ethIfaceData, &ethIfaceSlice)
	if err != nil {
		return fmt.Errorf("failed to unmarshal EthernetInterface data: %w", err)
	}
	log.Debug("unmarshaling Components")
	var compsStruct struct {
		Components []Component `json:"Components"`
	}
	err = json.Unmarshal(compsData, &compsStruct)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Components data: %w", err)
	}
	log.Debug("unmarshaling RedfishEndpoints")
	var rfeStruct struct {
		RedfishEndpoints []RedfishEndpoint `json:"RedfishEndpoints"`
	}
	err = json.Unmarshal(rfeData, &rfeStruct)
	if err != nil {
		return fmt.Errorf("failed to unmarshal RedfishEndpoints data: %w", err)
	}

	// Organize it to be referenced via map
	log.Debug("organizing EthernetInterfaces into map")
	eiMap := make(map[string]EthernetInterface)
	for _, ei := range ethIfaceSlice {
		eiMap[ei.MACAddress] = ei
	}
	log.Debug("organizing Component into map")
	compMap := make(map[string]Component)
	for _, comp := range compsStruct.Components {
		compMap[comp.ID] = comp
	}
	log.Debug("organizing RedfishEndpoints into map")
	rfeMap := make(map[string]RedfishEndpoint)
	for _, rfe := range rfeStruct.RedfishEndpoints {
		rfeMap[rfe.MACAddr] = rfe
	}

	// Update cache with info
	log.Debug("updating cache with map data")
	c.Mutex.Lock()
	c.EthernetInterfaces = eiMap
	c.Components = compMap
	c.RedfishEndpoints = rfeMap
	c.LastUpdated = time.Now()
	c.Mutex.Unlock()
	log.Infof("Cache updated with %d RedfishEndpoints, %d EthernetInterfaces, and %d Components", len(rfeMap), len(eiMap), len(compMap))
	log.Debugf("RedfishEndpoints: %v", rfeMap)
	log.Debugf("EthernetInterfaces: %v", eiMap)
	log.Debugf("Components: %v", compMap)

	return nil
}

func (c *Cache) RefreshLoop() {
	log.Info("initiating cache refresh loop")
	log.Infof("refreshing cache every duration: %s", c.Duration.String())

	// Initial refresh
	err := c.Refresh()
	if err != nil {
		log.Errorf("failed to refresh cache: %v", err)
	}

	// ...then each duration
	ticker := time.NewTicker(c.Duration)
	go func() {
		for range ticker.C {
			err := c.Refresh()
			if err != nil {
				log.Errorf("failed to refresh cache: %v", err)
			}
		}
	}()
}
