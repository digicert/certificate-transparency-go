package config

import (
	"github.com/digicert/ctutils/logging"
	"github.com/digicert/ctutils/logging/adapters"
)

// InitLogging sets up the logging adapter for CTFE.
func InitLogging() {
	// Initialize OpenTelemetry with config struct
	logging.InitOpenTelemetry(logging.TelemetryConfigFromEnv())

	// Initialize logging adapter with JSON format
	logCfg := logging.Config{Format: "json"}
	adapter := adapters.NewLogrusAdapter(logCfg)
	logging.SetLoggerAdapter(adapter)
}
