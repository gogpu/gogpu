//go:build linux

package x11

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseAuthFile(t *testing.T) {
	// Create a mock .Xauthority file content
	var buf bytes.Buffer

	// Write an entry: FamilyLocal, hostname "localhost", display "0", MIT-MAGIC-COOKIE-1, 16 bytes cookie
	writeAuthEntry(&buf, FamilyLocal, "localhost", "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))

	// Parse it
	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile: unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("parseAuthFile: got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Family != FamilyLocal {
		t.Errorf("Family: got %d, want %d", entry.Family, FamilyLocal)
	}
	if entry.Address != "localhost" {
		t.Errorf("Address: got %q, want %q", entry.Address, "localhost")
	}
	if entry.Number != "0" {
		t.Errorf("Number: got %q, want %q", entry.Number, "0")
	}
	if entry.Name != "MIT-MAGIC-COOKIE-1" {
		t.Errorf("Name: got %q, want %q", entry.Name, "MIT-MAGIC-COOKIE-1")
	}
	if len(entry.Data) != 16 {
		t.Errorf("Data length: got %d, want 16", len(entry.Data))
	}
}

func TestParseAuthFile_MultipleEntries(t *testing.T) {
	var buf bytes.Buffer

	// Write multiple entries
	writeAuthEntry(&buf, FamilyLocal, "host1", "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))
	writeAuthEntry(&buf, FamilyWild, "", "1", "MIT-MAGIC-COOKIE-1", make([]byte, 16))
	writeAuthEntry(&buf, FamilyInternet, "192.168.1.1", "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))

	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile: unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("parseAuthFile: got %d entries, want 3", len(entries))
	}

	// Verify each entry
	if entries[0].Address != "host1" {
		t.Errorf("Entry 0 Address: got %q, want %q", entries[0].Address, "host1")
	}
	if entries[1].Family != FamilyWild {
		t.Errorf("Entry 1 Family: got %d, want %d", entries[1].Family, FamilyWild)
	}
	if entries[2].Address != "192.168.1.1" {
		t.Errorf("Entry 2 Address: got %q, want %q", entries[2].Address, "192.168.1.1")
	}
}

func TestParseAuthFile_EmptyFile(t *testing.T) {
	var buf bytes.Buffer

	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile empty: unexpected error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("parseAuthFile empty: got %d entries, want 0", len(entries))
	}
}

func TestMatchesAuthEntry(t *testing.T) {
	tests := []struct {
		name       string
		entry      AuthEntry
		hostname   string
		displayNum string
		want       bool
	}{
		{
			name:       "local match",
			entry:      AuthEntry{Family: FamilyLocal, Address: "testhost", Number: "0"},
			hostname:   "",
			displayNum: "0",
			want:       true,
		},
		{
			name:       "local wrong display",
			entry:      AuthEntry{Family: FamilyLocal, Address: "testhost", Number: "0"},
			hostname:   "",
			displayNum: "1",
			want:       false,
		},
		{
			name:       "wildcard match",
			entry:      AuthEntry{Family: FamilyWild, Address: "", Number: "0"},
			hostname:   "anyhost",
			displayNum: "0",
			want:       true,
		},
		{
			name:       "remote match",
			entry:      AuthEntry{Family: FamilyInternet, Address: "192.168.1.1", Number: "0"},
			hostname:   "192.168.1.1",
			displayNum: "0",
			want:       true,
		},
		{
			name:       "remote wrong host",
			entry:      AuthEntry{Family: FamilyInternet, Address: "192.168.1.1", Number: "0"},
			hostname:   "192.168.1.2",
			displayNum: "0",
			want:       false,
		},
		{
			name:       "localhost match local family",
			entry:      AuthEntry{Family: FamilyLocal, Address: "", Number: "0"},
			hostname:   "localhost",
			displayNum: "0",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAuthEntry(tt.entry, tt.hostname, tt.displayNum)
			if got != tt.want {
				t.Errorf("matchesAuthEntry: got %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to write auth entry in .Xauthority format
func writeAuthEntry(buf *bytes.Buffer, family uint16, address, number, name string, data []byte) {
	// Family (big-endian)
	_ = binary.Write(buf, binary.BigEndian, family)

	// Address
	_ = binary.Write(buf, binary.BigEndian, uint16(len(address)))
	buf.WriteString(address)

	// Number
	_ = binary.Write(buf, binary.BigEndian, uint16(len(number)))
	buf.WriteString(number)

	// Name
	_ = binary.Write(buf, binary.BigEndian, uint16(len(name)))
	buf.WriteString(name)

	// Data
	_ = binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)
}
