package bootloop

import (
	"database/sql"
	"net"
	"path/filepath"
	"strings"
	"testing"
)

// helper to open a fresh test DB with the leases4 table created
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "leases.db")
	db, err := loadDB(path)
	if err != nil {
		t.Fatalf("loadDB(%q) error = %v", path, err)
	}
	return db
}

//
// Tests for loadDB
//

func TestLoadDB(t *testing.T) {
	tests := []struct {
		name string
	}{{
		name: "creates_leases4_table",
	}, {
		name: "idempotent_on_existing_db",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "leases.db")

			// first call
			db1, err := loadDB(path)
			if err != nil {
				t.Fatalf("first loadDB(%q) error = %v", path, err)
			}
			defer db1.Close()

			// verify leases4 table exists
			var name string
			if err := db1.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='leases4'`).Scan(&name); err != nil {
				t.Fatalf("leases4 table not found after loadDB: %v", err)
			}
			if name != "leases4" {
				t.Fatalf("expected table name 'leases4', got %q", name)
			}

			// second call should also succeed (idempotent)
			db2, err := loadDB(path)
			if err != nil {
				t.Fatalf("second loadDB(%q) error = %v", path, err)
			}
			defer db2.Close()
		})
	}
}

//
// Tests for loadRecords
//

func TestLoadRecords(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, db *sql.DB)
		wantErrSub string
		wantLen    int
		wantKey    string
		wantRecord *Record
	}{
		{
			name: "empty_table_returns_empty_map",
			setup: func(t *testing.T, db *sql.DB) {
				// nothing to insert
			},
			wantLen: 0,
		},
		{
			name: "single_valid_row_loaded",
			setup: func(t *testing.T, db *sql.DB) {
				const macStr = "aa:bb:cc:dd:ee:ff"
				_, err := db.Exec(
					`insert into leases4(mac, ip, expiry, hostname) values (?, ?, ?, ?)`,
					macStr,
					"192.168.1.10",
					123,
					"test-host",
				)
				if err != nil {
					t.Fatalf("insert test lease: %v", err)
				}
			},
			wantLen: 1,
			wantKey: "aa:bb:cc:dd:ee:ff",
			wantRecord: &Record{
				IP:       net.ParseIP("192.168.1.10"),
				expires:  123,
				hostname: "test-host",
			},
		},
		{
			name: "invalid_mac_gives_error",
			setup: func(t *testing.T, db *sql.DB) {
				_, err := db.Exec(
					`insert into leases4(mac, ip, expiry, hostname) values (?, ?, ?, ?)`,
					"zz:zz:zz:zz:zz:zz",
					"192.168.1.10",
					123,
					"bad-mac-host",
				)
				if err != nil {
					t.Fatalf("insert invalid mac lease: %v", err)
				}
			},
			wantErrSub: "malformed hardware address",
			wantLen:    0,
		},
		{
			name: "non_ipv4_address_gives_error",
			setup: func(t *testing.T, db *sql.DB) {
				_, err := db.Exec(
					`insert into leases4(mac, ip, expiry, hostname) values (?, ?, ?, ?)`,
					"aa:bb:cc:dd:ee:ff",
					"2001:db8::1",
					456,
					"ipv6-host",
				)
				if err != nil {
					t.Fatalf("insert ipv6 lease: %v", err)
				}
			},
			wantErrSub: "expected an IPv4 address",
			wantLen:    0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			defer db.Close()

			if tt.setup != nil {
				tt.setup(t, db)
			}

			records, err := loadRecords(db)

			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("loadRecords() error = nil, want substring %q", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("loadRecords() error = %v, want substring %q", err, tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("loadRecords() unexpected error = %v", err)
			}

			if gotLen := len(records); gotLen != tt.wantLen {
				t.Fatalf("loadRecords() len(records) = %d, want %d", gotLen, tt.wantLen)
			}

			if tt.wantRecord != nil {
				rec, ok := records[tt.wantKey]
				if !ok {
					t.Fatalf("loadRecords() missing key %q in records map", tt.wantKey)
				}
				if !rec.IP.Equal(tt.wantRecord.IP) {
					t.Errorf("record.IP = %v, want %v", rec.IP, tt.wantRecord.IP)
				}
				if rec.expires != tt.wantRecord.expires {
					t.Errorf("record.expires = %d, want %d", rec.expires, tt.wantRecord.expires)
				}
				if rec.hostname != tt.wantRecord.hostname {
					t.Errorf("record.hostname = %q, want %q", rec.hostname, tt.wantRecord.hostname)
				}
			}
		})
	}
}

//
// Tests for (*PluginState).deleteIPAddress
//

func TestDeleteIPAddress(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) (*PluginState, net.HardwareAddr)
		wantErrSub    string
		wantRemaining int // -1 means "don't check"
	}{
		{
			name: "delete_existing_lease",
			setup: func(t *testing.T) (*PluginState, net.HardwareAddr) {
				db := openTestDB(t)
				mac, err := net.ParseMAC("00:11:22:33:44:55")
				if err != nil {
					t.Fatalf("ParseMAC: %v", err)
				}
				_, err = db.Exec(
					`insert into leases4(mac, ip, expiry, hostname) values (?, ?, ?, ?)`,
					mac.String(),
					"192.168.1.20",
					111,
					"delete-me",
				)
				if err != nil {
					t.Fatalf("insert lease: %v", err)
				}
				return &PluginState{leasedb: db}, mac
			},
			wantRemaining: 0,
		},
		{
			name: "delete_nonexistent_lease_no_error",
			setup: func(t *testing.T) (*PluginState, net.HardwareAddr) {
				db := openTestDB(t)
				mac, err := net.ParseMAC("aa:bb:cc:dd:ee:ff")
				if err != nil {
					t.Fatalf("ParseMAC: %v", err)
				}
				return &PluginState{leasedb: db}, mac
			},
			wantRemaining: 0,
		},
		{
			name: "closed_db_causes_statement_preparation_error",
			setup: func(t *testing.T) (*PluginState, net.HardwareAddr) {
				db := openTestDB(t)
				db.Close()
				mac, err := net.ParseMAC("00:11:22:33:44:55")
				if err != nil {
					t.Fatalf("ParseMAC: %v", err)
				}
				return &PluginState{leasedb: db}, mac
			},
			wantErrSub:    "statement preparation failed",
			wantRemaining: -1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			p, mac := tt.setup(t)
			defer func() {
				if p.leasedb != nil {
					p.leasedb.Close()
				}
			}()

			err := p.deleteIPAddress(mac)

			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("deleteIPAddress() error = nil, want substring %q", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("deleteIPAddress() error = %v, want substring %q", err, tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("deleteIPAddress() unexpected error = %v", err)
			}

			if tt.wantRemaining >= 0 {
				var count int
				if err := p.leasedb.QueryRow(
					`select count(*) from leases4 where mac=?`,
					mac.String(),
				).Scan(&count); err != nil {
					t.Fatalf("count query failed: %v", err)
				}
				if count != tt.wantRemaining {
					t.Fatalf("rows with mac %s = %d, want %d", mac.String(), count, tt.wantRemaining)
				}
			}
		})
	}
}

//
// Tests for (*PluginState).saveIPAddress
//

func TestSaveIPAddress(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) *PluginState
		mac        string
		record     *Record
		wantErrSub string
	}{
		{
			name: "insert_new_lease",
			setup: func(t *testing.T) *PluginState {
				db := openTestDB(t)
				return &PluginState{leasedb: db}
			},
			mac: "00:11:22:33:44:55",
			record: &Record{
				IP:       net.ParseIP("192.168.1.30"),
				expires:  222,
				hostname: "insert-host",
			},
		},
		{
			name: "closed_db_causes_statement_preparation_error",
			setup: func(t *testing.T) *PluginState {
				db := openTestDB(t)
				db.Close()
				return &PluginState{leasedb: db}
			},
			mac: "aa:bb:cc:dd:ee:ff",
			record: &Record{
				IP:       net.ParseIP("192.168.1.40"),
				expires:  333,
				hostname: "closed-db-host",
			},
			wantErrSub: "statement preparation failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setup(t)
			defer func() {
				if p.leasedb != nil {
					p.leasedb.Close()
				}
			}()

			mac, err := net.ParseMAC(tt.mac)
			if err != nil {
				t.Fatalf("ParseMAC(%q): %v", tt.mac, err)
			}

			err = p.saveIPAddress(mac, tt.record)

			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("saveIPAddress() error = nil, want substring %q", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("saveIPAddress() error = %v, want substring %q", err, tt.wantErrSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("saveIPAddress() unexpected error = %v", err)
			}

			// Verify row in DB
			var ip string
			var expiry int
			var hostname string
			if err := p.leasedb.QueryRow(
				`select ip, expiry, hostname from leases4 where mac=?`,
				mac.String(),
			).Scan(&ip, &expiry, &hostname); err != nil {
				t.Fatalf("select lease failed: %v", err)
			}
			if ip != tt.record.IP.String() {
				t.Errorf("stored ip = %q, want %q", ip, tt.record.IP.String())
			}
			if expiry != tt.record.expires {
				t.Errorf("stored expiry = %d, want %d", expiry, tt.record.expires)
			}
			if hostname != tt.record.hostname {
				t.Errorf("stored hostname = %q, want %q", hostname, tt.record.hostname)
			}
		})
	}
}

//
// Tests for (*PluginState).registerBackingDB
//

func TestRegisterBackingDB(t *testing.T) {
	tests := []struct {
		name       string
		initialDB  *sql.DB
		wantErrSub string
		check      func(t *testing.T, p *PluginState)
	}{
		{
			name:      "sets_db_when_nil",
			initialDB: nil,
			check: func(t *testing.T, p *PluginState) {
				if p.leasedb == nil {
					t.Fatalf("leasedb is nil after successful registerBackingDB")
				}
			},
		},
		{
			name:       "errors_when_db_already_set",
			initialDB:  &sql.DB{}, // any non-nil DB pointer
			wantErrSub: "cannot swap out a lease database while running",
			check: func(t *testing.T, p *PluginState) {
				// should not have changed leasedb
				if p.leasedb == nil {
					t.Fatalf("leasedb was cleared unexpectedly")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var p PluginState
			p.leasedb = tt.initialDB

			filename := filepath.Join(t.TempDir(), "leases.db")
			err := p.registerBackingDB(filename)

			if tt.wantErrSub != "" {
				if err == nil {
					t.Fatalf("registerBackingDB() error = nil, want substring %q", tt.wantErrSub)
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("registerBackingDB() error = %v, want substring %q", err, tt.wantErrSub)
				}
			} else if err != nil {
				t.Fatalf("registerBackingDB() unexpected error = %v", err)
			}

			tt.check(t, &p)

			// A segfault occurs if we try to close. For test's sake, we don't.
			//
			//if p.leasedb != nil || !reflect.DeepEqual(p.leasedb, sql.DB{}) {
			//	p.leasedb.Close()
			//}
		})
	}
}
