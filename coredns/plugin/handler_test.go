package plugin

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/OpenCHAMI/coresmd/coresmd"
	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

// Mock plugin handler for testing
type mockHandler struct {
	plugin.Handler
	called bool
}

func (m *mockHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	m.called = true
	return dns.RcodeSuccess, nil
}

func (m *mockHandler) Name() string { return "mock" }

// createTestPlugin creates a plugin instance with test data
func createTestPlugin() *Plugin {
	// Create test cache data - using exact same structure as production
	cache := &coresmd.Cache{
		Duration:    1 * time.Minute,
		Client:      &coresmd.SmdClient{},
		LastUpdated: time.Now(),
		Mutex:       sync.RWMutex{},
		EthernetInterfaces: map[string]coresmd.EthernetInterface{
			"00:11:22:33:44:55": {
				MACAddress:  "00:11:22:33:44:55",
				ComponentID: "node001",
				Type:        "Node",
				Description: "Test Node Interface",
				IPAddresses: []struct {
					IPAddress string `json:"IPAddress"`
				}{
					{IPAddress: "192.168.1.10"},
				},
			},
			"aa:bb:cc:dd:ee:ff": {
				MACAddress:  "aa:bb:cc:dd:ee:ff",
				ComponentID: "bmc001",
				Type:        "NodeBMC",
				Description: "Test BMC Interface",
				IPAddresses: []struct {
					IPAddress string `json:"IPAddress"`
				}{
					{IPAddress: "192.168.1.100"},
				},
			},
		},
		Components: map[string]coresmd.Component{
			"node001": {
				ID:   "node001",
				NID:  1,
				Type: "Node",
			},
			"bmc001": {
				ID:   "bmc001",
				NID:  0,
				Type: "NodeBMC",
			},
		},
	}

	return &Plugin{
		zones: []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
			},
		},
		cache: cache,
	}
}

func TestCreateTestPlugin(t *testing.T) {
	p := createTestPlugin()
	if p == nil {
		t.Fatal("Plugin is nil")
	}
	if p.cache == nil {
		t.Fatal("Cache is nil")
	}
	if p.zones == nil {
		t.Fatal("Zones is nil")
	}
	if len(p.zones) == 0 {
		t.Fatal("Zones is empty")
	}
	if p.cache.EthernetInterfaces == nil {
		t.Fatal("EthernetInterfaces is nil")
	}
	if p.cache.Components == nil {
		t.Fatal("Components is nil")
	}
	t.Log("Plugin created successfully")
}

func TestServeDNS_A_Record_Node(t *testing.T) {
	p := createTestPlugin()

	// Debug: Check if plugin and cache are properly initialized
	if p == nil {
		t.Fatal("Plugin is nil")
	}
	if p.cache == nil {
		t.Fatal("Cache is nil")
	}
	if p.zones == nil {
		t.Fatal("Zones is nil")
	}

	mock := &mockHandler{}
	p.Next = mock

	// Create A record query for node
	req := new(dns.Msg)
	req.SetQuestion("nid0001.cluster.local.", dns.TypeA)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	rcode, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	if rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d, got %d", dns.RcodeSuccess, rcode)
	}

	if len(w.msg.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(w.msg.Answer))
	}

	// Check A record
	if a, ok := w.msg.Answer[0].(*dns.A); ok {
		if !a.A.Equal(net.ParseIP("192.168.1.10")) {
			t.Errorf("Expected IP 192.168.1.10, got %v", a.A)
		}
		if a.Hdr.Name != "nid0001.cluster.local." {
			t.Errorf("Expected name nid0001.cluster.local., got %s", a.Hdr.Name)
		}
	} else {
		t.Fatal("Answer is not an A record")
	}

	// Should not call next plugin
	if mock.called {
		t.Error("Expected next plugin not to be called")
	}
}

