// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package plugin

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/transfer"
	"github.com/miekg/dns"

	"github.com/openchami/coresmd/internal/cache"
	"github.com/openchami/coresmd/internal/smdclient"
)

func createXFRTestPlugin() *Plugin {
	return &Plugin{
		xfr: &xfrState{},
		zones: []Zone{
			{
				Name:        "cluster.local",
				NodePattern: "nid{04d}",
			},
			{
				Name: "bmc.cluster.local",
			},
		},
		cache: &cache.Cache{
			Duration:    1 * time.Minute,
			LastUpdated: time.Now(),
			Mutex:       sync.RWMutex{},
			EthernetInterfaces: map[string]smdclient.EthernetInterface{
				"00:11:22:33:44:55": {
					MACAddress:  "00:11:22:33:44:55",
					ComponentID: "node001",
					Type:        "Node",
					Description: "Test Node Interface",
					IPAddresses: []smdclient.IPAddress{
						{IPAddress: "192.168.1.10"},
						{IPAddress: "2001:db8::1"},
						{IPAddress: "not-an-ip"},
						{IPAddress: "192.168.1.10"}, // duplicate, should be deduplicated
					},
				},
				"aa:bb:cc:dd:ee:ff": {
					MACAddress:  "aa:bb:cc:dd:ee:ff",
					ComponentID: "bmc001",
					Type:        "NodeBMC",
					Description: "Test BMC Interface",
					IPAddresses: []smdclient.IPAddress{
						{IPAddress: "192.168.1.100"},
					},
				},
				"00:00:00:00:00:00": {
					MACAddress:  "00:00:00:00:00:00",
					ComponentID: "unknown001",
					Type:        "Unknown",
					Description: "Unknown component type",
					IPAddresses: []smdclient.IPAddress{
						{IPAddress: "192.168.1.200"},
					},
				},
				"11:11:11:11:11:11": {
					MACAddress:  "11:11:11:11:11:11",
					ComponentID: "missing001",
					Type:        "Node",
					Description: "Missing component",
					IPAddresses: []smdclient.IPAddress{
						{IPAddress: "192.168.1.201"},
					},
				},
			},
			Components: map[string]smdclient.Component{
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
				"unknown001": {
					ID:   "unknown001",
					NID:  0,
					Type: "Unknown",
				},
			},
		},
	}
}

func TestTransferUnknownZone(t *testing.T) {
	p := createXFRTestPlugin()
	_, err := p.Transfer("other.zone", 0)
	if err != transfer.ErrNotAuthoritative {
		t.Fatalf("expected ErrNotAuthoritative, got %v", err)
	}
}

func TestTransferMissingCacheOrXFR(t *testing.T) {
	p := createXFRTestPlugin()

	p.cache = nil
	_, err := p.Transfer("cluster.local", 0)
	if err != transfer.ErrNotAuthoritative {
		t.Fatalf("expected ErrNotAuthoritative with nil cache, got %v", err)
	}

	p = createXFRTestPlugin()
	p.xfr = nil
	_, err = p.Transfer("cluster.local", 0)
	if err != transfer.ErrNotAuthoritative {
		t.Fatalf("expected ErrNotAuthoritative with nil xfr, got %v", err)
	}
}

