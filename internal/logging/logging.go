// Package logging provides structured logging with simultaneous file and console output.
package logging

import (
	"context"
	"log/slog"
	"os"
)

var logger *slog.Logger

// Init initialises the package-level logger with the given level and optional file path.
// If filePath is empty, logs only to stdout.
func Init(level, filePath string) error {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	if filePath == "" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
		return nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	jsonHandler := slog.NewJSONHandler(f, opts)
	textHandler := slog.NewTextHandler(os.Stdout, opts)
	logger = slog.New(&multiHandler{handlers: []slog.Handler{jsonHandler, textHandler}})
	return nil
}

// Debug logs at debug level.
func Debug(msg string, args ...any) { ensureLogger(); logger.Debug(msg, args...) }

// Info logs at info level.
func Info(msg string, args ...any) { ensureLogger(); logger.Info(msg, args...) }

// Warn logs at warn level.
func Warn(msg string, args ...any) { ensureLogger(); logger.Warn(msg, args...) }

// Error logs at error level.
func Error(msg string, args ...any) { ensureLogger(); logger.Error(msg, args...) }

func ensureLogger() {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}

// multiHandler fans out log records to multiple slog.Handler instances.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: hs}
}
