//go:build linux

package x11

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Authentication protocol names.
const (
	AuthMITMagicCookie = "MIT-MAGIC-COOKIE-1"
)

// Authority family values (from Xauth).
const (
	FamilyInternet  uint16 = 0
	FamilyDECnet    uint16 = 1
	FamilyChaos     uint16 = 2
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
type AuthEntry struct {
	Family  uint16
	Address string
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

	// Read address
	address, err := readAuthString(r)
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

// getAuth returns the authentication data for the given display.
// hostname should be empty for local connections.
// displayNum is the display number (e.g., "0" for :0).
// If no matching auth is found, returns empty values (some servers allow unauthenticated connections).
func getAuth(hostname, displayNum string) (name string, data []byte, err error) {
	entries, readErr := readAuthFile()
	if readErr == nil {
		// Try to find a matching entry
		for _, entry := range entries {
			// Check if this entry matches our connection
			if matchesAuthEntry(entry, hostname, displayNum) {
				return entry.Name, entry.Data, nil
			}
		}
	}
	// If no authority file exists, read failed, or no matching entry found,
	// return empty auth - this is not an error as some servers allow
	// unauthenticated connections.
	return "", nil, nil
}

// matchesAuthEntry checks if an auth entry matches the connection parameters.
func matchesAuthEntry(entry AuthEntry, hostname, displayNum string) bool {
	// Check display number
	if entry.Number != displayNum {
		return false
	}

	// Check hostname/family
	if hostname == "" || hostname == "localhost" {
		// Local connection - match FamilyLocal or FamilyWild
		if entry.Family == FamilyLocal || entry.Family == FamilyWild || entry.Family == FamilyLocalHost {
			return true
		}
		// Also check if the address is our hostname
		if ourHostname, err := os.Hostname(); err == nil {
			if entry.Address == ourHostname {
				return true
			}
		}
	} else if entry.Address == hostname {
		// Remote connection - check address
		return true
	}

	// Check for wildcard
	if entry.Family == FamilyWild {
		return true
	}

	return false
}

// localHostname returns the local hostname.
func localHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}
