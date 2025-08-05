// SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
//
// SPDX-License-Identifier: MIT

// Package logger provides logging functionality for the params2env tool.
//
// It wraps the standard library's log/slog package to provide consistent logging
// across the application with configurable log levels. The package supports
// debug, info, warn, and error levels, defaulting to info if an invalid level
// is specified.
package logger

import (
	"log/slog"
	"os"
)

// InitLogger initializes and returns a new slog.Logger with the specified log level.
// It also sets this logger as the default global logger.
//
// The level parameter is case-insensitive and can be one of:
//   - "debug": Most verbose level, includes detailed debugging information
//   - "info": Standard log level for general operational information (default)
//   - "warn": Warnings and potentially harmful situations
//   - "error": Error conditions that should be addressed
//
// If an invalid level is provided, it defaults to "info".
//
// Example usage:
//
//	logger := InitLogger("debug")
//	logger.Debug("Detailed information", "key", "value")
//	logger.Info("General information")
//	logger.Warn("Warning message")
//	logger.Error("Error condition", "error", err)
func InitLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug", "DEBUG":
		logLevel = slog.LevelDebug
	case "info", "INFO":
		logLevel = slog.LevelInfo
	case "warn", "WARN":
		logLevel = slog.LevelWarn
	case "error", "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
