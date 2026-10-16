package handler

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/thd-spatial-ai/city2tabula/internal/utils"
)

// RequestIDHeader carries a request's id both ways: a caller may send its own,
// so that its log and this server's log share one id for the request.
const RequestIDHeader = "X-Request-ID"

// validRequestID bounds what a caller's id may contain, because it is written
// into every log line of the request.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type requestIDKey struct{}

// RequestID returns the id LogRequests gave the request carrying ctx, or ""
// outside LogRequests.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// statusWriter records the status code the wrapped handler writes.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader records code before passing it on.
func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// LogRequests gives every request an id, returns it in the X-Request-ID response
// header, and logs one line per request once it has been served: method, path,
// query, status and duration. A server error logs at ERROR, a client error at
// WARN, and health checks at DEBUG, since the container probe sends one every
// few seconds.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)

		next.ServeHTTP(sw, r.WithContext(ctx))

		level := slog.LevelInfo
		switch {
		case sw.status >= http.StatusInternalServerError:
			level = slog.LevelError
		case sw.status >= http.StatusBadRequest:
			level = slog.LevelWarn
		case r.URL.Path == "/api/v1/health":
			level = slog.LevelDebug
		}
		utils.Logger.LogAttrs(ctx, level, "http request",
			slog.String("request_id", id),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.Int("status", sw.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}
