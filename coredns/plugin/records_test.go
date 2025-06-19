package plugin

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/OpenCHAMI/coresmd/coresmd"
	"github.com/miekg/dns"
)

func createTestRecordGenerator() *RecordGenerator {
	// Create test cache data
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

	zones := []Zone{
		{
			Name:        "cluster.local",
			NodePattern: "nid{04d}",
			BMCPattern:  "bmc-{id}",
		},
	}

	return NewRecordGenerator(zones, cache)
}

func TestNewRecordGenerator(t *testing.T) {
	rg := createTestRecordGenerator()
	if rg == nil {
		t.Fatal("Expected RecordGenerator to be created")
	}
	if rg.zones == nil {
		t.Fatal("Expected zones to be initialized")
	}
	if rg.cache == nil {
		t.Fatal("Expected cache to be initialized")
	}
}

func TestGenerateARecord_Node(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test A record for node
	aRecord := rg.GenerateARecord("nid0001.cluster.local")
	if aRecord == nil {
		t.Fatal("Expected A record to be generated")
	}

	if !aRecord.A.Equal(net.ParseIP("192.168.1.10")) {
		t.Errorf("Expected IP 192.168.1.10, got %v", aRecord.A)
	}

	if aRecord.Hdr.Name != "nid0001.cluster.local." {
		t.Errorf("Expected name nid0001.cluster.local., got %s", aRecord.Hdr.Name)
	}

	if aRecord.Hdr.Rrtype != dns.TypeA {
		t.Errorf("Expected record type A, got %d", aRecord.Hdr.Rrtype)
	}
}

func TestGenerateARecord_BMC(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test A record for BMC
	aRecord := rg.GenerateARecord("bmc-bmc001.cluster.local")
	if aRecord == nil {
		t.Fatal("Expected A record to be generated")
	}

	if !aRecord.A.Equal(net.ParseIP("192.168.1.100")) {
		t.Errorf("Expected IP 192.168.1.100, got %v", aRecord.A)
	}

	if aRecord.Hdr.Name != "bmc-bmc001.cluster.local." {
		t.Errorf("Expected name bmc-bmc001.cluster.local., got %s", aRecord.Hdr.Name)
	}
}

func TestGenerateARecord_Unknown(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test A record for unknown hostname
	aRecord := rg.GenerateARecord("unknown.cluster.local")
	if aRecord != nil {
		t.Error("Expected no A record for unknown hostname")
	}
}

func TestGeneratePTRRecord_Node(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test PTR record for node IP
	ptrRecord := rg.GeneratePTRRecord("10.1.168.192.in-addr.arpa")
	if ptrRecord == nil {
		t.Fatal("Expected PTR record to be generated")
	}

	if ptrRecord.Ptr != "nid0001.cluster.local." {
		t.Errorf("Expected PTR nid0001.cluster.local., got %s", ptrRecord.Ptr)
	}

	if ptrRecord.Hdr.Name != "10.1.168.192.in-addr.arpa." {
		t.Errorf("Expected name 10.1.168.192.in-addr.arpa., got %s", ptrRecord.Hdr.Name)
	}

	if ptrRecord.Hdr.Rrtype != dns.TypePTR {
		t.Errorf("Expected record type PTR, got %d", ptrRecord.Hdr.Rrtype)
	}
}

func TestGeneratePTRRecord_BMC(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test PTR record for BMC IP
	ptrRecord := rg.GeneratePTRRecord("100.1.168.192.in-addr.arpa")
	if ptrRecord == nil {
		t.Fatal("Expected PTR record to be generated")
	}

	if ptrRecord.Ptr != "bmc-bmc001.cluster.local." {
		t.Errorf("Expected PTR bmc-bmc001.cluster.local., got %s", ptrRecord.Ptr)
	}
}

func TestGeneratePTRRecord_Unknown(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test PTR record for unknown IP
	ptrRecord := rg.GeneratePTRRecord("1.1.168.192.in-addr.arpa")
	if ptrRecord != nil {
		t.Error("Expected no PTR record for unknown IP")
	}
}

