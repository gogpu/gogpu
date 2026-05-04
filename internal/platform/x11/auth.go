//go:build linux

package x11

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

// Authentication protocol names.
const (
	AuthMITMagicCookie = "MIT-MAGIC-COOKIE-1"
)

// Authority family values (from Xauth/libXau).
const (
	FamilyInternet  uint16 = 0
	FamilyDECnet    uint16 = 1
	FamilyChaos     uint16 = 2
	FamilyInternet6 uint16 = 6
	FamilyLocal     uint16 = 256
	FamilyWild      uint16 = 65535
	FamilyNetname   uint16 = 254
	FamilyKrb5      uint16 = 253
	FamilyLocalHost uint16 = 252
)

// Errors returned by authentication operations.
var (
	ErrNoAuthority     = errors.New("x11: no authority file found")
	ErrNoMatchingAuth  = errors.New("x11: no matching authentication entry")
	ErrInvalidAuthFile = errors.New("x11: invalid authority file format")
)

// AuthEntry represents an entry in the .Xauthority file.
// Address is raw bytes: 4-byte IPv4 for FamilyInternet, 16-byte IPv6 for
// FamilyInternet6, hostname string bytes for FamilyLocal.
type AuthEntry struct {
	Family  uint16
	Address []byte
	Number  string
	Name    string
	Data    []byte
}

// readAuthFile reads the .Xauthority file and returns all entries.
func readAuthFile() ([]AuthEntry, error) {
	path := getAuthFilePath()
	if path == "" {
		return nil, ErrNoAuthority
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoAuthority
		}
		return nil, fmt.Errorf("x11: failed to open authority file: %w", err)
	}
	defer file.Close()

	return parseAuthFile(file)
}

// getAuthFilePath returns the path to the .Xauthority file.
func getAuthFilePath() string {
	// Check XAUTHORITY environment variable first
	if path := os.Getenv("XAUTHORITY"); path != "" {
		return path
	}

	// Fall back to $HOME/.Xauthority
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}

	return filepath.Join(home, ".Xauthority")
}

// parseAuthFile parses the .Xauthority file format.
// The file contains a sequence of entries, each with:
//   - family: uint16 (big-endian!)
//   - address: uint16 length + data
//   - number: uint16 length + data (display number as string)
//   - name: uint16 length + data (auth protocol name)
//   - data: uint16 length + data (auth data, e.g., 16-byte cookie)
func parseAuthFile(r io.Reader) ([]AuthEntry, error) {
	var entries []AuthEntry

	for {
		entry, err := readAuthEntry(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// readAuthEntry reads a single authentication entry.
func readAuthEntry(r io.Reader) (AuthEntry, error) {
	var entry AuthEntry

	// Read family (big-endian uint16)
	var familyBuf [2]byte
	if _, err := io.ReadFull(r, familyBuf[:]); err != nil {
		return entry, err
	}
	entry.Family = binary.BigEndian.Uint16(familyBuf[:])

	// Read address as raw bytes (binary IPv4/IPv6 for FamilyInternet/6,
	// hostname string for FamilyLocal).
	address, err := readAuthBytes(r)
	if err != nil {
		return entry, err
	}
	entry.Address = address

	// Read display number
	number, err := readAuthString(r)
	if err != nil {
		return entry, err
	}
	entry.Number = number

	// Read auth protocol name
	name, err := readAuthString(r)
	if err != nil {
		return entry, err
	}
	entry.Name = name

	// Read auth data
	data, err := readAuthData(r)
	if err != nil {
		return entry, err
	}
	entry.Data = data

	return entry, nil
}

// readAuthString reads a length-prefixed string (big-endian length).
func readAuthString(r io.Reader) (string, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return "", err
	}
	length := binary.BigEndian.Uint16(lenBuf[:])

	if length == 0 {
		return "", nil
	}

	// Sanity check
	if length > 1024 {
		return "", ErrInvalidAuthFile
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}

	return string(data), nil
}

// readAuthBytes reads length-prefixed binary data (big-endian length).
// Used for address field which stores raw bytes (not necessarily valid UTF-8).
func readAuthBytes(r io.Reader) ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint16(lenBuf[:])

	if length == 0 {
		return nil, nil
	}

	if length > 1024 {
		return nil, ErrInvalidAuthFile
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	return data, nil
}

// readAuthData reads length-prefixed auth data (big-endian length).
func readAuthData(r io.Reader) ([]byte, error) {
	var lenBuf [2]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint16(lenBuf[:])

	if length == 0 {
		return nil, nil
	}

	// Sanity check - cookies are typically 16 bytes
	if length > 256 {
		return nil, ErrInvalidAuthFile
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	return data, nil
}

// getAuth returns the authentication data for the given connection.
// family and address come from connAddress() (getpeername pattern, like libxcb).
// displayNum is the display number (e.g., "0" for :0).
// If no matching auth is found, returns empty values (some servers allow unauthenticated connections).
func getAuth(family uint16, address []byte, displayNum string) (name string, data []byte, err error) {
	entries, readErr := readAuthFile()
	if readErr == nil {
		for _, entry := range entries {
			if matchesAuthEntry(entry, family, address, displayNum) {
				return entry.Name, entry.Data, nil
			}
		}
	}
	return "", nil, nil
}

// matchesAuthEntry checks if an auth entry matches the connection parameters.
// Uses binary address comparison (like libXau's XauGetBestAuthByAddr).
func matchesAuthEntry(entry AuthEntry, family uint16, address []byte, displayNum string) bool {
	if entry.Number != displayNum {
		return false
	}

	if entry.Family == FamilyWild {
		return true
	}

	if family == FamilyLocal {
		if entry.Family == FamilyLocal || entry.Family == FamilyLocalHost {
			if len(entry.Address) == 0 {
				return true
			}
			if bytes.Equal(entry.Address, address) {
				return true
			}
			if ourHostname, err := os.Hostname(); err == nil {
				if bytes.Equal(entry.Address, []byte(ourHostname)) {
					return true
				}
			}
		}
		return false
	}

	// TCP: binary match family + address (FamilyInternet or FamilyInternet6)
	return entry.Family == family && bytes.Equal(entry.Address, address)
}

// connAddress extracts the binary address and family from a connected socket.
// This follows the libxcb getpeername() pattern — no extra DNS lookups.
func connAddress(conn net.Conn) (family uint16, address []byte) {
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		if v4 := tcpAddr.IP.To4(); v4 != nil {
			return FamilyInternet, []byte(v4)
		}
		if v6 := tcpAddr.IP.To16(); v6 != nil {
			return FamilyInternet6, []byte(v6)
		}
	}
	hostname, _ := os.Hostname()
	return FamilyLocal, []byte(hostname)
}
