package logging

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc/metadata"
)

func TestGenerateUUID(t *testing.T) {
	// Test that generateUUID returns a non-empty string
	uuid1 := generateUUID()
	uuid2 := generateUUID()

	if uuid1 == "" {
		t.Error("generateUUID returned empty string")
	}

	if uuid2 == "" {
		t.Error("generateUUID returned empty string")
	}

	// Test that two UUIDs are different
	if uuid1 == uuid2 {
		t.Error("generateUUID returned the same UUID twice, should be unique")
	}

	// Test that UUID has expected format (basic check for dashes)
	if !strings.Contains(uuid1, "-") {
		t.Error("generateUUID didn't return expected UUID format")
	}
}

func TestWithContext(t *testing.T) {
	// Create a test HTTP request
	req := httptest.NewRequest("GET", "/test", nil)

	// Test case 1: Request without existing transaction ID
	ctx := WithContext(req)

	// Check that transaction ID was added
	txID := ctx.Value(CtxKeyTxID)
	if txID == nil {
		t.Error("WithContext didn't add transaction ID to context")
	}

	// Check that span ID was added
	spanID := ctx.Value(CtxKeySpanID)
	if spanID == nil {
		t.Error("WithContext didn't add span ID to context")
	}

	// Check that both IDs are strings
	if _, ok := txID.(string); !ok {
		t.Error("Transaction ID is not a string")
	}
	if _, ok := spanID.(string); !ok {
		t.Error("Span ID is not a string")
	}
}

func TestWithContextExistingTransactionID(t *testing.T) {
	// Create a test HTTP request with existing transaction ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Transaction-ID", "existing-tx-id")

	ctx := WithContext(req)

	// Check that the existing transaction ID was preserved
	txID := ctx.Value(CtxKeyTxID)
	if txID != "existing-tx-id" {
		t.Errorf("Expected transaction ID 'existing-tx-id', got %v", txID)
	}

	// Check that a new span ID was still generated
	spanID := ctx.Value(CtxKeySpanID)
	if spanID == nil {
		t.Error("WithContext didn't add span ID to context")
	}
}

func TestWithGRPCContext(t *testing.T) {
	// Test case 1: Empty context
	ctx := context.Background()
	newCtx := WithGRPCContext(ctx)

	txID := newCtx.Value(CtxKeyTxID)
	spanID := newCtx.Value(CtxKeySpanID)

	if txID == nil || spanID == nil {
		t.Error("WithGRPCContext didn't add IDs to empty context")
	}

	// Test case 2: Context with existing values
	existingCtx := context.WithValue(context.Background(), CtxKeyTxID, "existing-tx")
	existingCtx = context.WithValue(existingCtx, CtxKeySpanID, "existing-span")

	newCtx2 := WithGRPCContext(existingCtx)

	if newCtx2.Value(CtxKeyTxID) != "existing-tx" {
		t.Error("WithGRPCContext didn't preserve existing transaction ID")
	}
	if newCtx2.Value(CtxKeySpanID) != "existing-span" {
		t.Error("WithGRPCContext didn't preserve existing span ID")
	}
}

func TestWithGRPCContextFromMetadata(t *testing.T) {
	// Create context with gRPC metadata
	md := metadata.Pairs("X-Transaction-ID", "metadata-tx-id", "X-Span-ID", "metadata-span-id")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	newCtx := WithGRPCContext(ctx)

	txID := newCtx.Value(CtxKeyTxID)
	spanID := newCtx.Value(CtxKeySpanID)

	if txID != "metadata-tx-id" {
		t.Errorf("Expected transaction ID from metadata 'metadata-tx-id', got %v", txID)
	}
	// Span ID should be newly generated, not from metadata (each service gets its own span)
	if spanID == "metadata-span-id" {
		t.Errorf("Expected new span ID to be generated, but got metadata span ID: %v", spanID)
	}
	if spanID == "" {
		t.Error("Expected new span ID to be generated, but got empty string")
	}
}