func TestGenerateCNAMERecord(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test CNAME record
	cnameRecord := rg.GenerateCNAMERecord("alias.cluster.local", "target.cluster.local")
	if cnameRecord == nil {
		t.Fatal("Expected CNAME record to be generated")
	}

	if cnameRecord.Target != "target.cluster.local." {
		t.Errorf("Expected target target.cluster.local., got %s", cnameRecord.Target)
	}

	if cnameRecord.Hdr.Name != "alias.cluster.local." {
		t.Errorf("Expected name alias.cluster.local., got %s", cnameRecord.Hdr.Name)
	}

	if cnameRecord.Hdr.Rrtype != dns.TypeCNAME {
		t.Errorf("Expected record type CNAME, got %d", cnameRecord.Hdr.Rrtype)
	}
}

func TestGenerateTXTRecord_Node(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test TXT record for node
	txtRecord := rg.GenerateTXTRecord("nid0001.cluster.local")
	if txtRecord == nil {
		t.Fatal("Expected TXT record to be generated")
	}

	if len(txtRecord.Txt) == 0 {
		t.Fatal("Expected TXT record to have content")
	}

	// Check for expected metadata
	expectedMetadata := []string{"type=Node", "nid=1", "component_id=node001", "mac=00:11:22:33:44:55"}
	for _, expected := range expectedMetadata {
		found := false
		for _, txt := range txtRecord.Txt {
			if txt == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metadata '%s' not found in TXT record", expected)
		}
	}

	if txtRecord.Hdr.Name != "nid0001.cluster.local." {
		t.Errorf("Expected name nid0001.cluster.local., got %s", txtRecord.Hdr.Name)
	}

	if txtRecord.Hdr.Rrtype != dns.TypeTXT {
		t.Errorf("Expected record type TXT, got %d", txtRecord.Hdr.Rrtype)
	}
}

func TestGenerateTXTRecord_BMC(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test TXT record for BMC
	txtRecord := rg.GenerateTXTRecord("bmc-bmc001.cluster.local")
	if txtRecord == nil {
		t.Fatal("Expected TXT record to be generated")
	}

	if len(txtRecord.Txt) == 0 {
		t.Fatal("Expected TXT record to have content")
	}

	// Check for expected metadata
	expectedMetadata := []string{"type=NodeBMC", "component_id=bmc001", "mac=aa:bb:cc:dd:ee:ff"}
	for _, expected := range expectedMetadata {
		found := false
		for _, txt := range txtRecord.Txt {
			if txt == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metadata '%s' not found in TXT record", expected)
		}
	}
}

func TestGenerateTXTRecord_Unknown(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test TXT record for unknown hostname
	txtRecord := rg.GenerateTXTRecord("unknown.cluster.local")
	if txtRecord != nil {
		t.Error("Expected no TXT record for unknown hostname")
	}
}

func TestGenerateAllRecordsForHostname(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test generating all records for a node
	records := rg.GenerateAllRecordsForHostname("nid0001.cluster.local")
	if len(records) != 2 {
		t.Fatalf("Expected 2 records (A and TXT), got %d", len(records))
	}

	// Check that we have both A and TXT records
	hasA := false
	hasTXT := false
	for _, record := range records {
		switch record.(type) {
		case *dns.A:
			hasA = true
		case *dns.TXT:
			hasTXT = true
		}
	}

	if !hasA {
		t.Error("Expected A record in results")
	}
	if !hasTXT {
		t.Error("Expected TXT record in results")
	}
}

func TestGenerateAllRecordsForIP(t *testing.T) {
	rg := createTestRecordGenerator()

	// Test generating all records for an IP
	records := rg.GenerateAllRecordsForIP("10.1.168.192.in-addr.arpa")
	if len(records) != 1 {
		t.Fatalf("Expected 1 record (PTR), got %d", len(records))
	}

	// Check that we have a PTR record
	if _, ok := records[0].(*dns.PTR); !ok {
		t.Error("Expected PTR record in results")
	}
}

func TestRecordGeneratorWithNilCache(t *testing.T) {
	rg := &RecordGenerator{
		zones: []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
				BMCPattern:  "bmc-{id}",
			},
		},
		cache: nil,
	}

	// All record generation should return nil with nil cache
	if aRecord := rg.GenerateARecord("nid0001.cluster.local"); aRecord != nil {
		t.Error("Expected nil A record with nil cache")
	}

	if ptrRecord := rg.GeneratePTRRecord("10.1.168.192.in-addr.arpa"); ptrRecord != nil {
		t.Error("Expected nil PTR record with nil cache")
	}

	if txtRecord := rg.GenerateTXTRecord("nid0001.cluster.local"); txtRecord != nil {
		t.Error("Expected nil TXT record with nil cache")
	}
}
