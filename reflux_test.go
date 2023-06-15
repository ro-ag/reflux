package reflux_test

import (
	"gopkg.in/ro-ag/reflux.v0"
	"io"
	"os"
	"testing"
	"time"
)

func TestTransferManager(t *testing.T) {
	// Create a new TransferManager instance
	tm, err := reflux.NewTransferManager()
	if err != nil {
		t.Fatalf("Failed to create TransferManager: %v", err)
	}
	defer func() {
		err := tm.Close()
		if err != nil {
			t.Errorf("Failed to finish TransferManager: %v", err)
		}
	}()

	// Set up test data
	sourcePath := "/path/to/source/file.txt"
	targetPath := "/path/to/target/file.txt"

	// Store file metadata
	fileMetadata := reflux.FileMetadata{
		SourcePath:       sourcePath,
		TargetPath:       targetPath,
		Status:           reflux.StatusNotStarted,
		BytesTransferred: 0,
		TimeStart:        time.Time{},
		TimeEnd:          time.Time{},
		ErrorMsg:         "",
	}
	err = tm.Files.StoreOrUpdate(fileMetadata)
	if err != nil {
		t.Errorf("Failed to store file metadata: %v", err)
	}

	// Verify the stored file metadata
	storedMetadata, ok := tm.Files.Load(sourcePath)
	if !ok {
		t.Error("Failed to load stored file metadata")
	} else if storedMetadata != fileMetadata {
		t.Error("Stored file metadata does not match")
	}

	// Update the status of the file metadata
	newStatus := reflux.StatusInProgress
	newBytesTransferred := 100
	err = tm.Files.UpdateStatus(sourcePath, newStatus, newBytesTransferred, nil)
	if err != nil {
		t.Errorf("Failed to update file metadata status: %v", err)
	}

	// Verify the updated file metadata
	updatedMetadata, ok := tm.Files.Load(sourcePath)
	if !ok {
		t.Error("Failed to load updated file metadata")
	} else if updatedMetadata.Status != newStatus || updatedMetadata.BytesTransferred != newBytesTransferred {
		t.Error("Updated file metadata does not match")
	}

	// Delete the file metadata
	err = tm.Files.Delete(sourcePath)
	if err != nil {
		t.Errorf("Failed to delete file metadata: %v", err)
	}

	// Verify that the file metadata is deleted
	_, ok = tm.Files.Load(sourcePath)
	if ok {
		t.Error("File metadata was not deleted")
	}
}

func TestServerInfo(t *testing.T) {
	// Create a new TransferManager instance
	tm, err := reflux.NewTransferManager()
	if err != nil {
		t.Fatalf("Failed to create TransferManager: %v", err)
	}
	defer func() {
		err := tm.Finish()
		if err != nil {
			t.Errorf("Failed to finish TransferManager: %v", err)
		}
	}()

	// Create server info
	address := "localhost"
	port := 8080
	user := "admin"
	serverInfo, err := reflux.CreateServerInfo(address, port, user)
	if err != nil {
		t.Errorf("Failed to create server info: %v", err)
	}

	// Store or update server info
	err = tm.StoreOrUpdateServerInfo(serverInfo)
	if err != nil {
		t.Errorf("Failed to store or update server info: %v", err)
	}

	// Retrieve server info
	retrievedServerInfo, err := tm.GetServerInfo()
	if err != nil {
		t.Errorf("Failed to retrieve server info: %v", err)
	}

	// Verify the retrieved server info
	if retrievedServerInfo.Address != address || retrievedServerInfo.Port != port || retrievedServerInfo.User != user {
		t.Error("Retrieved server info does not match")
	}
}

func TestAttributesMap(t *testing.T) {
	// Create a new TransferManager instance
	tm, err := reflux.NewTransferManager()
	if err != nil {
		t.Fatalf("Failed to create TransferManager: %v", err)
	}
	defer func() {
		err := tm.Finish()
		if err != nil {
			t.Errorf("Failed to finish TransferManager: %v", err)
		}
	}()

	// Set up test data
	key := "testKey"
	data := "testData"

	// Store or update additional data
	err = tm.Attributes.StoreOrUpdate(key, data)
	if err != nil {
		t.Errorf("Failed to store or update additional data: %v", err)
	}

	// Retrieve additional data
	retrievedData, ok := tm.Attributes.Load(key)
	if !ok {
		t.Error("Failed to retrieve additional data")
	} else if retrievedData != data {
		t.Error("Retrieved additional data does not match")
	}

	// Delete additional data
	err = tm.Attributes.Delete(key)
	if err != nil {
		t.Errorf("Failed to delete additional data: %v", err)
	}

	// Verify that the additional data is deleted
	_, ok = tm.Attributes.Load(key)
	if ok {
		t.Error("Additional data was not deleted")
	}
}

func TestIntegration(t *testing.T) {
	// Create a new TransferManager instance
	tm, err := reflux.NewTransferManager()
	if err != nil {
		t.Fatalf("Failed to create TransferManager: %v", err)
	}
	defer func() {
		err := tm.Finish()
		if err != nil {
			t.Errorf("Failed to finish TransferManager: %v", err)
		}
	}()

	// Set up test data
	sourcePath := "test/source/data.txt"
	targetPath := "test/target/data.txt"

	err = tm.Files.StoreOrUpdate(reflux.FileMetadata{
		SourcePath: sourcePath,
		TargetPath: targetPath,
	})

	if err != nil {
		t.Errorf("Failed to store file metadata: %v", err)
	}

	// Start the transfer
	err = tm.Files.Start(sourcePath)
	if err != nil {
		t.Errorf("Failed to start transfer: %v", err)
	}

	// Perform the transfer operation
	transfer := func(sourcePath string, targetPath string) (int, error) {
		// Simulate the transfer operation by copying the file
		err := copyFile(sourcePath, targetPath)
		if err != nil {
			return 0, err
		}
		return 100, nil // Assuming 100 bytes transferred
	}

	// Operate on the file metadata
	files, err := tm.Files.Operate(transfer)
	if err != nil {
		t.Errorf("Failed to perform transfer operation: %v", err)
	}

	// Verify the status of the file metadata
	for _, file := range files {
		if file.Status != reflux.StatusCompleted {
			t.Errorf("Transfer status is not completed: %s", file.SourcePath)
		}
	}

	// Retrieve the slice of file metadata
	fileMetadataSlice, err := tm.Files.GetSlice()
	if err != nil {
		t.Errorf("Failed to retrieve file metadata slice: %v", err)
	}

	// Verify the length of the file metadata slice
	expectedLength := 1
	if len(fileMetadataSlice) != expectedLength {
		t.Errorf("Unexpected length of file metadata slice. Expected: %d, Actual: %d", expectedLength, len(fileMetadataSlice))
	}
}

// Helper function to copy a file
func copyFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}
