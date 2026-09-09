// Package logger configures the process-wide zerolog logger.
package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup configures the global logger. In dev it writes human-readable console
// output; otherwise structured JSON.
func Setup(level string, dev bool) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var l zerolog.Logger
	if dev {
		l = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"})
	} else {
		l = zerolog.New(os.Stderr)
	}
	l = l.With().Timestamp().Caller().Logger()
	log.Logger = l
	return l
}