func TestTransferIXFRUpToDate(t *testing.T) {
	p := createXFRTestPlugin()
	p.xfr.serial = 42

	ch, err := p.Transfer("cluster.local", 42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var batches [][]dns.RR
	for batch := range ch {
		batches = append(batches, batch)
	}

	if len(batches) != 1 {
		t.Fatalf("expected 1 batch for up-to-date IXFR, got %d", len(batches))
	}
	if len(batches[0]) != 1 {
		t.Fatalf("expected 1 RR in batch, got %d", len(batches[0]))
	}
	if _, ok := batches[0][0].(*dns.SOA); !ok {
		t.Fatalf("expected SOA RR, got %T", batches[0][0])
	}
}

func TestTransferAXFR(t *testing.T) {
	p := createXFRTestPlugin()
	p.xfr.serial = 42

	ch, err := p.Transfer("cluster.local", 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var total int
	var firstSOA, lastSOA *dns.SOA
	var nsCount int
	for batch := range ch {
		for _, rr := range batch {
			switch r := rr.(type) {
			case *dns.SOA:
				if firstSOA == nil {
					firstSOA = r
				}
				lastSOA = r
			case *dns.NS:
				nsCount++
			case *dns.A, *dns.AAAA:
				// expected records
			default:
				t.Fatalf("unexpected RR type: %T", rr)
			}
		}
		total += len(batch)
	}

	if firstSOA == nil || lastSOA == nil {
		t.Fatal("expected opening and closing SOA records")
	}
	if nsCount != 1 {
		t.Fatalf("expected 1 NS record, got %d", nsCount)
	}
	if total < 3 {
		t.Fatalf("expected at least 3 RRs (SOA, NS, A/AAAA), got %d", total)
	}
}

func TestTransferBatching(t *testing.T) {
	p := createXFRTestPlugin()
	p.xfr.serial = 1

	// Add many components so records exceed batch size of 100.
	p.cache.Mutex.Lock()
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("node%03d", i)
		p.cache.Components[id] = smdclient.Component{
			ID:   id,
			NID:  int64(i),
			Type: "Node",
		}
		p.cache.EthernetInterfaces[id] = smdclient.EthernetInterface{
			MACAddress:  id,
			ComponentID: id,
			Type:        "Node",
			IPAddresses: []smdclient.IPAddress{
				{IPAddress: fmt.Sprintf("10.0.0.%d", 2*i+1)},
				{IPAddress: fmt.Sprintf("10.0.0.%d", 2*i+2)},
			},
		}
	}
	p.cache.Mutex.Unlock()

	ch, err := p.Transfer("cluster.local", 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var batchCount int
	for range ch {
		batchCount++
	}
	if batchCount < 2 {
		t.Fatalf("expected multiple batches, got %d", batchCount)
	}
}

func TestFindConfiguredZone(t *testing.T) {
	p := createXFRTestPlugin()

	tests := []struct {
		zone string
		want bool
	}{
		{"cluster.local", true},
		{"cluster.local.", true},
		{"CLUSTER.LOCAL", true},
		{"bmc.cluster.local", true},
		{"sub.cluster.local", false},
		{"other.local", false},
	}

	for _, tc := range tests {
		got := p.findConfiguredZone(tc.zone)
		if tc.want && got == nil {
			t.Errorf("expected to find zone %q", tc.zone)
		}
		if !tc.want && got != nil {
			t.Errorf("expected not to find zone %q", tc.zone)
		}
	}
}

func TestNSNames(t *testing.T) {
	p := createXFRTestPlugin()

	custom := &Zone{Name: "example.com", NS: []string{"ns1.example.com", "ns2.example.com"}}
	names := p.nsNames(custom)
	if len(names) != 2 || names[0] != "ns1.example.com." || names[1] != "ns2.example.com." {
		t.Fatalf("unexpected custom NS names: %v", names)
	}

	def := &Zone{Name: "example.com"}
	names = p.nsNames(def)
	if len(names) != 1 || names[0] != "ns.example.com." {
		t.Fatalf("unexpected default NS names: %v", names)
	}
}

func TestMboxName(t *testing.T) {
	p := createXFRTestPlugin()

	custom := &Zone{Name: "example.com", Mailbox: "admin.example.com"}
	if got := p.mboxName(custom); got != "admin.example.com." {
		t.Fatalf("unexpected custom mailbox: %s", got)
	}

	def := &Zone{Name: "example.com"}
	if got := p.mboxName(def); got != "hostmaster.example.com." {
		t.Fatalf("unexpected default mailbox: %s", got)
	}
}

func TestSOARecord(t *testing.T) {
	p := createXFRTestPlugin()
	z := &Zone{Name: "example.com"}
	soa := p.soaRecord(z, 12345)

	if soa.Hdr.Name != "example.com." {
		t.Errorf("unexpected SOA name: %s", soa.Hdr.Name)
	}
	if soa.Serial != 12345 {
		t.Errorf("unexpected SOA serial: %d", soa.Serial)
	}
	if soa.Ns != "ns.example.com." {
		t.Errorf("unexpected SOA ns: %s", soa.Ns)
	}
	if soa.Mbox != "hostmaster.example.com." {
		t.Errorf("unexpected SOA mbox: %s", soa.Mbox)
	}
}

func TestNSRecords(t *testing.T) {
	p := createXFRTestPlugin()
	z := &Zone{Name: "example.com", NS: []string{"ns1.example.com"}}
	rrs := p.nsRecords(z)
	if len(rrs) != 1 {
		t.Fatalf("expected 1 NS record, got %d", len(rrs))
	}
	ns := rrs[0].(*dns.NS)
	if ns.Ns != "ns1.example.com." {
		t.Errorf("unexpected NS name: %s", ns.Ns)
	}
}

func TestZoneRecordsNilCache(t *testing.T) {
	p := createXFRTestPlugin()
	p.cache = nil
	rrs := p.zoneRecords(&p.zones[0])
	if len(rrs) != 0 {
		t.Fatalf("expected no records with nil cache, got %d", len(rrs))
	}
}

func TestZoneRecordsFiltering(t *testing.T) {
	p := createXFRTestPlugin()
	rrs := p.zoneRecords(&p.zones[0])

	var aCount, aaaaCount int
	for _, rr := range rrs {
		switch r := rr.(type) {
		case *dns.A:
			aCount++
			if r.A.String() == "192.168.1.10" && r.Hdr.Name != "node001.cluster.local." && r.Hdr.Name != "nid0001.cluster.local." {
				t.Errorf("unexpected A record owner: %s", r.Hdr.Name)
			}
		case *dns.AAAA:
			aaaaCount++
			if r.AAAA.String() != "2001:db8::1" {
				t.Errorf("unexpected AAAA record: %s", r.AAAA)
			}
		default:
			t.Fatalf("unexpected RR type: %T", rr)
		}
	}

	// Expected records:
	//   node001 (xname + nid pattern) -> 2 A (192.168.1.10 on each name) + 2 AAAA (2001:db8::1 on each name)
	//   bmc001 (xname only)           -> 1 A (192.168.1.100)
	// Duplicate IPv4 for node001 is deduplicated; invalid IP is skipped.
	if aCount != 3 {
		t.Errorf("expected 3 A records, got %d", aCount)
	}
	if aaaaCount != 2 {
		t.Errorf("expected 2 AAAA records, got %d", aaaaCount)
	}
}

func TestZoneRecordsNodeBMC(t *testing.T) {
	p := createXFRTestPlugin()
	rrs := p.zoneRecords(&p.zones[1])

	// All components generate records for every zone using the zone's name.
	// bmc.cluster.local has no NodePattern, so node001 contributes xname only.
	if len(rrs) != 3 {
		t.Fatalf("expected 3 records for bmc zone, got %d", len(rrs))
	}
	var foundBMC bool
	for _, rr := range rrs {
		a, ok := rr.(*dns.A)
		if ok && a.A.String() == "192.168.1.100" {
			foundBMC = true
		}
	}
	if !foundBMC {
		t.Error("expected A record for BMC IP 192.168.1.100")
	}
}

func TestZoneRecordsSorted(t *testing.T) {
	p := createXFRTestPlugin()
	rrs := p.zoneRecords(&p.zones[0])
	for i := 1; i < len(rrs); i++ {
		if rrs[i].String() < rrs[i-1].String() {
			t.Fatal("zone records are not sorted")
		}
	}
}

func TestZoneHash(t *testing.T) {
	p := createXFRTestPlugin()
	h1 := p.zoneHash()
	h2 := p.zoneHash()
	if h1 != h2 {
		t.Fatal("zone hash should be deterministic")
	}

	p.cache.Mutex.Lock()
	p.cache.EthernetInterfaces["new"] = smdclient.EthernetInterface{
		MACAddress:  "new",
		ComponentID: "newnode",
		Type:        "Node",
		IPAddresses: []smdclient.IPAddress{
			{IPAddress: "10.10.10.10"},
		},
	}
	p.cache.Components["newnode"] = smdclient.Component{
		ID:   "newnode",
		NID:  99,
		Type: "Node",
	}
	p.cache.Mutex.Unlock()

	h3 := p.zoneHash()
	if h1 == h3 {
		t.Fatal("zone hash should change when records change")
	}
}

func TestStartSerialWatcherNilXFR(t *testing.T) {
	p := createXFRTestPlugin()
	p.xfr = nil
	// Should not panic and should return immediately.
	p.startSerialWatcher(10 * time.Millisecond)
}

func TestRunSerialUpdate(t *testing.T) {
	p := createXFRTestPlugin()
	p.xfr.serial = 100

	changed, serial := p.runSerialUpdate()
	if !changed {
		t.Fatal("expected serial to change on first update")
	}
	if serial <= 100 {
		t.Fatalf("expected serial > 100, got %d", serial)
	}

	changed2, serial2 := p.runSerialUpdate()
	if changed2 {
		t.Fatal("expected no change when zone data is unchanged")
	}
	if serial2 != serial {
		t.Fatalf("expected serial to remain %d, got %d", serial, serial2)
	}
}

func TestRunSerialUpdateMonotonic(t *testing.T) {
	p := createXFRTestPlugin()
	oldSerial := uint32(time.Now().Unix()) + 1000
	p.xfr.serial = oldSerial

	p.cache.Mutex.Lock()
	p.cache.EthernetInterfaces = map[string]smdclient.EthernetInterface{}
	p.cache.Components = map[string]smdclient.Component{}
	p.cache.Mutex.Unlock()

	changed, serial := p.runSerialUpdate()
	if !changed {
		t.Fatal("expected change")
	}
	if serial != oldSerial+1 {
		t.Fatalf("expected serial to increment by 1, got %d", serial)
	}
}
