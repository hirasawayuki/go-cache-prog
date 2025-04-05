// package cache implements the GOCACHEPROG protocol introduced in Go 1.24.
// The type definitions and comments in this package are based on Go's official implementation.
// Reference: https://pkg.go.dev/cmd/go/internal/cacheprog
//
// Go's source code is released under a BSD license, and this package
// complies with that licensing.
// https://github.com/golang/go/blob/master/LICENSE

package cache

import (
	"io"
	"time"
)

type Cmd string

const (
	// CmdPut tells the cache program to store an object in the cache.
	//
	// [Request.ActionID] is the cache key of this object. The cache should
	// store [Request.OutputID] and [Request.Body] under this key for a
	// later "get" request. It must also store the Body in a file in the local
	// file system and return the path to that file in [Response.DiskPath],
	// which must exist at least until a "close" request.
	CmdPut = Cmd("put")

	// CmdGet tells the cache program to retrieve an object from the cache.
	//
	// [Request.ActionID] specifies the key of the object to get. If the
	// cache does not contain this object, it should set [Response.Miss] to
	// true. Otherwise, it should populate the fields of [Response],
	// including setting [Response.OutputID] to the OutputID of the original
	// "put" request and [Response.DiskPath] to the path of a local file
	// containing the Body of the original "put" request. That file must
	// continue to exist at least until a "close" request.
	CmdGet = Cmd("get")

	// CmdClose requests that the cache program exit gracefully.
	//
	// The cache program should reply to this request and then exit
	// (thus closing its stdout).
	CmdClose = Cmd("close")
)

type Request struct {
	// ID is a unique number per process across all requests.
	// It must be echoed in the Response from the child.
	ID int64

	// Command is the type of request.
	// The go command will only send commands that were declared
	// as supported by the child.
	Command Cmd

	// ActionID is the cache key for "put" and "get" requests.
	ActionID []byte `json:",omitempty"` // or nil if not used

	// OutputID is stored with the body for "put" requests.
	//
	// Prior to Go 1.24, when GOCACHEPROG was still an experiment, this was
	// accidentally named ObjectID. It was renamed to OutputID in Go 1.24.
	OutputID []byte `json:",omitempty"` // or nil if not used

	// Body is the body for "put" requests. It's sent after the JSON object
	// as a base64-encoded JSON string when BodySize is non-zero.
	// It's sent as a separate JSON value instead of being a struct field
	// send in this JSON object so large values can be streamed in both directions.
	// The base64 string body of a Request will always be written
	// immediately after the JSON object and a newline.
	Body io.Reader `json:"-"`

	// BodySize is the number of bytes of Body. If zero, the body isn't written.
	BodySize int64 `json:",omitempty"`

	// ObjectID is the accidental spelling of OutputID that was used prior to Go
	// 1.24.
	//
	// Deprecated: use OutputID. This field is only populated temporarily for
	// backwards compatibility with Go 1.23 and earlier when
	// GOEXPERIMENT=gocacheprog is set. It will be removed in Go 1.25.
	ObjectID []byte `json:",omitempty"`
}

type Response struct {
	ID  int64  // that corresponds to Request; they can be answered out of order
	Err string `json:",omitempty"` // if non-empty, the error

	// KnownCommands is included in the first message that cache helper program
	// writes to stdout on startup (with ID==0). It includes the
	// Request.Command types that are supported by the program.
	//
	// This lets the go command extend the protocol gracefully over time (adding
	// "get2", etc), or fail gracefully when needed. It also lets the go command
	// verify the program wants to be a cache helper.
	KnownCommands []Cmd `json:",omitempty"`

	Miss     bool       `json:",omitempty"` // cache miss
	OutputID []byte     `json:",omitempty"` // the ObjectID stored with the body
	Size     int64      `json:",omitempty"` // body size in bytes
	Time     *time.Time `json:",omitempty"` // when the object was put in the cache (optional; used for cache expiration)

	// DiskPath is the absolute path on disk of the body corresponding to a
	// "get" (on cache hit) or "put" request's ActionID.
	DiskPath string `json:",omitempty"`
}
