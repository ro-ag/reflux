package reflux

import (
	"bytes"
	"encoding/gob"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"sync"
	"time"
)

// FileMetadata contains information about a file transfer.
type FileMetadata struct {
	SourcePath       string         // The path of the file on the local machine
	TargetPath       string         // The path of the file on the remote machine
	Status           TransferStatus // The status of the transfer
	BytesTransferred int            // The number of bytes transferred
	TimeStart        time.Time      // The time the transfer started
	TimeEnd          time.Time      // The time the transfer ended
	ErrorMsg         string         // The error that occurred during the transfer
}

type fileMetadataMap struct {
	m  *sync.Map
	db *bolt.DB
}

type Transfer func(sourcePath string, targetPath string) (int, error)

// FileMetadataMap provides a synchronized map for storing and managing file metadata.
type FileMetadataMap interface {

	// loadAll loads the file metadata from the database into the TransferManager's files map.
	loadAll(tx *bolt.Tx) error

	// sync synchronizes the file metadata in the database with the file metadata in the TransferManager's files map.
	sync() error

	// StoreOrUpdate stores or updates the file metadata in the database.
	// It encodes the file metadata and stores it in the Lock File (BoltDB database).
	StoreOrUpdate(metadata FileMetadata) error

	// Load returns the file metadata for the given source path.
	Load(sourcePath string) (FileMetadata, bool)

	// Delete deletes the file metadata for the given source path.
	Delete(sourcePath string) error

	// Operate operates on the file metadata for the given source path.
	Operate(op Transfer) ([]FileMetadata, error)

	// GetSlice returns a slice of file metadata
	GetSlice() ([]FileMetadata, error)

	// UpdateStatus updates the status of the file metadata for the given source path.
	UpdateStatus(sourcePath string, status TransferStatus, bytesTransferred int, err error) error

	// Start starts the transfer for the given source path.
	Start(sourcePath string) error

	// SetError sets the error for the given source path.
	SetError(sourcePath string, err error) error

	// SetSuccess sets the success for the given source path.
	SetSuccess(sourcePath string, bytesTransferred int) error
}

// loadAll loads the file metadata from the database into the TransferManager's files map.
func (fmm *fileMetadataMap) loadAll(tx *bolt.Tx) error {
	b := tx.Bucket(filesBucket.Bytes())
	if b == nil {
		return nil
	}

	return b.ForEach(func(k, v []byte) error {
		var metadata FileMetadata
		dec := gob.NewDecoder(bytes.NewReader(v))
		err := dec.Decode(&metadata)
		if err != nil {
			return err
		}
		fmm.m.Store(string(k), metadata)
		return nil
	})
}

// StoreOrUpdate stores or updates the file metadata in the database.
// It encodes the file metadata and stores it in the Lock File (BoltDB database).
func (fmm *fileMetadataMap) StoreOrUpdate(metadata FileMetadata) error {

	err := fmm.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(filesBucket.Bytes())
		if err != nil {
			return err
		}

		// Convert the file metadata to bytes.
		buf := new(bytes.Buffer)
		enc := gob.NewEncoder(buf)
		if err := enc.Encode(metadata); err != nil {
			return err
		}
		return b.Put([]byte(metadata.SourcePath), buf.Bytes())
	})

	if err != nil {
		return err
	}

	fmm.m.Store(metadata.SourcePath, metadata)

	return nil
}

// Load returns the file metadata for the given source path.
func (fmm *fileMetadataMap) syncFile(sourcePath string) error {
	meta, ok := fmm.m.Load(sourcePath)
	if !ok {
		return errors.Errorf("'%s' file key not found in map", sourcePath)
	}
	return fmm.StoreOrUpdate(meta.(FileMetadata))
}

// Load returns the file metadata for the given source path.
func (fmm *fileMetadataMap) Load(sourcePath string) (FileMetadata, bool) {
	meta, ok := fmm.m.Load(sourcePath)
	if !ok {
		return FileMetadata{}, false
	}
	return meta.(FileMetadata), true
}

// Delete deletes the file metadata for the given source path.
func (fmm *fileMetadataMap) Delete(sourcePath string) error {
	err := fmm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(filesBucket.Bytes())
		if b == nil {
			return nil
		}
		return b.Delete([]byte(sourcePath))
	})
	if err != nil {
		return err
	}
	fmm.m.Delete(sourcePath)
	return nil
}

// sync synchronizes the file metadata in the database with the file metadata in the TransferManager's files map.
func (fmm *fileMetadataMap) sync() error {
	fmm.m.Range(func(key, value any) bool {
		err := fmm.syncFile(key.(string))
		if err != nil {
			return false
		}
		return true
	})
	return fmm.db.Sync()
}

// Operate executes the given operation on each file metadata in the map.
func (fmm *fileMetadataMap) Operate(transfer Transfer) ([]FileMetadata, error) {

	var errGeneral error
	fmm.m.Range(func(key, value any) bool {
		meta := value.(FileMetadata)

		errGeneral = fmm.UpdateStatus(meta.SourcePath, StatusInProgress, 0, nil)
		if errGeneral != nil {
			return false
		}

		n, err := transfer(meta.SourcePath, meta.TargetPath)
		if err != nil {
			errGeneral = fmm.UpdateStatus(meta.SourcePath, StatusFailed, n, err)
		} else {
			errGeneral = fmm.UpdateStatus(meta.SourcePath, StatusCompleted, n, nil)
		}

		if errGeneral != nil {
			return false
		}

		return true
	})

	if errGeneral != nil {
		return nil, errGeneral
	}

	if err := fmm.sync(); err != nil {
		return nil, err
	}

	return fmm.GetSlice()

}

// GetSlice returns a slice of file metadata
func (fmm *fileMetadataMap) GetSlice() ([]FileMetadata, error) {
	files := make([]FileMetadata, 0)
	fmm.m.Range(func(key, value any) bool {
		files = append(files, value.(FileMetadata))
		return true
	})
	if len(files) == 0 {
		return nil, errors.New("no files found")
	}
	return files, nil
}

// UpdateStatus updates the status of the file metadata for the given source path.
func (fmm *fileMetadataMap) UpdateStatus(sourcePath string, status TransferStatus, bytesTransferred int, err error) error {
	meta, ok := fmm.Load(sourcePath)
	if !ok {
		return errors.Errorf("'%s' file key not found in map", sourcePath)
	}
	meta.Status = status
	meta.BytesTransferred = bytesTransferred
	if err != nil {
		meta.ErrorMsg = err.Error()
	}
	if status == StatusInProgress {
		meta.TimeStart = time.Now()
	} else if status == StatusCompleted || status == StatusFailed {
		meta.TimeEnd = time.Now()
	}

	return fmm.StoreOrUpdate(meta)
}

// Start starts the transfer for the given source path.
func (fmm *fileMetadataMap) Start(sourcePath string) error {
	return fmm.UpdateStatus(sourcePath, StatusInProgress, 0, nil)
}

// SetError sets the error for the given source path.
func (fmm *fileMetadataMap) SetError(sourcePath string, err error) error {
	return fmm.UpdateStatus(sourcePath, StatusFailed, 0, err)
}

// SetSuccess sets the success for the given source path.
func (fmm *fileMetadataMap) SetSuccess(sourcePath string, bytesTransferred int) error {
	return fmm.UpdateStatus(sourcePath, StatusCompleted, bytesTransferred, nil)
}
