// Package log configures the structured logger shared by user service components.
package log

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger aliases zerolog.Logger for the service logging boundary.
type Logger = zerolog.Logger

// Fields is a set of structured values attached to a log event.
type Fields map[string]interface{}

// New returns the service logger configured for the requested environment.
func New(env string) Logger {
	level := zerolog.InfoLevel
	if env == "local" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	return log.Level(level)
}

// With returns a logger enriched with the supplied structured fields.
func With(logger Logger, fields Fields) Logger {
	event := logger
	for k, v := range fields {
		event = event.With().Interface(k, v).Logger()
	}
	return event
}
