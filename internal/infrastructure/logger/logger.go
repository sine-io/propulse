package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/sine-io/propulse/internal/infrastructure/config"
)

func New(cfg config.LogConfig) zerolog.Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}

	var output io.Writer = os.Stdout
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{Out: os.Stdout}
	}

	log := zerolog.New(output).Level(level).With().Timestamp().Logger()

	return log
}
