package reflux

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"net"
	"net/url"
)

var (
	ErrServerInfoNotSet     = errors.New("server info not set")
	ErrInvalidAddressFormat = errors.New("invalid address format")
)

// ServerInfo represents information about the server.
type ServerInfo struct {
	Address string // The address of the server
	Port    int    // The port of the server
	User    string // The user of the server
}

// validateAddress checks if the address is a valid URL, DNS name, or IP address.
func (si *ServerInfo) validate() error {
	if _, err := url.Parse(si.Address); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidAddressFormat, err)
	}

	if _, err := net.ResolveIPAddr("ip", si.Address); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidAddressFormat, err)
	}

	if si.Port < 0 || si.Port > 65535 {
		return fmt.Errorf("invalid port: %d", si.Port)
	}

	if si.User == "" {
		return fmt.Errorf("empty user")
	}

	return nil
}

// GetServerInfo returns the server information.
// If the server information is not set, it returns nil and the ErrServerInfoNotSet error.
func (tm *TransferManager) GetServerInfo() (*ServerInfo, error) {
	if tm.serverInfo == nil {
		return nil, ErrServerInfoNotSet
	}
	si := *tm.serverInfo
	return &si, nil
}

// StoreOrUpdateServerInfo stores the server information in the database.
// It encodes the server info and stores it in the Lock File (BoltDB database).
func (tm *TransferManager) StoreOrUpdateServerInfo(info *ServerInfo) error {
	if err := info.validate(); err != nil {
		return err
	}
	err := tm.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(serverBucket.Bytes())
		if err != nil {
			return err
		}

		// Convert the server info to bytes.
		buf := new(bytes.Buffer)
		enc := gob.NewEncoder(buf)
		if err := enc.Encode(*info); err != nil {
			return err
		}

		return b.Put([]byte(serverInfoKey), buf.Bytes())
	})

	if err != nil {
		return err
	}

	tm.serverInfo = info

	return nil
}

// CreateServerInfo creates a new instance of the ServerInfo interface.
func CreateServerInfo(address string, port int, user string) (*ServerInfo, error) {
	si := ServerInfo{
		Address: address,
		Port:    port,
		User:    user,
	}

	if err := si.validate(); err != nil {
		return nil, err
	}

	return &si, nil
}
