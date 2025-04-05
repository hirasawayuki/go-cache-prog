# go-cache-prog

A library for implementing the `GOCACHEPROG` protocol introduced in Go 1.24. This project provides a server to create custom caching solutions for the Go build system.

## Overview

The `go-cache-prog` project offers:

- A server implementation for the GOCACHEPROG protocol
- A flexible handler interface for implementing custom cache backends
- Easy-to-use middleware support for logging, metrics, and more
- An example disk cache implementation

## Server Package

The `cache` package is the library, providing the GOCACHEPROG protocol implementation:

### Key Components

- `cache.Request`: Represents a cache request (Get, Put, Close)
- `cache.Response`: Represents a cache response
- `cache.Handler`: Interface for implementing cache backends
- `cache.Middleware`: Function type for implementing middleware
- `cache.Server`: Main server that processes GOCACHEPROG protocol

### Getting Started

To implement your own GOCACHEPROG handler:

1. Import the cache package:

```go
import "github.com/hirasawayuki/go-cache-prog/cache"
```

2. Create a struct that implements the handler functions:

```go
type MyCacheHandler struct {
    // your cache state
}

func (h *MyCacheHandler) HandleGet(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
    // Implementation of cache GET logic

    // When cache hit (content exists in cache)
    // w.WriteResponse(&cache.Response{
    //     ID:       r.ID,        // Echo back the request ID
    //     OutputID: "...",       // Output identifier (if applicable)
    //     Size:     1024,        // Size of the cached content in bytes
    //     Time:     time.Now(),  // Timestamp of when the content was cached
    //     DiskPath: "/path/to/cached/content", // Optional path where content is stored
    // })

    // When cache miss (content not found)
    // w.WriteResponse(&cache.Response{
    //     ID:   r.ID,    // Echo back the request ID
    //     Miss: true,    // Indicate cache miss
    // })
}

func (h *MyCacheHandler) HandlePut(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
    // Implementation of cache PUT logic

    // On successful caching
    // w.WriteResponse(&cache.Response{
    //     ID:       r.ID,      // Echo back the request ID
    //     DiskPath: "/path/to/stored/object",  // Path where the object was stored
    // })
}

func (h *MyCacheHandler) HandleClose(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
    // Implementation of CLOSE logic
    // Called when the cache program is about to terminate

    // On successful closure
    // w.WriteResponse(&cache.Response{
    //     ID: r.ID,      // Echo back the request ID
    // })
}
```

3. Register your handler and run cache server:

```go
func main() {
    // Create your handler
    handler := NewMyCacheHandler()

    // Register handlers
    cache.Mux.HandleGetFunc(handler.HandleGet)
    cache.Mux.HandlePutFunc(handler.HandlePut)
    cache.Mux.HandleCloseFunc(handler.HandleClose)

    // Start cache server
    if err := cache.Serve(); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

### Using Middleware

The server supports middleware for cross-cutting concerns:

```go
// Add logging middleware
cache.Mux.Use(LoggingMiddleware())
```

## Example Usage

The example directory contains a complete implementation of a disk cache handler:

```go
// Create disk cache handler
handler, err := diskcache.NewDiskCacheHandler()
if err != nil {
    log.Fatalf("Error creating cache handler: %v", err)
}

// Register with middleware
cache.Mux.Use(diskcache.LoggingMiddleware())
cache.Mux.HandleGetFunc(handler.HandleGet)
cache.Mux.HandlePutFunc(handler.HandlePut)
cache.Mux.HandleCloseFunc(handler.HandleClose)

// Run server
if err := cache.Serve(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

## Using with Go 1.24

To use a GOCACHEPROG implementation with Go 1.24:

1. Build your custom cache program:

```bash
go build -o "/path/to/mycacheprogram" ./example/diskcache/cmd
```

2. Set the GOCACHEPROG environment variable and run Go builds:

```bash
# Example: Install standard library
GOCACHEPROG="/path/to/mycacheprogram" go install std

# The first run will populate your cache
# Subsequent runs will use the cache
```
