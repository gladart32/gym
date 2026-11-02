package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/gladart32/gym/internal/config"
)

// New builds a slog logger from config. Supported outputs: stdout, file
// (rotated via lumberjack). Returns a closer that must be called on shutdown.
func New(cfg config.Logger) (*slog.Logger, io.Closer, error) {
	var writers []io.Writer
	var closers []io.Closer

	for _, out := range cfg.Outputs {
		switch strings.ToLower(out) {
		case "stdout":
			writers = append(writers, os.Stdout)
		case "file":
			if cfg.File.Path == "" {
				return nil, nil, errors.New("logger: file output requires logger.file.path")
			}
			if err := os.MkdirAll(filepath.Dir(cfg.File.Path), 0o755); err != nil {
				return nil, nil, fmt.Errorf("logger: create log dir: %w", err)
			}
			lj := &lumberjack.Logger{
				Filename:   cfg.File.Path,
				MaxSize:    cfg.File.MaxSizeMB,
				MaxAge:     cfg.File.MaxAgeDays,
				MaxBackups: cfg.File.MaxBackups,
				Compress:   cfg.File.Compress,
			}
			writers = append(writers, lj)
			closers = append(closers, lj)
		default:
			return nil, nil, fmt.Errorf("logger: unknown output %q (expected stdout or file)", out)
		}
	}

	if len(writers) == 0 {
		return nil, nil, errors.New("logger: at least one output is required")
	}

	var w io.Writer = writers[0]
	if len(writers) > 1 {
		w = io.MultiWriter(writers...)
	}

	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	log := slog.New(handler)
	slog.SetDefault(log)

	return log, multiCloser(closers), nil
}

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var errs []error
	for _, c := range m {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
