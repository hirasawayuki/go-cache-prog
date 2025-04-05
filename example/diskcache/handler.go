package diskcache

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hirasawayuki/go-cache-prog/cache"
)

const (
	actionFileSuffix = "-a"
	objectFileSuffix = "-d"
)

type LocalDiskCacheHandler struct {
	cacheDir string
}

func NewExampleCacheHandler() (*LocalDiskCacheHandler, error) {
	cacheDir := filepath.Join(os.TempDir(), "cacheprog")
	handler := &LocalDiskCacheHandler{
		cacheDir: cacheDir,
	}

	if err := handler.initializeCache(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return handler, nil
}

// initializeCache prepares the cache directory structure.
// It creates the main cache directory and 256 subdirectories (16x16)
// named with hexadecimal values from "00" to "ff". These subdirectories
// are used to distribute cache files and avoid having too many files
// in a single directory, which can lead to performance issues on some
// filesystems. Returns an error if directory creation fails.
func (h *LocalDiskCacheHandler) initializeCache() error {
	if err := os.MkdirAll(h.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	for i := range 16 {
		for j := range 16 {
			subdir := filepath.Join(h.cacheDir, fmt.Sprintf("%x%x", i, j))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory %s: %w", subdir, err)
			}
		}
	}

	log.Printf("Initialized cache directory at %s", h.cacheDir)
	return nil
}

// HandleGet processes cache retrieval requests.
// It checks if the requested cache entry exists by looking up the action file
// using the provided ActionID. If found, it reads the metadata (OutputID, size,
// and timestamp), verifies the corresponding object file exists with the expected
// size, and returns its details. If any step fails or the cache entry is not found,
// it returns a cache miss or appropriate error.
func (h *LocalDiskCacheHandler) HandleGet(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
	actionPath := h.getActionPath(r.ActionID)

	actionFile, err := os.Open(actionPath)
	if os.IsNotExist(err) {
		w.WriteResponse(cache.Response{
			ID:   r.ID,
			Miss: true,
		})
		return
	} else if err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to open action file: %w", err))
		return
	}
	defer actionFile.Close()

	var outputID []byte
	var fileSize int64
	var timestampUnix int64
	var hexOutputID string
	_, err = fmt.Fscanf(actionFile, "%s %d %d", &hexOutputID, &fileSize, &timestampUnix)
	if err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to parse action file: %w", err))
		return
	}

	timestamp := time.Unix(timestampUnix, 0)
	outputID, err = hex.DecodeString(hexOutputID)
	if err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to decode output ID: %w", err))
		return
	}

	objectPath := h.getObjectPath(outputID)
	fi, err := os.Stat(objectPath)
	if os.IsNotExist(err) {
		w.WriteResponse(cache.Response{
			ID:   r.ID,
			Miss: true,
		})
		return
	} else if err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to stat object file: %w", err))
		return
	}

	if fi.Size() != fileSize {
		w.WriteResponse(cache.Response{
			ID:   r.ID,
			Miss: true,
		})
		return
	}

	w.WriteResponse(cache.Response{
		ID:       r.ID,
		OutputID: outputID,
		Size:     fileSize,
		Time:     &timestamp,
		DiskPath: objectPath,
	})
}

// HandlePut processes cache storage requests.
// It saves the cache data from the request body to a file named after the OutputID,
// then creates a metadata file keyed by ActionID containing the OutputID, file size,
// and timestamp. If any step in the process fails, it cleans up any partially created
// files and returns an error. On success, it returns the path to the stored object.
func (h *LocalDiskCacheHandler) HandlePut(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
	outputID := r.OutputID
	objectPath := h.getObjectPath(outputID)
	if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to create directory: %w", err))
		return
	}

	objectFile, err := os.Create(objectPath)
	if err != nil {
		h.writeErrorResponse(w, r, fmt.Errorf("failed to create object file: %w", err))
		return
	}

	size, err := io.Copy(objectFile, r.Body)
	objectFile.Close()
	if err != nil {
		os.Remove(objectPath)
		h.writeErrorResponse(w, r, fmt.Errorf("failed to write object file: %w", err))
		return
	}

	actionPath := h.getActionPath(r.ActionID)
	actionFile, err := os.Create(actionPath)
	if err != nil {
		os.Remove(objectPath)
		h.writeErrorResponse(w, r, fmt.Errorf("failed to create action file: %w", err))
		return
	}

	_, err = fmt.Fprintf(actionFile, "%x %d %d", outputID, size, time.Now().Unix())
	actionFile.Close()
	if err != nil {
		os.Remove(objectPath)
		os.Remove(actionPath)
		h.writeErrorResponse(w, r, fmt.Errorf("failed to write action file: %w", err))
		return
	}

	w.WriteResponse(cache.Response{
		ID:       r.ID,
		DiskPath: objectPath,
	})
}

// HandleClose processes the close command.
// It responds with the request ID to acknowledge receipt of the close command,
// allowing the Go command to terminate the cache program.
func (h *LocalDiskCacheHandler) HandleClose(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
	w.WriteResponse(cache.Response{
		ID: r.ID,
	})
}

func (h *LocalDiskCacheHandler) writeErrorResponse(w cache.ResponseWriter, r *cache.Request, err error) {
	w.WriteResponse(cache.Response{
		ID:  r.ID,
		Err: err.Error(),
	})
}

func (h *LocalDiskCacheHandler) getObjectPath(objectID []byte) string {
	hexID := hex.EncodeToString(objectID)
	return filepath.Join(h.cacheDir, hexID[:2], hexID+objectFileSuffix)
}

func (h *LocalDiskCacheHandler) getActionPath(actionID []byte) string {
	hexID := hex.EncodeToString(actionID)
	return filepath.Join(h.cacheDir, hexID[:2], hexID+actionFileSuffix)
}
