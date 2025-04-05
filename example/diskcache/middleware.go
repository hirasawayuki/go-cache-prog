package diskcache

import (
	"context"
	"log"
	"time"

	"github.com/hirasawayuki/go-cache-prog/cache"
)

type loggingResponseWriter struct {
	cache.ResponseWriter
	requestID int64
	command   cache.Cmd
}

func (lw *loggingResponseWriter) WriteResponse(res cache.Response) {
	switch {
	case res.Err != "":
		log.Printf("response error for id=%d: %s", lw.requestID, res.Err)
	case res.Miss:
		log.Printf("cache miss for id=%d", lw.requestID)
	case lw.command == cache.CmdGet:
		log.Printf("cache hit for id=%d, size=%d bytes", lw.requestID, res.Size)
	case lw.command == cache.CmdPut:
		log.Printf("cache saved for id=%d, diskpath=%s", lw.requestID, res.DiskPath)
	case lw.command == cache.CmdClose:
		log.Printf("cache closed for id=%d", lw.requestID)
	}

	lw.ResponseWriter.WriteResponse(res)
}

func LoggingMiddleware() cache.Middleware {
	return func(next cache.Handler) cache.Handler {
		return cache.HandlerFunc(func(ctx context.Context, w cache.ResponseWriter, r *cache.Request) {
			start := time.Now()

			switch r.Command {
			case cache.CmdGet:
				log.Printf("get request received: id=%d, actionID=%x", r.ID, r.ActionID)
			case cache.CmdPut:
				log.Printf("put request received: id=%d, actionID=%x, bodySize=%d", r.ID, r.ActionID, r.BodySize)
			case cache.CmdClose:
				log.Printf("close request received: id=%d", r.ID)
			default:
				log.Printf("unknown command received: id=%d, command=%s", r.ID, r.Command)
			}

			lw := &loggingResponseWriter{
				ResponseWriter: w,
				requestID:      r.ID,
				command:        r.Command,
			}

			next.Handle(ctx, lw, r)

			duration := time.Since(start)
			log.Printf("request id=%d completed in %v", r.ID, duration)
		})
	}
}
