package logging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	CtxKeyTxID   contextKey = "transaction_id"
	CtxKeySpanID contextKey = "span_id"
)

var log = logrus.New()

func init() {
	log.Formatter = &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

func generateUUID() string {
	return uuid.New().String()
}

// isReasonableTransactionID performs basic validation for propagation safety
// Rejects empty, too long, or obviously malicious values
func isReasonableTransactionID(txID string) bool {
	if txID == "" || len(txID) > 128 {
		return false
	}
	// Reject values with control characters that could cause issues
	for _, r := range txID {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func WithContext(r *http.Request) context.Context {
	txID := r.Header.Get("X-Transaction-ID")
	if txID == "" || !isReasonableTransactionID(txID) {
		// If no header or user-provided txID looks suspicious, generate a new one
		txID = generateUUID()
	}

	spanID := generateUUID()
	ctx := context.WithValue(r.Context(), CtxKeyTxID, txID)
	ctx = context.WithValue(ctx, CtxKeySpanID, spanID)
	return ctx
}

func WithGRPCContext(ctx context.Context) context.Context {
	// First check if there's already a transaction_id in the context (from HTTP)
	txID, ok := ctx.Value(CtxKeyTxID).(string)
	if !ok || txID == "" {
		// If not, try to get it from gRPC metadata
		txID = getFromMetadata(ctx, "X-Transaction-ID")
		if txID == "" || !isReasonableTransactionID(txID) {
			// If no metadata or gRPC metadata txID looks suspicious, generate a new one
			txID = generateUUID()
		}
	}

	// Check for span_id in context first
	spanID, ok := ctx.Value(CtxKeySpanID).(string)
	if !ok || spanID == "" {
		// Always generate a new span_id for this service
		// No longer checking metadata since span_id is not propagated
		spanID = generateUUID()
	}

	ctx = context.WithValue(ctx, CtxKeyTxID, txID)
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

// PropagateToGRPC adds the transaction_id from the context to gRPC metadata
// This ensures transaction correlation across service boundaries
// Note: span_id is NOT propagated - each service generates its own span_id
// Note: transaction_id values are validated at input to prevent propagation of malicious values
func PropagateToGRPC(ctx context.Context) context.Context {
	txID := ctx.Value(CtxKeyTxID)

	if txID == nil {
		return ctx
	}

	mdMap := map[string]string{
		"X-Transaction-ID": txID.(string),
	}
	// Deliberately NOT propagating span_id - each service gets its own

	md := metadata.New(mdMap)
	return metadata.NewOutgoingContext(ctx, md)
}

func LogWithContext(ctx context.Context, eventID string, msg string, fields map[string]interface{}) {
	lf := logrus.Fields{
		"event_id": eventID,
	}

	// Safely extract transaction_id and span_id
	if txID := ctx.Value(CtxKeyTxID); txID != nil {
		lf["transaction_id"] = txID
	}
	if spanID := ctx.Value(CtxKeySpanID); spanID != nil {
		lf["span_id"] = spanID
	}

	for k, v := range fields {
		lf[k] = v
	}
	// JSONFormatter automatically handles escaping, preventing log injection
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
