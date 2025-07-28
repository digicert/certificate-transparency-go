package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	CtxKeyTraceID contextKey = "trace_id"
	CtxKeySpanID  contextKey = "span_id"
)

var log = logrus.New()

func init() {
	log.Formatter = &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

func generateTraceID() string {
	// OpenTelemetry standard: 32 hex characters (128 bits)
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateSpanID() string {
	// OpenTelemetry standard: 16 hex characters (64 bits)
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func extractTraceID(r *http.Request) string {
	// Try W3C traceparent header first: "00-{trace_id}-{span_id}-{flags}"
	if traceparent := r.Header.Get("traceparent"); traceparent != "" {
		parts := strings.Split(traceparent, "-")
		if len(parts) >= 2 && len(parts[1]) == 32 {
			return parts[1] // Extract the trace_id part
		}
	}
	
	// Fallback to direct trace ID header
	if traceID := r.Header.Get("x-trace-id"); traceID != "" {
		return normalizeToHex(traceID)
	}
	
	return ""
}

func normalizeToHex(id string) string {
	// Remove hyphens from UUID format and convert to lowercase
	normalized := strings.ReplaceAll(id, "-", "")
	normalized = strings.ToLower(normalized)
	
	// Ensure it's 32 characters for trace ID (pad or truncate if needed)
	if len(normalized) > 32 {
		return normalized[:32]
	}
	if len(normalized) < 32 {
		// Pad with zeros
		return normalized + strings.Repeat("0", 32-len(normalized))
	}
	return normalized
}

func WithContext(r *http.Request) context.Context {
	traceID := extractTraceID(r)
	if traceID == "" {
		traceID = generateTraceID()
	}

	spanID := generateSpanID()
	ctx := context.WithValue(r.Context(), CtxKeyTraceID, traceID)
	ctx = context.WithValue(ctx, CtxKeySpanID, spanID)
	return ctx
}

func WithGRPCContext(ctx context.Context) context.Context {
	// First check if there's already a trace_id in the context (from HTTP)
	traceID, ok := ctx.Value(CtxKeyTraceID).(string)
	if !ok || traceID == "" {
		// If not, try to get it from gRPC metadata
		traceID = getFromMetadata(ctx, "x-trace-id")
		if traceID == "" {
			traceID = generateTraceID()
		}
	}

	// Always generate a new span_id for this service
	// This ensures each service has its own span while maintaining trace correlation
	spanID := generateSpanID()

	ctx = context.WithValue(ctx, CtxKeyTraceID, traceID)
	ctx = context.WithValue(ctx, CtxKeySpanID, spanID)
	return ctx
}

func getFromMetadata(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// PropagateToGRPC adds the trace_id from the context to gRPC metadata
// This ensures trace correlation across service boundaries
// Note: span_id is NOT propagated - each service generates its own span_id
func PropagateToGRPC(ctx context.Context) context.Context {
	traceID := ctx.Value(CtxKeyTraceID)

	if traceID == nil {
		return ctx
	}

	mdMap := map[string]string{
		"x-trace-id": traceID.(string),
	}
	// Deliberately NOT propagating span_id - each service gets its own

	md := metadata.New(mdMap)
	return metadata.NewOutgoingContext(ctx, md)
}

func LogWithContext(ctx context.Context, eventID string, msg string, fields map[string]interface{}) {
	lf := logrus.Fields{
		"event_id": eventID,
	}

	// Extract trace_id and span_id using standard OpenTelemetry field names
	if traceID := ctx.Value(CtxKeyTraceID); traceID != nil {
		lf["trace_id"] = traceID
	}
	if spanID := ctx.Value(CtxKeySpanID); spanID != nil {
		lf["span_id"] = spanID
	}

	for k, v := range fields {
		lf[k] = v
	}
	log.WithFields(lf).Info(msg)
}

func LogTiming(ctx context.Context, r *http.Request, status int, elapsed time.Duration) {
	elapsedInMsStr := fmt.Sprintf("%dms", elapsed.Milliseconds())
	LogWithContext(ctx, "timing", "request completed", map[string]interface{}{
		"path":    r.URL.Path,
		"method":  r.Method,
		"status":  status,
		"elapsed": elapsedInMsStr,
	})
}

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		ctx = WithGRPCContext(ctx)
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		LogWithContext(ctx, "timing", "gRPC call completed", map[string]interface{}{
			"method":  info.FullMethod,
			"status":  statusCodeFromError(err),
			"elapsed": elapsed.Milliseconds(),
		})
		return resp, err
	}
}

func statusCodeFromError(err error) string {
	if err == nil {
		return "OK"
	}
	return err.Error()
}