func TestServeDNS_PTR_Record_Node(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create PTR record query for node IP
	req := new(dns.Msg)
	req.SetQuestion("10.1.168.192.in-addr.arpa.", dns.TypePTR)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	rcode, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	if rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d, got %d", dns.RcodeSuccess, rcode)
	}

	if len(w.msg.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(w.msg.Answer))
	}

	// Check PTR record
	if ptr, ok := w.msg.Answer[0].(*dns.PTR); ok {
		// Always expect the xname, not the NodePattern
		if ptr.Ptr != "node001.cluster.local." {
			t.Errorf("Expected PTR node001.cluster.local., got %s", ptr.Ptr)
		}
		if ptr.Hdr.Name != "10.1.168.192.in-addr.arpa." {
			t.Errorf("Expected name 10.1.168.192.in-addr.arpa., got %s", ptr.Hdr.Name)
		}
	} else {
		t.Fatal("Answer is not a PTR record")
	}

	// Should not call next plugin
	if mock.called {
		t.Error("Expected next plugin not to be called")
	}
}

func TestServeDNS_PTR_Record_BMC(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create PTR record query for BMC IP
	req := new(dns.Msg)
	req.SetQuestion("100.1.168.192.in-addr.arpa.", dns.TypePTR)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	rcode, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	if rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d, got %d", dns.RcodeSuccess, rcode)
	}

	if len(w.msg.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(w.msg.Answer))
	}

	// Check PTR record
	if ptr, ok := w.msg.Answer[0].(*dns.PTR); ok {
		// Always expect the xname
		if ptr.Hdr.Name != "100.1.168.192.in-addr.arpa." {
			t.Errorf("Expected name 100.1.168.192.in-addr.arpa., got %s", ptr.Hdr.Name)
		}
	} else {
		t.Fatal("Answer is not a PTR record")
	}

	// Should not call next plugin
	if mock.called {
		t.Error("Expected next plugin not to be called")
	}
}

func TestServeDNS_Unknown_A_Record(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create A record query for unknown hostname
	req := new(dns.Msg)
	req.SetQuestion("unknown.cluster.local.", dns.TypeA)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	_, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	// Should call next plugin
	if !mock.called {
		t.Error("Expected next plugin to be called")
	}

	// Should not have written a response
	if w.msg != nil {
		t.Error("Expected no response to be written")
	}
}

func TestServeDNS_Unknown_PTR_Record(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create PTR record query for unknown IP
	req := new(dns.Msg)
	req.SetQuestion("1.1.1.1.in-addr.arpa.", dns.TypePTR)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	_, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	// Should call next plugin
	if !mock.called {
		t.Error("Expected next plugin to be called")
	}

	// Should not have written a response
	if w.msg != nil {
		t.Error("Expected no response to be written")
	}
}

func TestServeDNS_Other_Record_Types(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create MX record query
	req := new(dns.Msg)
	req.SetQuestion("nid0001.cluster.local.", dns.TypeMX)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	_, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	// Should call next plugin
	if !mock.called {
		t.Error("Expected next plugin to be called")
	}

	// Should not have written a response
	if w.msg != nil {
		t.Error("Expected no response to be written")
	}
}

func TestServeDNS_Empty_Question(t *testing.T) {
	p := createTestPlugin()
	mock := &mockHandler{}
	p.Next = mock

	// Create request with no questions
	req := new(dns.Msg)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	_, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	// Should call next plugin
	if !mock.called {
		t.Error("Expected next plugin to be called")
	}

	// Should not have written a response
	if w.msg != nil {
		t.Error("Expected no response to be written")
	}
}

func TestServeDNS_Nil_Cache(t *testing.T) {
	p := &Plugin{
		zones: []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
			},
		},
		cache: nil, // No cache
	}
	mock := &mockHandler{}
	p.Next = mock

	// Create A record query
	req := new(dns.Msg)
	req.SetQuestion("nid0001.cluster.local.", dns.TypeA)

	// Create mock response writer
	w := &mockResponseWriter{}

	// Call ServeDNS
	_, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	// Should call next plugin
	if !mock.called {
		t.Error("Expected next plugin to be called")
	}

	// Should not have written a response
	if w.msg != nil {
		t.Error("Expected no response to be written")
	}
}

