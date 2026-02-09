package config

import (
	"os"

	"github.com/digicert/ctutils/logging"
	"github.com/digicert/ctutils/logging/adapters"
)

// InitLogging sets up the logging adapter for CTFE.
// Note: This function is fail-safe. If OpenTelemetry initialization fails,
// it logs an error and falls back to a no-op tracer provider to ensure the binary continues running.
// Returns a shutdown function that should be deferred in main().
func InitLogging() func() {
	// Initialize OpenTelemetry with config struct
	shutdown := logging.InitOpenTelemetry(logging.TelemetryConfigFromEnv())

	// Read logger backend configuration from env vars
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text" // Default to text for human readability if not specified
	}

	logLevelStr := os.Getenv("LOG_LEVEL")
	var logLevel logging.LogLevel
	switch logLevelStr {
	case "DEBUG":
		logLevel = logging.DebugLevel
	case "INFO":
		logLevel = logging.InfoLevel
	case "WARN":
		logLevel = logging.WarnLevel
	case "ERROR":
		logLevel = logging.ErrorLevel
	default:
		logLevel = logging.InfoLevel
	}

	logCfg := logging.Config{
		Format: logFormat,
		Level:  logLevel,
	}
	adapter := adapters.NewLogrusAdapter(logCfg)
	logging.SetLoggerAdapter(adapter)

	return shutdown
}
