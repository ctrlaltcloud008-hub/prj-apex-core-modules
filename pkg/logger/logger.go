package logger

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

type Logger struct {
	logger  *slog.Logger
	service string
}

// replaceAttrGCP remaps slog's default "level" key to the "severity" field that
// GCP Cloud Logging reads, and translates slog level strings to GCP's
// LogSeverity enum names (e.g. "WARN" -> "WARNING").
func replaceAttrGCP(groups []string, a slog.Attr) slog.Attr {
	if a.Key != slog.LevelKey {
		return a
	}
	a.Key = "severity"
	if level, ok := a.Value.Any().(slog.Level); ok {
		switch {
		case level < slog.LevelInfo:
			a.Value = slog.StringValue("DEBUG")
		case level < slog.LevelWarn:
			a.Value = slog.StringValue("INFO")
		case level < slog.LevelError:
			a.Value = slog.StringValue("WARNING")
		default:
			a.Value = slog.StringValue("ERROR")
		}
	}
	return a
}

func New(service, region, appEnv string) *Logger {

	var logger *slog.Logger
	if appEnv != "local" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: replaceAttrGCP,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	return &Logger{
		service: service,
		logger: logger.With(
			slog.String("service", service),
			slog.String("region", region),
			slog.String("app_env", appEnv),
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
	sc := trace.SpanContextFromContext(ctx)
	base := []slog.Attr{
		slog.String("event_type", eventType),
		slog.String("actor", l.service),
	}
	if sc.IsValid() {
		base = append(base,
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	attrs = append(base, attrs...)
	l.logger.LogAttrs(ctx, level, msg, attrs...)
}
