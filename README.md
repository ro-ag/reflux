# reflux

[![Go Reference](https://pkg.go.dev/badge/gopkg.in/ro-ag/reflux.v0.svg)](https://pkg.go.dev/gopkg.in/ro-ag/reflux.v0)

`reflux` is a Go package designed to manage file transfers with an emphasis on resuming interrupted transfers. Its main features include the maintenance of comprehensive metadata and additional data associated with each file. By leveraging BoltDB, a lightweight embedded key-value store, Reflux ensures persistent storage of all file-related information.

The utility of Reflux extends to server information storage, allowing developers to store and retrieve server details such as address, port, and user. This makes Reflux a versatile tool for developers looking to manage file transfers and server information in their applications.

Whether you're building a client-server application, a peer-to-peer network, or simply handling files in a local setting, Reflux provides a set of utilities to make your work easier and more efficient. From storing file metadata, tracking transfer status, and handling additional data associated with files, Reflux has got you covered.

## Features

- File transfer management: The `TransferManager` struct allows you to manage file transfers, including storing file metadata, tracking transfer status, and handling additional data associated with files.
- Server information storage: The package provides functionality to store and retrieve server information, such as server address, port, and user.
- Database persistence: The file metadata, additional data, and server information are stored persistently using BoltDB, a lightweight embedded key-value store.

## Installation

To install the `reflux` package, use the following command:

```shell
go get gopkg.in/ro-ag/reflux.v0
```

## Usage

### Importing the package
To use the `reflux` package in your Go code, import it as follows:

```go
import "gopkg.in/ro-ag/reflux.v0"
```

### Creating a new transfer manager
To create a new transfer manager, use the `NewTransferManager` function:

```go
tm, err := reflux.NewTransferManager()
if err != nil {
    // Handle error
}
defer tm.Finish()
```
This initializes the `TransferManager`, opens the database, and sets up the necessary resources. Make sure to defer the `Finish` method to perform cleanup operations when you're done using the `TransferManager`.

### Storing and retrieving file metadata
To store file metadata, use the `StoreOrUpdate` method of the `FileMetadataMap` interface:

```go
metadata := reflux.FileMetadata{
    SourcePath:       "/path/to/source/file.txt",
    TargetPath:       "/path/to/target/file.txt",
    Status:           reflux.StatusNotStarted,
    BytesTransferred: 0,
    // ... additional fields
}

err := tm.Files.StoreOrUpdate(metadata)
if err != nil {
    // Handle error
}
```

To retrieve file metadata, use the `Load` method of the `FileMetadataMap` interface:

```go

fileNameKey := "/path/to/source/file.txt"
metadata, found := tm.Files.Load(fileNameKey) // this is not actually a file name, but a key
if found {
    // File metadata found
    fmt.Println(metadata)
} else {
    // File metadata not found
}
```

### Storing and retrieving server information
To store server information, use the `StoreOrUpdateServerInfo` method of the `TransferManager`:

```go
info := reflux.CreateServerInfo("example.com", 8080, "user")

err := tm.StoreOrUpdateServerInfo(info)
if err != nil {
    // Handle error
}
```

To retrieve server information, use the `GetServerInfo` method of the `TransferManager`:

```go
serverInfo, err := tm.GetServerInfo()
if err != nil {
    // Handle error
} else {
    fmt.Println("Server Address:", serverInfo.Address)
    fmt.Println("Server Port:", serverInfo.Port)
    fmt.Println("Server User:", serverInfo.User)
}
```

## Additional functionality

The `reflux` package provides additional functionality for managing file transfers, including updating transfer status, starting transfers, setting errors, and more. Refer to the package documentation and the source code for detailed usage examples and available methods.