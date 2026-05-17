package logging

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type Logger struct {
	logger *slog.Logger
}

func New(service, region, appEnv string) *Logger {
	var logger *slog.Logger

	if appEnv != "local" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	return &Logger{
		logger: logger.With(
			slog.String("service", service),
			slog.String("region", region),
		),
	}
}

func (l *Logger) WithVideoID(videoID string) *Logger {
	return &Logger{logger: l.logger.With(slog.String("video_id", videoID))}
}

func (l *Logger) WithSpanContext(ctx context.Context) *Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return &Logger{logger: l.logger}
	}
	return &Logger{logger: l.logger.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)}
}

func (l *Logger) Info(ctx context.Context, eventType, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelInfo, eventType, msg, attrs...)
}

func (l *Logger) Error(ctx context.Context, eventType, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelError, eventType, msg, attrs...)
}

func (l *Logger) Debug(ctx context.Context, eventType, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelDebug, eventType, msg, attrs...)
}

func (l *Logger) Warn(ctx context.Context, eventType, msg string, attrs ...slog.Attr) {
	l.log(ctx, slog.LevelWarn, eventType, msg, attrs...)
}

func (l *Logger) log(ctx context.Context, level slog.Level, eventType, msg string, attrs ...slog.Attr) {
	attrs = append([]slog.Attr{slog.String("event_type", eventType)}, attrs...)
	l.logger.LogAttrs(ctx, level, msg, attrs...)
}
