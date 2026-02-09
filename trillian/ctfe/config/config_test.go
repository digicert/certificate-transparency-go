package config

import (
	"testing"

	"github.com/digicert/ctutils/logging"
)

func TestInitLogging(t *testing.T) {
	tests := []struct {
		name      string
		logFormat string
		logLevel  string
	}{
		{"Default", "", ""},
		{"JSON_Debug", "json", "DEBUG"},
		{"Text_Error", "text", "ERROR"},
		{"Text_Info", "text", "INFO"},
		{"Invalid_Formats", "invalid", "INVALID"}, // Should fallback to defaults
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.logFormat != "" {
				t.Setenv("LOG_FORMAT", tt.logFormat)
			}
			if tt.logLevel != "" {
				t.Setenv("LOG_LEVEL", tt.logLevel)
			}

			// Verify shutdown function is returned and works
			shutdown := InitLogging()
			if shutdown == nil {
				t.Fatal("InitLogging() should return non-nil shutdown function")
			}
			defer shutdown() // Verify shutdown doesn't panic

			// Check that a logger is set (not nil)
			if logging.GetLogger() == nil {
				t.Error("InitLogging() failed to set global logger")
			}
		})
	}
}
