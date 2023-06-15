package reflux

import (
	"bytes"
	"encoding/gob"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"sync"
)

var ErrAttBucketNotFound = errors.Errorf("bucket '%s' not found", additionalDataBucket)

type attributes struct {
	m  *sync.Map
	db *bolt.DB
}

// AttributesMap provides a synchronized map for storing and managing attributes.

type AttributesMap interface {

	// loadAll loads the additional data from the database into the TransferManager's additionalData map.
	loadAll(tx *bolt.Tx) error

	// sync synchronizes the additional data in the database with the additional data in the TransferManager's additionalData map.
	sync() error

	// GetSlice returns a slice of additional data
	GetSlice() ([]any, error)

	// StoreOrUpdate stores or updates the additional data in the database.
	StoreOrUpdate(key string, data any) error

	// Load returns the additional data for the given key.
	Load(key string) (any, bool)

	// Delete deletes the additional data for the given key.
	Delete(key string) error

	// Exists returns true if the additional data for the given key exists.
	Exists(key string) bool
}

// loadAll loads the additional data from the database into
// the TransferManager's additionalData map.
func (at *attributes) loadAll(tx *bolt.Tx) error {
	b := tx.Bucket(additionalDataBucket.Bytes())
	if b == nil {
		return ErrAttBucketNotFound
	}

	return b.ForEach(func(k, v []byte) error {
		var data any
		rdr := bytes.NewReader(v)
		dec := gob.NewDecoder(rdr)
		if err := dec.Decode(&data); err != nil {
			return err
		}
		at.m.Store(string(k), data)
		return nil
	})
}

// syncAttribute returns the file metadata for the given source path.
func (at *attributes) syncAttribute(key string) error {
	att, ok := at.m.Load(key)
	if !ok {
		return errors.Errorf("'%s' file key not found in map", key)
	}
	return at.StoreOrUpdate(key, att)
}

// sync synchronizes the additional data in the database with the additional data in the TransferManager's additionalData map.
func (at *attributes) sync() error {
	at.m.Range(func(key, value any) bool {
		err := at.syncAttribute(key.(string))
		if err != nil {
			return false
		}
		return true
	})
	return at.db.Sync()
}

// GetSlice returns a slice of additional data
func (at *attributes) GetSlice() ([]any, error) {
	var data []any
	at.m.Range(func(key, value any) bool {
		data = append(data, value)
		return true
	})
	return data, nil
}

// StoreOrUpdate stores or updates the additional data in the database.
// It encodes the additional data and stores it in the Lock File (BoltDB database).
func (at *attributes) StoreOrUpdate(key string, data any) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return err
	}

	err := at.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(additionalDataBucket.Bytes())
		if b == nil {
			return ErrAttBucketNotFound
		}
		return b.Put([]byte(key), buf.Bytes())
	})

	if err != nil {
		return err
	}

	at.m.Store(key, data)

	return nil
}

// Load returns the additional data for the given key.
func (at *attributes) Load(key string) (any, bool) {
	return at.m.Load(key)
}

// Delete deletes the additional data for the given key.
func (at *attributes) Delete(key string) error {
	err := at.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(additionalDataBucket.Bytes())
		if b == nil {
			return ErrAttBucketNotFound
		}
		return b.Delete([]byte(key))
	})
	if err != nil {
		return err
	}
	at.m.Delete(key)
	return nil
}

// Exists returns true if the additional data for the given key exists.
func (at *attributes) Exists(key string) bool {
	_, ok := at.m.Load(key)
	return ok
}
