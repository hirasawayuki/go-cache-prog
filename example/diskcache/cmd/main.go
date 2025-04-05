package main

import (
	"log"
	"os"
	"time"

	"github.com/hirasawayuki/go-cache-prog/cache"
	"github.com/hirasawayuki/go-cache-prog/example/diskcache"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[go-cache-prog] ")

	// Initialize the disk cache handler which implements the cache operations
	h, err := diskcache.NewExampleCacheHandler()
	if err != nil {
		log.Printf("unexpected error: %v", err)
		os.Exit(1)
	}

	// Register the logging middleware to record request/response details
	cache.Use(diskcache.LoggingMiddleware())

	// Register handlers for each of the GOCACHEPROG commands
	cache.HandleGetFunc(h.HandleGet)
	cache.HandlePutFunc(h.HandlePut)
	cache.HandleCloseFunc(h.HandleClose)

	// Start the cache server with server options
	if err := cache.Serve(
		cache.WithConcurrency(4),                  // default: 6
		cache.WithResponseTimeout(10*time.Second), // default: 30 * time.Second
	); err != nil {
		log.Printf("unexpected error: %v", err)
		os.Exit(1)
	}
}
