package cache

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultConcurrency = 8
)

// Serve starts the GOCACHEPROG server with the provided options.
func Serve(opts ...serverOption) error {
	var err error
	sync.OnceFunc(func() {
		srv := &server{
			decoder: json.NewDecoder(os.Stdin),
			writer: &defaultWriter{
				encoder: json.NewEncoder(os.Stdout),
			},
			timeout: defaultTimeout,
			sem:     make(chan struct{}, defaultConcurrency),
		}

		for _, opt := range opts {
			opt(srv)
		}

		err = srv.serve()
	})()
	return err
}

// serverOption is a function that configures a Server.
type serverOption func(*server)

// WithResponseTimeout sets the timeout for request handling.
func WithConcurrency(concurrency uint) serverOption {
	return func(s *server) {
		s.sem = make(chan struct{}, concurrency)
	}
}

// WithResponseTimeout sets the timeout for request handling.
func WithResponseTimeout(timeout time.Duration) serverOption {
	return func(s *server) {
		s.timeout = timeout
	}
}

// server handles the GOCACHEPROG protocol and dispatches requests to registered handlers.
type server struct {
	decoder *json.Decoder
	writer  ResponseWriter
	timeout time.Duration
	wg      sync.WaitGroup
	sem     chan struct{} // Semaphore to limit concurrency
}

// serve starts handling GOCACHEPROG requests until a close request is received
// or an error occurs.
func (s *server) serve() error {
	s.ack()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		var req *Request
		if err := s.decoder.Decode(&req); err != nil {
			s.wg.Wait()
			cancel()
			return fmt.Errorf("error: invalid request: %w", err)
		}

		switch req.Command {
		case CmdGet:
			s.asyncHandleRequest(ctx, req, cancel)
		case CmdPut:
			if err := s.decodePutBody(req); err != nil {
				s.writer.WriteResponse(Response{
					ID:  req.ID,
					Err: fmt.Errorf("error: failed to decode request body: %w", err).Error(),
				})
				cancel()
				continue
			}
			s.asyncHandleRequest(ctx, req, cancel)
		case CmdClose:
			s.wg.Wait()
			s.handleRequest(ctx, req)
			cancel()
			return nil
		default:
			s.writer.WriteResponse(Response{
				ID:  req.ID,
				Err: fmt.Sprintf("error: %s is unknown command", req.Command),
			})
			cancel()
		}
	}
}

// ack sends the initial KnownCommands response, indicating which commands this server supports.
func (s *server) ack() {
	s.writer.WriteResponse(Response{
		ID:            0,
		KnownCommands: serveMux.knownCommands(),
	})
}

// handleRequest processes a request by finding the appropriate handler and applying middlewares.
func (s *server) handleRequest(ctx context.Context, r *Request) {
	serveMux.mu.RLock()
	h, ok := serveMux.m[r.Command]
	serveMux.mu.RUnlock()
	if !ok {
		s.writer.WriteResponse(Response{
			ID:  r.ID,
			Err: fmt.Sprintf("error: unknown command: %s", r.Command),
		})
		return
	}
	serveMux.Apply(h, serveMux.middleware...).Handle(ctx, s.writer, r)
}

// asyncHandleRequest handles a request asynchronously, managing concurrency limits and timeouts.
func (s *server) asyncHandleRequest(ctx context.Context, req *Request, cancel context.CancelFunc) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()

		select {
		case s.sem <- struct{}{}:
			defer func() { <-s.sem }()
			s.handleRequest(ctx, req)
		case <-ctx.Done():
			s.writer.WriteResponse(Response{
				ID:  req.ID,
				Err: fmt.Sprintf("context canceled: %v", ctx.Err()),
			})
			return
		}
	}()
}

// decodePutBody decodes the base64-encoded body that follows a put request.
func (s *server) decodePutBody(req *Request) error {
	if req.BodySize == 0 {
		req.Body = bytes.NewReader(nil)
		return nil
	}
	var base64Body string
	if err := s.decoder.Decode(&base64Body); err != nil {
		return fmt.Errorf("error: failed to decode body: %w", err)
	}
	req.Body = base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64Body))
	return nil
}

// ServeMux is a request multiplexer.
// It registers handlers for different commands and applies middleware.
type ServeMux struct {
	mu              sync.RWMutex
	allowedCommands map[Cmd]struct{}
	m               map[Cmd]Handler
	middleware      []Middleware
}

// Global serveMux instance
var serveMux = &ServeMux{
	allowedCommands: map[Cmd]struct{}{
		CmdGet:   {},
		CmdPut:   {},
		CmdClose: {},
	},
	m: map[Cmd]Handler{},
}
var Mux = serveMux

// HandleFunc registers a handler function for a specific command.
func (mux *ServeMux) HandleFunc(cmd Cmd, handler func(ctx context.Context, w ResponseWriter, r *Request)) {
	if _, ok := mux.allowedCommands[cmd]; !ok {
		panic(fmt.Sprintf("error: unsupported command registered: %s", cmd))
	}

	mux.mu.Lock()
	defer mux.mu.Unlock()
	mux.m[cmd] = HandlerFunc(handler)
}

// HandleGetFunc registers a handler for the get command.
func (mux *ServeMux) HandleGetFunc(handler func(ctx context.Context, w ResponseWriter, r *Request)) {
	mux.HandleFunc(CmdGet, handler)
}

// HandlePutFunc registers a handler for the put command.
func (mux *ServeMux) HandlePutFunc(handler func(ctx context.Context, w ResponseWriter, r *Request)) {
	mux.HandleFunc(CmdPut, handler)
}

// HandleCloseFunc registers a handler for the close command.
func (mux *ServeMux) HandleCloseFunc(handler func(ctx context.Context, w ResponseWriter, r *Request)) {
	mux.HandleFunc(CmdClose, handler)
}

// Use adds middleware to the middleware chain.
func (mux *ServeMux) Use(middleware ...Middleware) {
	mux.middleware = append(mux.middleware, middleware...)
}

// Apply wraps a handler with a chain of middleware in the order they
// should be executed (from outermost to innermost).
func (mux *ServeMux) Apply(h Handler, middleware ...Middleware) Handler {
	for i := range len(middleware) {
		h = middleware[(len(middleware)-1)-i](h)
	}
	return h
}

// knownCommands returns a list of commands that have registered handlers.
func (mux *ServeMux) knownCommands() []Cmd {
	return slices.Collect(maps.Keys(mux.m))
}

// Handler is the interface that handles GOCACHEPROG requests.
type Handler interface {
	Handle(ctx context.Context, w ResponseWriter, r *Request)
}

// HandlerFunc is a function type that implements the Handler interface.
type HandlerFunc func(ctx context.Context, w ResponseWriter, r *Request)

// Handle calls the handler function.
func (f HandlerFunc) Handle(ctx context.Context, w ResponseWriter, r *Request) {
	f(ctx, w, r)
}

// Middleware is a function that wraps a Handler to add functionality.
type Middleware func(Handler) Handler

// ResponseWriter is the interface for writing responses.
type ResponseWriter interface {
	WriteResponse(res Response)
}

// defaultWriter is the default implementation of ResponseWriter that
// writes JSON responses to stdout.
type defaultWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

// WriteResponse encodes and writes a Response as JSON to stdout.
func (w *defaultWriter) WriteResponse(res Response) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.encoder.Encode(res); err != nil {
		err := w.encoder.Encode(Response{
			ID:  res.ID,
			Err: fmt.Sprintf("error: failed to encode response: %v", err),
		})
		if err != nil {
			log.Printf("error: failed to encode response: %v", err)
		}
	}
}
