package logging

import (
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// this middleware measures how long each HTTP request takes to process.
// It does this by recording the start time before calling the next handler,
// then the end time after the handler finishes.
// It logs the elapsed time, HTTP status code, and request details using LogTiming.
// This helps track request latency for monitoring and debugging.

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := WithContext(r)
		rw := &statusRecorder{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(rw, r.WithContext(ctx))
		elapsed := time.Since(start)
		LogTiming(ctx, r, rw.statusCode, elapsed)
	})
}
