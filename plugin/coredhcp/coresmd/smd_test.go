package coresmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

//==============================================================================
// Helpers
//==============================================================================

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

//==============================================================================
// NewSmdClient
//==============================================================================

func TestNewSmdClient(t *testing.T) {
	baseURL, err := url.Parse("https://example.com/smd")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	tests := []struct {
		name    string
		baseURL *url.URL
	}{
		{
			name:    "basic_new_client",
			baseURL: baseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSmdClient(tt.baseURL)
			if client == nil {
				t.Fatalf("NewSmdClient() returned nil")
			}
			if client.BaseURL != tt.baseURL {
				t.Errorf("BaseURL = %v, want %v", client.BaseURL, tt.baseURL)
			}
			if client.Client == nil {
				t.Errorf("Client is nil, want non-nil http.Client")
			}
		})
	}
}

//==============================================================================
// SmdClient.UseCACert
//==============================================================================

func TestSmdClientUseCACert(t *testing.T) {
	// Create a temp file containing some bytes (doesn't have to be a valid cert)
	makeTempCertFile := func(t *testing.T) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "cacert-*.pem")
		if err != nil {
			t.Fatalf("CreateTemp failed: %v", err)
		}
		// Content doesn't have to be a valid certificate for this test.
		if _, err := f.Write([]byte("dummy cert data")); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close temp file: %v", err)
		}
		return f.Name()
	}

	tests := []struct {
		name             string
		clientFactory    func(t *testing.T) *SmdClient
		pathFactory      func(t *testing.T) string
		wantErr          bool
		expectTransport  bool
		checkTransportFn func(t *testing.T, c *SmdClient)
	}{
		{
			name: "success_sets_transport_with_tls_config",
			clientFactory: func(t *testing.T) *SmdClient {
				return &SmdClient{
					Client: &http.Client{},
				}
			},
			pathFactory: func(t *testing.T) string {
				return makeTempCertFile(t)
			},
			wantErr:         false,
			expectTransport: true,
			checkTransportFn: func(t *testing.T, c *SmdClient) {
				t.Helper()
				tr, ok := c.Transport.(*http.Transport)
				if !ok {
					t.Fatalf("Transport type = %T, want *http.Transport", c.Transport)
				}
				if tr.TLSClientConfig == nil {
					t.Fatalf("TLSClientConfig is nil")
				}
				if tr.TLSClientConfig.RootCAs == nil {
					t.Errorf("RootCAs is nil, expected non-nil cert pool")
				}
				if tr.TLSClientConfig.InsecureSkipVerify {
					t.Errorf("InsecureSkipVerify = true, want false")
				}
				if !tr.DisableKeepAlives {
					t.Errorf("DisableKeepAlives = false, want true")
				}
				if tr.TLSHandshakeTimeout != defaultTlsHandshakeTimeout {
					t.Errorf("TLSHandshakeTimeout = %v, want %v", tr.TLSHandshakeTimeout, defaultTlsHandshakeTimeout)
				}
				if tr.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
					t.Errorf("ResponseHeaderTimeout = %v, want %v", tr.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
				}
			},
		},
		{
			name: "missing_file_returns_error_and_keeps_transport_nil",
			clientFactory: func(t *testing.T) *SmdClient {
				return &SmdClient{
					Client: &http.Client{},
				}
			},
			pathFactory: func(t *testing.T) string {
				return "this_file_should_not_exist.pem"
			},
			wantErr:         true,
			expectTransport: false,
		},
		{
			name: "nil_client_field_returns_error",
			clientFactory: func(t *testing.T) *SmdClient {
				return &SmdClient{
					Client: nil,
				}
			},
			pathFactory: func(t *testing.T) string {
				return makeTempCertFile(t)
			},
			wantErr:         true,
			expectTransport: false,
		},
		{
			name: "nil_receiver_returns_error",
			clientFactory: func(t *testing.T) *SmdClient {
				return nil
			},
			pathFactory: func(t *testing.T) string {
				return makeTempCertFile(t)
			},
			wantErr:         true,
			expectTransport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.clientFactory(t)
			path := tt.pathFactory(t)

			// Call method on possibly-nil receiver
			var err error
			if client == nil {
				var nilClient *SmdClient
				err = nilClient.UseCACert(path)
			} else {
				err = client.UseCACert(path)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("UseCACert() error = %v, wantErr %v", err, tt.wantErr)
			}

			if client == nil {
				// Nothing further to check
				return
			}
			if client.Client == nil {
				// Nothing further to check
				return
			}

			if tt.expectTransport {
				if client.Client.Transport == nil {
					t.Fatalf("Transport is nil, want non-nil")
				}
				if tt.checkTransportFn != nil {
					tt.checkTransportFn(t, client)
				}
			} else {
				if client.Client.Transport != nil {
					t.Errorf("Transport = %#v, want nil", client.Transport)
				}
			}
		})
	}
}

//==============================================================================
// SmdClient.APIGet
//==============================================================================

