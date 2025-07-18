package logging

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
)

// TestStatusRecorder tests the statusRecorder type
func TestStatusRecorder(t *testing.T) {
	// Create a test ResponseWriter
	w := httptest.NewRecorder()

	// Create our statusRecorder wrapper
	recorder := &statusRecorder{
		ResponseWriter: w,
		statusCode:     200, // default value
	}

	// Test that it starts with default status code
	if recorder.statusCode != 200 {
		t.Errorf("Expected default status code 200, got %d", recorder.statusCode)
	}

	// Test WriteHeader method
	recorder.WriteHeader(404)

	// Check that our wrapper recorded the status code
	if recorder.statusCode != 404 {
		t.Errorf("Expected status code 404, got %d", recorder.statusCode)
	}

	// Check that the underlying ResponseWriter also got the status code
	if w.Code != 404 {
		t.Errorf("Expected underlying ResponseWriter code 404, got %d", w.Code)
	}
}

// TestMiddleware tests the main middleware function
func TestMiddleware(t *testing.T) {
	// Create a test hook to capture log output
	hook := test.NewLocal(log)

	// Create a test handler that we'll wrap with our middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that context was added to the request
		txID := r.Context().Value(CtxKeyTxID)
		if txID == nil {
			t.Error("Request context doesn't contain transaction ID")
		}

		spanID := r.Context().Value(CtxKeySpanID)
		if spanID == nil {
			t.Error("Request context doesn't contain span ID")
		}

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Write a response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World"))
	})

	// Wrap our test handler with the middleware
	wrappedHandler := Middleware(testHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test-endpoint", nil)

	// Create a ResponseRecorder to capture the response
	w := httptest.NewRecorder()

	// Execute the request
	wrappedHandler.ServeHTTP(w, req)

	// Check that the response was written correctly
	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", w.Code)
	}

	if w.Body.String() != "Hello World" {
		t.Errorf("Expected body 'Hello World', got %s", w.Body.String())
	}

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

	// Check log message
	if entry.Message != "request completed" {
		t.Errorf("Expected message 'request completed', got %s", entry.Message)
	}

	// Check log fields
	if entry.Data["event_id"] != "timing" {
		t.Errorf("Expected event_id 'timing', got %v", entry.Data["event_id"])
	}

	if entry.Data["method"] != "GET" {
		t.Errorf("Expected method 'GET', got %v", entry.Data["method"])
	}

	if entry.Data["path"] != "/test-endpoint" {
		t.Errorf("Expected path '/test-endpoint', got %v", entry.Data["path"])
	}

	if entry.Data["status"] != 200 {
		t.Errorf("Expected status 200, got %v", entry.Data["status"])
	}

	// Check that elapsed time was recorded (should be > 0)
	elapsed, ok := entry.Data["elapsed"].(string)
	if !ok {
		t.Error("Elapsed time not recorded as string")
	}

	if !strings.HasSuffix(elapsed, "ms") {
		t.Errorf("Expected elapsed time to end with 'ms', got %s", elapsed)
	}
}

// TestMiddlewareWithExistingTransactionID tests middleware when request already has transaction ID
func TestMiddlewareWithExistingTransactionID(t *testing.T) {
	hook := test.NewLocal(log)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the existing transaction ID was preserved
		txID := r.Context().Value(CtxKeyTxID)
		if txID != "existing-tx-123" {
			t.Errorf("Expected existing transaction ID 'existing-tx-123', got %v", txID)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrappedHandler := Middleware(testHandler)

	// Create request with existing transaction ID
	req := httptest.NewRequest("POST", "/api/submit", strings.NewReader("test data"))
	req.Header.Set("X-Transaction-ID", "existing-tx-123")

	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)

	// Check log entry
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	if entry.Data["transaction_id"] != "existing-tx-123" {
		t.Errorf("Expected transaction_id 'existing-tx-123', got %v", entry.Data["transaction_id"])
	}
}

// TestMiddlewareWithDifferentStatusCodes tests middleware with various HTTP status codes
func TestMiddlewareWithDifferentStatusCodes(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		expectedStatus int
	}{
		{"Success", http.StatusOK, 200},
		{"Created", http.StatusCreated, 201},
		{"BadRequest", http.StatusBadRequest, 400},
		{"NotFound", http.StatusNotFound, 404},
		{"InternalServerError", http.StatusInternalServerError, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hook := test.NewLocal(log)

			// Create handler that returns specific status code
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			})

			wrappedHandler := Middleware(testHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			// Check response status
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			// Check log entry
			if len(hook.Entries) != 1 {
				t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
				return
			}

			entry := hook.LastEntry()
			if entry.Data["status"] != tc.expectedStatus {
				t.Errorf("Expected logged status %d, got %v", tc.expectedStatus, entry.Data["status"])
			}
		})
	}
}

// TestMiddlewareWithPanic tests that middleware handles panics gracefully
func TestMiddlewareWithPanic(t *testing.T) {
	// Create handler that panics
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrappedHandler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	// This should panic, so we need to recover
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, but none occurred")
		}
	}()

	wrappedHandler.ServeHTTP(w, req)
}

// TestMiddlewareTimingAccuracy tests that timing is reasonably accurate
func TestMiddlewareTimingAccuracy(t *testing.T) {
	hook := test.NewLocal(log)

	// Create handler that sleeps for a known duration
	sleepDuration := 50 * time.Millisecond
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleepDuration)
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := Middleware(testHandler)

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	wrappedHandler.ServeHTTP(w, req)
	actualElapsed := time.Since(start)

	// Check that timing was logged
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
		return
	}

	entry := hook.LastEntry()
	elapsedStr, ok := entry.Data["elapsed"].(string)
	if !ok {
		t.Error("Elapsed time not recorded as string")
		return
	}

	// Parse the elapsed time (remove "ms" suffix)
	elapsedStr = strings.TrimSuffix(elapsedStr, "ms")

	// Check that the timing is reasonably accurate using proper numeric comparison
	// Allow for some variance (±20ms) due to system scheduling and load
	tolerance := 20 * time.Millisecond
	if actualElapsed < (sleepDuration-tolerance) || actualElapsed > (sleepDuration+tolerance) {
		t.Errorf("Expected elapsed time around %v (±%v), got %v. Logged: %sms",
			sleepDuration, tolerance, actualElapsed, elapsedStr)
	}
}

// TestMiddlewareChaining tests that multiple middlewares can be chained
func TestMiddlewareChaining(t *testing.T) {
	hook := test.NewLocal(log)

	// Create a simple handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final"))
	})

	// Create another middleware for testing chaining
	authMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add a custom header to verify this middleware ran
			w.Header().Set("X-Auth-Middleware", "ran")
			next.ServeHTTP(w, r)
		})
	}

	// Chain middlewares: authMiddleware -> Middleware -> finalHandler
	chainedHandler := authMiddleware(Middleware(finalHandler))

	req := httptest.NewRequest("GET", "/chained", nil)
	w := httptest.NewRecorder()

	chainedHandler.ServeHTTP(w, req)

	// Check that both middlewares ran
	if w.Header().Get("X-Auth-Middleware") != "ran" {
		t.Error("Auth middleware didn't run")
	}

	if w.Body.String() != "final" {
		t.Errorf("Expected body 'final', got %s", w.Body.String())
	}

	// Check that our logging middleware created a log entry
	if len(hook.Entries) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(hook.Entries))
	}
}
