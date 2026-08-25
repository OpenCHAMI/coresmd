// SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC.
// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package smdclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	defaultDialTimeout           = 10 * time.Second
	defaultDialKeepAlive         = 30 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultResponseHeaderTimeout = 10 * time.Second
	defaultRequestTimeout        = 30 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultMaxIdleConns          = 100
	defaultMaxIdleConnsPerHost   = 100
)

type SmdClient struct {
	*http.Client
	BaseURL *url.URL
}

type EthernetInterface struct {
	MACAddress  string      `json:"MACAddress"`
	ComponentID string      `json:"ComponentID"`
	Type        string      `json:"Type"`
	Description string      `json:"Description"`
	IPAddresses []IPAddress `json:"IPAddresses"`
}

// SMD is weird and uses an embedded struct like this
type IPAddress struct {
	IPAddress string `json:"IPAddress"`
}

type Component struct {
	ID   string `json:"ID"`
	NID  int64  `json:"NID"`
	Type string `json:"Type"`
}

func NewSmdClient(baseURL *url.URL) *SmdClient {
	s := &SmdClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Transport: newTransport(nil),
			Timeout:   defaultRequestTimeout,
		},
	}

	return s
}

func newTransport(rootCAs *x509.CertPool) *http.Transport {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultDialKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
	}
	if rootCAs != nil {
		transport.TLSClientConfig = &tls.Config{
			RootCAs:            rootCAs,
			InsecureSkipVerify: false,
		}
	}
	return transport
}

func (sc *SmdClient) UseCACert(path string) error {
	if sc == nil {
		return fmt.Errorf("SmdClient is nil")
	}
	if sc.Client == nil {
		return fmt.Errorf("SmdClient's HTTP client is nil")
	}

	cacert, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cacert)

	(*sc).Transport = newTransport(certPool)

	return nil
}

func (sc *SmdClient) APIGet(path string) ([]byte, error) {
	if sc == nil {
		return nil, fmt.Errorf("SmdClient is nil")
	}
	if sc.Client == nil {
		return nil, fmt.Errorf("SmdClient's HTTP client is nil")
	}
	if sc.BaseURL == nil {
		return nil, fmt.Errorf("SmdClient's BaseURL is nil")
	}

	endpoint := sc.BaseURL.JoinPath(path)
	req, err := http.NewRequest("GET", endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := sc.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("SMD GET %s returned %s: %s", endpoint.String(), resp.Status, string(data))
	}

	return data, nil
}
