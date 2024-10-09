package coresmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	defaultTlsHandshakeTimeout   = 120 * time.Second
	defaultResponseHeaderTimeout = 120 * time.Second
)

type SmdClient struct {
	*http.Client
	BaseURL *url.URL
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

func NewSmdClient(baseURL string) (*SmdClient, error) {
	url, err := url.Parse(baseURL)
	if err != nil {
		err = fmt.Errorf("failed to parse base URL: %w", err)
	}

	s := &SmdClient{
		BaseURL: url,
		Client:  &http.Client{},
	}

	return s, err
}

func (sc *SmdClient) UseCACert(path string) error {
	cacert, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)

	if sc == nil {
		return fmt.Errorf("SmdClient is nil")
	}
	if sc.Client == nil {
		return fmt.Errorf("SmdClient's HTTP client is nil")
	}

	(*sc).Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true,
		TLSHandshakeTimeout: defaultTlsHandshakeTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
	}

	return nil
}

func (sc *SmdClient) APIGet(path string) ([]byte, error) {
	endpoint := sc.BaseURL.JoinPath(path)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if sc == nil {
		return nil, fmt.Errorf("SmdClient is nil")
	}
	if sc.Client == nil {
		return nil, fmt.Errorf("SmdClient's HTTP client is nil")
	}

	resp, err := sc.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}