func TestLogWithContext(t *testing.T) {
	// Create a test hook and attach it to our global logger
	hook := test.NewLocal(log)

	// Create context with IDs
	ctx := context.WithValue(context.Background(), CtxKeyTxID, "test-tx-id")
	ctx = context.WithValue(ctx, CtxKeySpanID, "test-span-id")

	// Test logging
	LogWithContext(ctx, "test-event", "test message", map[string]interface{}{
		"custom_field": "custom_value",
	})

	// Check that a log entry was created
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	if entry == nil {
		t.Error("LastEntry() returned nil")
		return
	}

	// Check log level
	if entry.Level != logrus.InfoLevel {
		t.Errorf("Expected INFO level, got %v", entry.Level)
	}

	// Check message
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %v", entry.Message)
	}

	// Check fields
	if entry.Data["event_id"] != "test-event" {
		t.Errorf("Expected event_id 'test-event', got %v", entry.Data["event_id"])
	}
	if entry.Data["transaction_id"] != "test-tx-id" {
		t.Errorf("Expected transaction_id 'test-tx-id', got %v", entry.Data["transaction_id"])
	}
	if entry.Data["span_id"] != "test-span-id" {
		t.Errorf("Expected span_id 'test-span-id', got %v", entry.Data["span_id"])
	}
	if entry.Data["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field 'custom_value', got %v", entry.Data["custom_field"])
	}
}

func TestLogTiming(t *testing.T) {
	// Create a test hook and attach it to our global logger
	hook := test.NewLocal(log)

	// Create context with IDs
	ctx := context.WithValue(context.Background(), CtxKeyTxID, "timing-tx-id")
	ctx = context.WithValue(ctx, CtxKeySpanID, "timing-span-id")

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Test timing log
	elapsed := 250 * time.Millisecond
	LogTiming(ctx, req, 200, elapsed)

	// Check that a log entry was created
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	if entry == nil {
		t.Error("LastEntry() returned nil")
		return
	}

	// Check message
	if entry.Message != "request completed" {
		t.Errorf("Expected message 'request completed', got %v", entry.Message)
	}

	// Check timing fields
	if entry.Data["event_id"] != "timing" {
		t.Error("Expected event_id to be 'timing'")
	}
	if entry.Data["path"] != "/api/test" {
		t.Errorf("Expected path '/api/test', got %v", entry.Data["path"])
	}
	if entry.Data["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", entry.Data["method"])
	}
	if entry.Data["status"] != 200 {
		t.Errorf("Expected status 200, got %v", entry.Data["status"])
	}
	if entry.Data["elapsed"] != "250ms" {
		t.Errorf("Expected elapsed '250ms', got %v", entry.Data["elapsed"])
	}
}

func TestStatusCodeFromError(t *testing.T) {
	// Test with no error
	status := statusCodeFromError(nil)
	if status != "OK" {
		t.Errorf("Expected 'OK' for nil error, got %v", status)
	}

	// Test with error
	err := &testError{msg: "test error"}
	status = statusCodeFromError(err)
	if status != "test error" {
		t.Errorf("Expected 'test error', got %v", status)
	}
}

// Helper type for testing errors
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestPropagateToGRPC(t *testing.T) {
	// Test with context containing IDs
	ctx := context.WithValue(context.Background(), CtxKeyTxID, "propagate-tx-id")
	ctx = context.WithValue(ctx, CtxKeySpanID, "propagate-span-id")

	newCtx := PropagateToGRPC(ctx)

	// Extract metadata from the new context
	md, ok := metadata.FromOutgoingContext(newCtx)
	if !ok {
		t.Error("PropagateToGRPC didn't add metadata to context")
	}

	// Check that transaction ID was added to metadata
	txIDs := md.Get("X-Transaction-ID")
	if len(txIDs) != 1 || txIDs[0] != "propagate-tx-id" {
		t.Errorf("Expected X-Transaction-ID 'propagate-tx-id', got %v", txIDs)
	}

	// Check that span ID was NOT added to metadata (new design)
	spanIDs := md.Get("X-Span-ID")
	if len(spanIDs) != 0 {
		t.Errorf("Expected no X-Span-ID in metadata, but got %v", spanIDs)
	}

	// Test with empty context (should return same context)
	emptyCtx := context.Background()
	sameCtx := PropagateToGRPC(emptyCtx)

	if sameCtx != emptyCtx {
		t.Error("PropagateToGRPC should return same context when no IDs present")
	}
}
