// Package locker provides a TransferManager for managing file transfers and server information.
//
// The TransferManager handles file transfers, tracks transfer status, and stores additional data associated with files.
// It uses BoltDB as the underlying database to store file metadata, server information, and additional data.
//
// File Transfers:
// The TransferManager manages file transfers and tracks the status of each transfer. The status can be one of the following:
// - StatusNotStarted: The transfer has not started yet.
// - StatusInProgress: The transfer is currently in progress.
// - StatusCompleted: The transfer has been successfully completed.
// - StatusFailed: The transfer has failed.
//
// Additional Data:
// Developers can use the AttributesMap to store additional data related to files. This can be useful for storing custom information, such as command flags or any other data relevant to the file transfers.
//
// Server Information:
// The TransferManager can store server information, including the server address, port, and user. This information can be retrieved using the GetServerInfo method.
//
// Usage:
// 1. Create a new TransferManager instance using the NewTransferManager function.
// 2. Use the various methods provided by the TransferManager to manage file transfers, retrieve transfer status, store additional data, and handle server information.
// 3. Close the TransferManager using the Close method to perform cleanup operations and release resources.
//
// Example:
//	// Create a new TransferManager instance
//	tm, err := locker.NewTransferManager()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if tm.IsPreexisting() {
//	    log.Println("Lock file already exists")
//      // add some logic to recover from previous state or resume your transfer
//	}
//
//
//	// Store additional data
//	err = tm.Attributes.StoreOrUpdate("file1", "additional data")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Start a file transfer
//	err = tm.Files.Start("file1")
//	if err != nil {
//		log.Fatal(err)
//	}
//  // Do your oppertaion here
//  // ... transfer file
//  // ...  ftp.Put("file1", "file1")
//	// Set the transfer as successful
//	// Get transfer status
//	status, ok := tm.Files.Load("file1")
//	if !ok {
//		log.Fatal("File not found")
//	}
//	fmt.Println("Transfer status:", status)
//
//	// Get server information
//	serverInfo, err := tm.GetServerInfo()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println("Server address:", serverInfo.Address())
//
//	// Close and Finish the TransferManager
//	err = tm.Finish()
//	if err != nil {
//		log.Fatal(err)
//	}

package reflux

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// TransferStatus represents the status of a file transfer.
//
//go:generate stringer -type=TransferStatus
type TransferStatus uint8

const (
	StatusNotStarted TransferStatus = iota
	StatusInProgress
	StatusCompleted
	StatusFailed
)

// TransferManager manages file transfers and server information.
type TransferManager struct {
	lockFilePath string             // The path of the lock file
	serverInfo   *ServerInfo        // The server info to reconnect
	preexisting  bool               // Whether the lock file already existed
	Files        FileMetadataMap    // type FileMetadata, to avoid race conditions Key is the file path
	Attributes   AttributesMap      // Developers can use this to store additional data, for example command flags the developer is using to run the command
	db           *bolt.DB           // The BoltDB database instance.
	ctx          context.Context    // The context for handling signals and cancellation.
	cancel       context.CancelFunc // The cancelation function for the context.
}

type bucket string // The name of a bucket

// Bytes returns the byte representation of the bucket name.
func (b bucket) Bytes() []byte {
	return []byte(b)
}

const (
	filesBucket          = bucket("Files")
	serverBucket         = bucket("Server")
	additionalDataBucket = bucket("AdditionalData")
	serverInfoKey        = "Info"
)

// NewTransferManager creates a new TransferManager instance.
// It initializes the lock file path, opens the database, and initializes the buckets.
// If the lock file already exists, it loads the existing data from the database.
func NewTransferManager() (*TransferManager, error) {
	tm := &TransferManager{
		lockFilePath: "./." + filepath.Base(os.Args[0]) + ".lock",
	}
	// Check if the lock file exists.
	if _, err := os.Stat(tm.lockFilePath); err == nil {
		tm.preexisting = true
	}

	// Open the database.
	db, err := bolt.Open(tm.lockFilePath, 0600, nil)
	if err != nil {
		return nil, err
	}

	tm.ctx, tm.cancel = context.WithCancel(context.Background())

	tm.db = db
	tm.Files = &fileMetadataMap{
		db: tm.db,
		m:  &sync.Map{},
	}

	tm.Attributes = &attributes{
		db: tm.db,
		m:  &sync.Map{},
	}

	// Initialize buckets
	err = tm.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(filesBucket.Bytes())
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(serverBucket.Bytes())
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(additionalDataBucket.Bytes())
		return err
	})

	if err != nil {
		return nil, err
	}

	// If the lock file already existed, load the existing data.
	if tm.preexisting {
		if err := tm.loadExistingData(); err != nil {
			return nil, err
		}
	}

	err = tm.setupSignalHandling()
	if err != nil {
		return nil, err
	}

	return tm, nil
}

// IsPreexisting returns whether the lock file already existed.
// this is useful to get the latest run status and resume the transfer.
func (tm *TransferManager) IsPreexisting() bool {
	return tm.preexisting
}

// loadExistingData loads the existing data from the database.
// It loads the file metadata, server info, and additional data.
// After loading the data, it performs a database sync to ensure data integrity.
func (tm *TransferManager) loadExistingData() error {
	return tm.db.View(func(tx *bolt.Tx) error {
		if err := tm.Files.loadAll(tx); err != nil {
			return err
		}

		if err := tm.Attributes.loadAll(tx); err != nil {
			return err
		}

		if err := tm.loadServerInfo(tx); err != nil {
			return err
		}

		return tm.db.Sync()
	})
}

// loadServerInfo loads the server info from the database into the TransferManager's serverInfo field.
func (tm *TransferManager) loadServerInfo(tx *bolt.Tx) error {
	b := tx.Bucket(serverBucket.Bytes())
	if b == nil {
		return nil
	}

	v := b.Get([]byte(serverInfoKey))
	if v == nil {
		return nil
	}

	return gob.NewDecoder(bytes.NewReader(v)).Decode(&tm.serverInfo)
}

// Close closes the TransferManager and performs cleanup operations.
// It syncs the database, closes the database connection, and removes the lock file.
func (tm *TransferManager) Close() error {
	if err := tm.db.Sync(); err != nil {
		return errors.Wrap(err, "failed to sync database")
	}

	if err := tm.db.Close(); err != nil {
		return errors.Wrap(err, "failed to close database")
	}
	return nil
}

func (tm *TransferManager) Finish() error {
	if err := tm.Close(); err != nil {
		return err
	}
	if err := os.Remove(tm.lockFilePath); err != nil {
		return errors.Wrap(err, "failed to remove lock file")
	}
	return nil
}

// setupSignalHandling sets up the signal handling for the TransferManager.
func (tm *TransferManager) setupSignalHandling() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			// Received OS signal, initiate shutdown
			tm.cancel()
		case <-tm.ctx.Done():
			// Context canceled, exit goroutine
		}
	}()

	return nil
}

// sync synchronizes the file metadata and additional data in the database with the TransferManager's maps.
func (tm *TransferManager) sync() error {
	err := tm.Files.sync()
	if err != nil {
		return err
	}

	err = tm.Attributes.sync()
	if err != nil {
		return err
	}

	err = tm.db.Sync()
	if err != nil {
		return err
	}

	return nil
}
