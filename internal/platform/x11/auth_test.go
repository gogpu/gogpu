//go:build linux

package x11

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestParseAuthFile(t *testing.T) {
	var buf bytes.Buffer

	// FamilyLocal entry: address is hostname string bytes
	writeAuthEntry(&buf, FamilyLocal, []byte("localhost"), "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))

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
	if !bytes.Equal(entry.Address, []byte("localhost")) {
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

	// FamilyLocal: hostname string bytes
	writeAuthEntry(&buf, FamilyLocal, []byte("host1"), "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))
	// FamilyWild: empty address
	writeAuthEntry(&buf, FamilyWild, nil, "1", "MIT-MAGIC-COOKIE-1", make([]byte, 16))
	// FamilyInternet: raw 4-byte IPv4 (192.168.1.1 = 0xc0, 0xa8, 0x01, 0x01)
	writeAuthEntry(&buf, FamilyInternet, []byte{192, 168, 1, 1}, "0", "MIT-MAGIC-COOKIE-1", make([]byte, 16))

	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile: unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("parseAuthFile: got %d entries, want 3", len(entries))
	}

	if !bytes.Equal(entries[0].Address, []byte("host1")) {
		t.Errorf("Entry 0 Address: got %q, want %q", entries[0].Address, "host1")
	}
	if entries[1].Family != FamilyWild {
		t.Errorf("Entry 1 Family: got %d, want %d", entries[1].Family, FamilyWild)
	}
	// FamilyInternet: raw bytes, NOT ASCII string
	if !bytes.Equal(entries[2].Address, []byte{192, 168, 1, 1}) {
		t.Errorf("Entry 2 Address: got %v, want [192 168 1 1]", entries[2].Address)
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
		family     uint16
		address    []byte
		displayNum string
		want       bool
	}{
		{
			name:       "local match by hostname",
			entry:      AuthEntry{Family: FamilyLocal, Address: []byte("testhost"), Number: "0"},
			family:     FamilyLocal,
			address:    []byte("testhost"),
			displayNum: "0",
			want:       true,
		},
		{
			name:       "local wrong display",
			entry:      AuthEntry{Family: FamilyLocal, Address: []byte("testhost"), Number: "0"},
			family:     FamilyLocal,
			address:    []byte("testhost"),
			displayNum: "1",
			want:       false,
		},
		{
			name:       "wildcard matches any local",
			entry:      AuthEntry{Family: FamilyWild, Address: nil, Number: "0"},
			family:     FamilyLocal,
			address:    []byte("anyhost"),
			displayNum: "0",
			want:       true,
		},
		{
			name:       "wildcard matches any TCP",
			entry:      AuthEntry{Family: FamilyWild, Address: nil, Number: "0"},
			family:     FamilyInternet,
			address:    []byte{10, 0, 0, 1},
			displayNum: "0",
			want:       true,
		},
		{
			name:       "FamilyInternet binary IPv4 match",
			entry:      AuthEntry{Family: FamilyInternet, Address: []byte{192, 168, 1, 1}, Number: "0"},
			family:     FamilyInternet,
			address:    []byte{192, 168, 1, 1},
			displayNum: "0",
			want:       true,
		},
		{
			name:       "FamilyInternet binary IPv4 mismatch",
			entry:      AuthEntry{Family: FamilyInternet, Address: []byte{192, 168, 1, 1}, Number: "0"},
			family:     FamilyInternet,
			address:    []byte{192, 168, 1, 2},
			displayNum: "0",
			want:       false,
		},
		{
			name:       "FamilyInternet ASCII string does NOT match raw bytes (regression)",
			entry:      AuthEntry{Family: FamilyInternet, Address: []byte{192, 168, 0, 1}, Number: "0"},
			family:     FamilyInternet,
			address:    []byte("192.168.0.1"),
			displayNum: "0",
			want:       false,
		},
		{
			name:       "FamilyInternet6 binary IPv6 match",
			entry:      AuthEntry{Family: FamilyInternet6, Address: net.IPv6loopback, Number: "0"},
			family:     FamilyInternet6,
			address:    net.IPv6loopback,
			displayNum: "0",
			want:       true,
		},
		{
			name:       "FamilyInternet6 binary IPv6 mismatch",
			entry:      AuthEntry{Family: FamilyInternet6, Address: net.IPv6loopback, Number: "0"},
			family:     FamilyInternet6,
			address:    net.IPv6zero,
			displayNum: "0",
			want:       false,
		},
		{
			name:       "family mismatch: entry FamilyInternet vs query FamilyInternet6",
			entry:      AuthEntry{Family: FamilyInternet, Address: []byte{127, 0, 0, 1}, Number: "0"},
			family:     FamilyInternet6,
			address:    net.IPv6loopback,
			displayNum: "0",
			want:       false,
		},
		{
			name:       "FamilyLocalHost matches local connection",
			entry:      AuthEntry{Family: FamilyLocalHost, Address: nil, Number: "0"},
			family:     FamilyLocal,
			address:    []byte("myhost"),
			displayNum: "0",
			want:       true,
		},
		{
			name:       "FamilyLocal does not match TCP connection",
			entry:      AuthEntry{Family: FamilyLocal, Address: []byte("myhost"), Number: "0"},
			family:     FamilyInternet,
			address:    []byte{127, 0, 0, 1},
			displayNum: "0",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAuthEntry(tt.entry, tt.family, tt.address, tt.displayNum)
			if got != tt.want {
				t.Errorf("matchesAuthEntry: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnAddress_TCP(t *testing.T) {
	// Create a TCP listener to get a real connection
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	<-done

	family, address := connAddress(conn)

	if family != FamilyInternet {
		t.Errorf("family: got %d, want %d (FamilyInternet)", family, FamilyInternet)
	}
	if !bytes.Equal(address, []byte{127, 0, 0, 1}) {
		t.Errorf("address: got %v, want [127 0 0 1]", address)
	}
}

func TestConnAddress_Unix(t *testing.T) {
	family, address := connAddress(&fakeUnixConn{})

	if family != FamilyLocal {
		t.Errorf("family: got %d, want %d (FamilyLocal)", family, FamilyLocal)
	}
	if len(address) == 0 {
		t.Errorf("address should be hostname, got empty")
	}
}

// fakeUnixConn simulates a Unix domain socket connection.
type fakeUnixConn struct{ net.Conn }

func (f *fakeUnixConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "/tmp/.X11-unix/X0", Net: "unix"}
}

func TestGetAuth_EndToEnd(t *testing.T) {
	var buf bytes.Buffer
	cookie1 := []byte{0x74, 0xa6, 0x67, 0xbb, 0x67, 0x7d, 0x5e, 0x08, 0x0b, 0x6f, 0xee, 0x36, 0xc4, 0xc2, 0xef, 0xcf}
	cookie2 := []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}

	// Entry 1: FamilyInternet, 192.168.0.1:0
	writeAuthEntry(&buf, FamilyInternet, []byte{192, 168, 0, 1}, "0", "MIT-MAGIC-COOKIE-1", cookie1)
	// Entry 2: FamilyLocal, myhost:0
	writeAuthEntry(&buf, FamilyLocal, []byte("myhost"), "0", "MIT-MAGIC-COOKIE-1", cookie2)

	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile: %v", err)
	}

	tests := []struct {
		name       string
		family     uint16
		address    []byte
		displayNum string
		wantCookie []byte
		wantName   string
	}{
		{
			name:       "match FamilyInternet entry by raw IPv4",
			family:     FamilyInternet,
			address:    []byte{192, 168, 0, 1},
			displayNum: "0",
			wantCookie: cookie1,
			wantName:   "MIT-MAGIC-COOKIE-1",
		},
		{
			name:       "no match for different IP",
			family:     FamilyInternet,
			address:    []byte{10, 0, 0, 1},
			displayNum: "0",
			wantCookie: nil,
			wantName:   "",
		},
		{
			name:       "no match for wrong display number",
			family:     FamilyInternet,
			address:    []byte{192, 168, 0, 1},
			displayNum: "1",
			wantCookie: nil,
			wantName:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var foundName string
			var foundData []byte
			for _, entry := range entries {
				if matchesAuthEntry(entry, tt.family, tt.address, tt.displayNum) {
					foundName = entry.Name
					foundData = entry.Data
					break
				}
			}
			if foundName != tt.wantName {
				t.Errorf("name: got %q, want %q", foundName, tt.wantName)
			}
			if !bytes.Equal(foundData, tt.wantCookie) {
				t.Errorf("data: got %v, want %v", foundData, tt.wantCookie)
			}
		})
	}
}

func TestParseAuthFile_RealXauthorityDump(t *testing.T) {
	// Exact bytes from issue #203 example:
	// $ xauth -f sample.Xauthority add 192.168.0.1:0 . $(mcookie)
	// od -tx1 output:
	// 00 00 00 04 c0 a8 00 01 00 01 30 00 12 4d 49 54
	// 2d 4d 41 47 49 43 2d 43 4f 4f 4b 49 45 2d 31 00
	// 10 74 a6 67 bb 67 7d 5e 08 0b 6f ee 36 c4 c2 ef cf
	raw := []byte{
		0x00, 0x00, // family: FamilyInternet (0)
		0x00, 0x04, // address length: 4
		0xc0, 0xa8, 0x00, 0x01, // address: 192.168.0.1
		0x00, 0x01, // number length: 1
		0x30,       // number: "0"
		0x00, 0x12, // name length: 18
		0x4d, 0x49, 0x54, 0x2d, 0x4d, 0x41, 0x47, 0x49, 0x43, // "MIT-MAGIC"
		0x2d, 0x43, 0x4f, 0x4f, 0x4b, 0x49, 0x45, 0x2d, 0x31, // "-COOKIE-1"
		0x00, 0x10, // data length: 16
		0x74, 0xa6, 0x67, 0xbb, 0x67, 0x7d, 0x5e, 0x08,
		0x0b, 0x6f, 0xee, 0x36, 0xc4, 0xc2, 0xef, 0xcf,
	}

	entries, err := parseAuthFile(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parseAuthFile real dump: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}

	e := entries[0]
	if e.Family != FamilyInternet {
		t.Errorf("Family: got %d, want %d", e.Family, FamilyInternet)
	}
	if !bytes.Equal(e.Address, []byte{0xc0, 0xa8, 0x00, 0x01}) {
		t.Errorf("Address: got %v, want [192 168 0 1]", e.Address)
	}
	if e.Number != "0" {
		t.Errorf("Number: got %q, want %q", e.Number, "0")
	}
	if e.Name != "MIT-MAGIC-COOKIE-1" {
		t.Errorf("Name: got %q, want %q", e.Name, "MIT-MAGIC-COOKIE-1")
	}

	// The key test: matching with binary IP from getpeername succeeds
	if !matchesAuthEntry(e, FamilyInternet, []byte{192, 168, 0, 1}, "0") {
		t.Error("binary IPv4 address should match")
	}
	// And ASCII string does NOT match (this was the bug)
	if matchesAuthEntry(e, FamilyInternet, []byte("192.168.0.1"), "0") {
		t.Error("ASCII string should NOT match binary IPv4 address")
	}
}

func TestParseAuthFile_TruncatedFile(t *testing.T) {
	// Family bytes present but address length missing
	raw := []byte{0x00, 0x00, 0x00}
	_, err := parseAuthFile(bytes.NewReader(raw))
	if err == nil {
		t.Error("expected error for truncated file")
	}
}

func TestParseAuthFile_FirstMatchWins(t *testing.T) {
	var buf bytes.Buffer
	cookie1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	cookie2 := []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	// Two entries for same address — first should win
	writeAuthEntry(&buf, FamilyInternet, []byte{10, 0, 0, 1}, "0", "MIT-MAGIC-COOKIE-1", cookie1)
	writeAuthEntry(&buf, FamilyInternet, []byte{10, 0, 0, 1}, "0", "MIT-MAGIC-COOKIE-1", cookie2)

	entries, err := parseAuthFile(&buf)
	if err != nil {
		t.Fatalf("parseAuthFile: %v", err)
	}

	var found []byte
	for _, entry := range entries {
		if matchesAuthEntry(entry, FamilyInternet, []byte{10, 0, 0, 1}, "0") {
			found = entry.Data
			break
		}
	}
	if !bytes.Equal(found, cookie1) {
		t.Errorf("first-match-wins: got %v, want %v", found, cookie1)
	}
}

// writeAuthEntry writes an auth entry in .Xauthority binary format.
func writeAuthEntry(buf *bytes.Buffer, family uint16, address []byte, number, name string, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, family)

	_ = binary.Write(buf, binary.BigEndian, uint16(len(address)))
	buf.Write(address)

	_ = binary.Write(buf, binary.BigEndian, uint16(len(number)))
	buf.WriteString(number)

	_ = binary.Write(buf, binary.BigEndian, uint16(len(name)))
	buf.WriteString(name)

	_ = binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)
}
