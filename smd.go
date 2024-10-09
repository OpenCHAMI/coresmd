package coresmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type SmdClient struct {
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
	}

	return s, err
}

func (sc *SmdClient) APIGet(path string) ([]byte, error) {
	endpoint := sc.BaseURL.JoinPath(path)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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