func TestSmdClientAPIGet_SuccessAndStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "status_200_ok",
			statusCode: http.StatusOK,
			body:       "ok",
		},
		{
			name:       "status_500_still_reads_body",
			statusCode: http.StatusInternalServerError,
			body:       "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			baseURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatalf("failed to parse test server URL: %v", err)
			}

			client := NewSmdClient(baseURL)
			// Use test server's client
			client.Client = srv.Client()

			data, err := client.APIGet("/test/path")
			if err != nil {
				t.Fatalf("APIGet() unexpected error: %v", err)
			}

			if gotPath != "/test/path" {
				t.Errorf("requested path = %q, want %q", gotPath, "/test/path")
			}
			if string(data) != tt.body {
				t.Errorf("APIGet() body = %q, want %q", string(data), tt.body)
			}
		})
	}
}

func TestSmdClientAPIGet_NilClientField(t *testing.T) {
	baseURL, err := url.Parse("http://example.com")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	client := &SmdClient{
		BaseURL: baseURL,
		Client:  nil,
	}

	_, err = client.APIGet("/path")
	if err == nil {
		t.Fatalf("APIGet() error = nil, want non-nil when http.Client is nil")
	}
}

func TestSmdClientAPIGet_DoError(t *testing.T) {
	baseURL, err := url.Parse("http://example.com")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	client := &SmdClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("boom")
			}),
		},
	}

	_, err = client.APIGet("/path")
	if err == nil {
		t.Fatalf("APIGet() error = nil, want non-nil when transport returns error")
	}
}

func TestSmdClientAPIGet_ReadBodyError(t *testing.T) {
	baseURL, err := url.Parse("http://example.com")
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(errorReader{}),
	}

	client := &SmdClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return resp, nil
			}),
		},
	}

	_, err = client.APIGet("/path")
	if err == nil {
		t.Fatalf("APIGet() error = nil, want non-nil on body read error")
	}
}

//==============================================================================
// Struct JSON behavior
//==============================================================================

func TestEthernetInterfaceJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		want     EthernetInterface
	}{
		{
			name: "single_ip_address",
			jsonData: `{
				"MACAddress": "aa:bb:cc:dd:ee:ff",
				"ComponentID": "comp-1",
				"Type": "Ethernet",
				"Description": "test interface",
				"IPAddresses": [
					{"IPAddress": "192.0.2.1"}
				]
			}`,
			want: EthernetInterface{
				MACAddress:  "aa:bb:cc:dd:ee:ff",
				ComponentID: "comp-1",
				Type:        "Ethernet",
				Description: "test interface",
				IPAddresses: []struct {
					IPAddress string `json:"IPAddress"`
				}{
					{IPAddress: "192.0.2.1"},
				},
			},
		},
		{
			name: "multiple_ip_addresses",
			jsonData: `{
				"MACAddress": "11:22:33:44:55:66",
				"ComponentID": "comp-2",
				"Type": "Ethernet",
				"Description": "another interface",
				"IPAddresses": [
					{"IPAddress": "198.51.100.10"},
					{"IPAddress": "198.51.100.11"}
				]
			}`,
			want: EthernetInterface{
				MACAddress:  "11:22:33:44:55:66",
				ComponentID: "comp-2",
				Type:        "Ethernet",
				Description: "another interface",
				IPAddresses: []struct {
					IPAddress string `json:"IPAddress"`
				}{
					{IPAddress: "198.51.100.10"},
					{IPAddress: "198.51.100.11"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got EthernetInterface
			if err := json.Unmarshal([]byte(tt.jsonData), &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if got.MACAddress != tt.want.MACAddress {
				t.Errorf("MACAddress = %q, want %q", got.MACAddress, tt.want.MACAddress)
			}
			if got.ComponentID != tt.want.ComponentID {
				t.Errorf("ComponentID = %q, want %q", got.ComponentID, tt.want.ComponentID)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if len(got.IPAddresses) != len(tt.want.IPAddresses) {
				t.Fatalf("len(IPAddresses) = %d, want %d", len(got.IPAddresses), len(tt.want.IPAddresses))
			}
			for i := range got.IPAddresses {
				if got.IPAddresses[i].IPAddress != tt.want.IPAddresses[i].IPAddress {
					t.Errorf("IPAddresses[%d].IPAddress = %q, want %q",
						i, got.IPAddresses[i].IPAddress, tt.want.IPAddresses[i].IPAddress)
				}
			}
		})
	}
}

func TestComponentJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want Component
	}{
		{
			name: "simple_component",
			json: `{"ID":"x0","NID":1,"Type":"Node"}`,
			want: Component{
				ID:   "x0",
				NID:  1,
				Type: "Node",
			},
		},
		{
			name: "different_values",
			json: `{"ID":"blade-42","NID":2048,"Type":"Blade"}`,
			want: Component{
				ID:   "blade-42",
				NID:  2048,
				Type: "Blade",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Component
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if got.ID != tt.want.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.NID != tt.want.NID {
				t.Errorf("NID = %d, want %d", got.NID, tt.want.NID)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}
		})
	}
}

func TestUseCACertTLSConfigType(t *testing.T) {
	// Ensure Transport is an *http.Transport with TLS configuration
	fName := func(t *testing.T) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "cacert2-*.pem")
		if err != nil {
			t.Fatalf("CreateTemp failed: %v", err)
		}
		if _, err := f.Write([]byte("dummy cert data 2")); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close temp file: %v", err)
		}
		return f.Name()
	}(t)

	client := &SmdClient{Client: &http.Client{}}
	if err := client.UseCACert(fName); err != nil {
		t.Fatalf("UseCACert() error = %v, want nil", err)
	}

	_, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
}