// Mock response writer for testing
type mockResponseWriter struct {
	msg *dns.Msg
}

func (m *mockResponseWriter) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}

func (m *mockResponseWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.msg = msg
	return nil
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) Close() error {
	return nil
}

func (m *mockResponseWriter) TsigStatus() error {
	return nil
}

func (m *mockResponseWriter) TsigTimersOnly(bool) {
}

func (m *mockResponseWriter) Hijack() {
}

func TestReverseToIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected net.IP
	}{
		{
			name:     "Valid IPv4 reverse",
			input:    "10.1.168.192.in-addr.arpa",
			expected: net.ParseIP("192.168.1.10"),
		},
		{
			name:     "Valid IPv4 reverse with trailing dot",
			input:    "10.1.168.192.in-addr.arpa.",
			expected: net.ParseIP("192.168.1.10"),
		},
		{
			name:     "Invalid suffix",
			input:    "10.1.168.192.example.com",
			expected: nil,
		},
		{
			name:     "Invalid parts count",
			input:    "10.1.168.in-addr.arpa",
			expected: nil,
		},
		{
			name:     "Invalid IP",
			input:    "256.1.168.192.in-addr.arpa",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reverseToIP(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if !result.Equal(tt.expected) {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestLookupA_Direct(t *testing.T) {
	p := createTestPlugin()

	// Debug: Print what we're looking for
	t.Logf("Looking for: nid0001.cluster.local")
	t.Logf("Zones: %+v", p.zones)
	t.Logf("Cache components: %+v", p.cache.Components)
	t.Logf("Cache interfaces: %+v", p.cache.EthernetInterfaces)

	// Test direct lookupA call
	ip := p.lookupA("nid0001.cluster.local")
	if ip == nil {
		t.Fatal("Expected IP address, got nil")
	}
	if !ip.Equal(net.ParseIP("192.168.1.10")) {
		t.Errorf("Expected IP 192.168.1.10, got %v", ip)
	}
	t.Log("lookupA works correctly")
}

func makeTestPluginWithPattern(pattern string, nid int, xname, ip string) *Plugin {
	return &Plugin{
		zones: []Zone{
			{
				Name:        "test.cluster",
				NodePattern: pattern,
			},
		},
		cache: &coresmd.Cache{
			Duration:    1 * time.Minute,
			Client:      &coresmd.SmdClient{},
			LastUpdated: time.Now(),
			Mutex:       sync.RWMutex{},
			EthernetInterfaces: map[string]coresmd.EthernetInterface{
				xname + "-eth": {
					MACAddress:  xname + "-eth",
					ComponentID: xname,
					Type:        "Node",
					Description: "Test Node Interface",
					IPAddresses: []struct {
						IPAddress string `json:"IPAddress"`
					}{
						{IPAddress: ip},
					},
				},
			},
			Components: map[string]coresmd.Component{
				xname: {
					ID:   xname,
					NID:  int64(nid),
					Type: "Node",
				},
			},
		},
	}
}

func TestLookupA_Patterns(t *testing.T) {
	tests := []struct {
		pattern string
		nid     int
		xname   string
		ip      string
		want    []string // hostnames to test
	}{
		{"nid{04d}", 1, "x1000c0s0b0n0", "10.0.0.1", []string{"nid0001.test.cluster", "x1000c0s0b0n0.test.cluster"}},
		{"re{03d}", 7, "x1000c0s0b0n7", "10.0.0.7", []string{"re007.test.cluster", "x1000c0s0b0n7.test.cluster"}},
		{"fe{02d}", 12, "x1000c0s0b0n12", "10.0.0.12", []string{"fe12.test.cluster", "x1000c0s0b0n12.test.cluster"}},
		{"node-{05d}", 42, "x1000c0s0b0n42", "10.0.0.42", []string{"node-00042.test.cluster", "x1000c0s0b0n42.test.cluster"}},
		{"compute-{05d}", 123, "x1000c0s0b0n123", "10.0.1.23", []string{"compute-00123.test.cluster", "x1000c0s0b0n123.test.cluster"}},
	}

	for _, tt := range tests {
		p := makeTestPluginWithPattern(tt.pattern, tt.nid, tt.xname, tt.ip)
		for _, hostname := range tt.want {
			got := p.lookupA(hostname)
			if got == nil || got.String() != tt.ip {
				t.Errorf("pattern %q, hostname %q: got %v, want %v", tt.pattern, hostname, got, tt.ip)
			}
		}
	}
}

// createTestPluginForBugReport creates a plugin to reproduce the reported bug scenarios
func createTestPluginForBugReport() *Plugin {
	cache := &coresmd.Cache{
		Duration:    1 * time.Minute,
		Client:      &coresmd.SmdClient{},
		LastUpdated: time.Now(),
		Mutex:       sync.RWMutex{},
		EthernetInterfaces: map[string]coresmd.EthernetInterface{
			// BMC with xname x3000c0s0b1 and NID 2 for pattern bmc{03d} = bmc002
			"11:22:33:44:55:66": {
				MACAddress:  "11:22:33:44:55:66",
				ComponentID: "x3000c0s0b1",
				Type:        "NodeBMC",
				Description: "BMC for bug report test",
				IPAddresses: []struct {
					IPAddress string `json:"IPAddress"`
				}{
					{IPAddress: "192.168.100.10"},
				},
			},
		},
		Components: map[string]coresmd.Component{
			"x3000c0s0b1": {
				ID:   "x3000c0s0b1",
				NID:  2,
				Type: "NodeBMC",
			},
		},
	}

	return &Plugin{
		zones: []Zone{
			{
				Name:        "redondo.usrc",
				NodePattern: "nid{04d}",
			},
		},
		cache: cache,
	}
}

// TestServeDNS_BMC_XName_BugReport tests the reported bug for BMC xname lookup
func TestServeDNS_BMC_XName_BugReport(t *testing.T) {
	p := createTestPluginForBugReport()
	mock := &mockHandler{}
	p.Next = mock

	// Test the exact query from the bug report: x3000c0s0b1.redondo.usrc
	req := new(dns.Msg)
	req.SetQuestion("x3000c0s0b1.redondo.usrc.", dns.TypeA)

	w := &mockResponseWriter{}

	rcode, err := p.ServeDNS(context.Background(), w, req)

	if err != nil {
		t.Fatalf("ServeDNS failed: %v", err)
	}

	if rcode != dns.RcodeSuccess {
		t.Errorf("Expected rcode %d (SUCCESS), got %d", dns.RcodeSuccess, rcode)
		t.Logf("This reproduces the bug - BMC xname lookup fails")
	}

	if len(w.msg.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %d - this reproduces the bug", len(w.msg.Answer))
	}

	// Check A record
	if a, ok := w.msg.Answer[0].(*dns.A); ok {
		if !a.A.Equal(net.ParseIP("192.168.100.10")) {
			t.Errorf("Expected IP 192.168.100.10, got %v", a.A)
		}
		if a.Hdr.Name != "x3000c0s0b1.redondo.usrc." {
			t.Errorf("Expected name x3000c0s0b1.redondo.usrc., got %s", a.Hdr.Name)
		}
	} else {
		t.Fatal("Answer is not an A record")
	}

	// Should not call next plugin
	if mock.called {
		t.Error("Expected next plugin not to be called")
	}
}

// TestLookupA_BMC_XName_Direct tests BMC lookup by xname directly (unit test)
func TestLookupA_BMC_XName_Direct(t *testing.T) {
	p := createTestPluginForBugReport()

	// Test direct lookupA call for BMC xname
	ip := p.lookupA("x3000c0s0b1.redondo.usrc")
	if ip == nil {
		t.Fatal("BMC xname lookup failed - this reproduces the bug")
	}

	expected := net.ParseIP("192.168.100.10")
	if !ip.Equal(expected) {
		t.Errorf("Expected IP %v, got %v", expected, ip)
	}
}
