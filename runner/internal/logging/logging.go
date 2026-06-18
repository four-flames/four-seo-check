package logging

import (
	"io"
	"log/slog"
	"os"
)

// New creates a JSON slog.Logger with the given level writing to output.
func New(level slog.Level, output io.Writer) *slog.Logger {
	h := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(h)
}

// Default returns a text handler at Info level writing to stderr.
func Default() *slog.Logger {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(h)
}
