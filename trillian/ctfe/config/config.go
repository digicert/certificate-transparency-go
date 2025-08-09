package config

import (
	"github.com/digicert/ctutils/logging"
	"github.com/digicert/ctutils/logging/adapters"
)

// InitLogging sets up the logging adapter for CTFE.
func InitLogging() {
	// Initialize OpenTelemetry
	if err := logging.InitOpenTelemetry("ctfe"); err != nil {
		panic(err)
	}

	// Initialize logging adapter with JSON format
	logConfig := logging.Config{Level: logging.InfoLevel, Format: "json"}
	adapter := adapters.NewLogrusAdapter(logConfig)
	logging.SetLoggerAdapter(adapter)
}
