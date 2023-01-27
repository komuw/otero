package log

import (
	"context"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

var (
	onceSlog   sync.Once
	slogLogger *slog.Logger
)

func NewSlog(ctx context.Context) *slog.Logger {
	onceSlog.Do(func() {
		opts := slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}
		jh := opts.NewJSONHandler(os.Stdout)

		h := slogHandler{h: jh}
		l := slog.New(h).With("app", "my_demo_app")
		slogLogger = l
	})

	return slogLogger.WithContext(ctx)
}

// slogHandler implements the slog.Handler
// It;
// (a) adds TraceIds & spanIds to logs.
// (b) adds logs to the active span as events.
type slogHandler struct {
	h slog.Handler
}

func (s slogHandler) Enabled(_ slog.Level) bool { return true /* support all logging levels*/ }
func (s slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogHandler{h: s.h.WithAttrs(attrs)}
}

func (s slogHandler) WithGroup(name string) slog.Handler {
	return &slogHandler{h: s.h.WithGroup(name)}
}

func (s slogHandler) Handle(r slog.Record) (err error) {
	ctx := r.Context
	if ctx == nil {
		return s.h.Handle(r)
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return s.h.Handle(r)
	}

	{ // (a) adds TraceIds & spanIds to logs
		//
		// TODO: (komuw) add stackTraces maybe.
		//
		sCtx := span.SpanContext()
		attrs := make([]slog.Attr, 0)
		if sCtx.HasTraceID() {
			attrs = append(attrs, slog.Attr{Key: "traceId", Value: slog.StringValue(sCtx.TraceID().String())})
		}
		if sCtx.HasSpanID() {
			attrs = append(attrs, slog.Attr{Key: "spanId", Value: slog.StringValue(sCtx.SpanID().String())})
		}
		if len(attrs) > 0 {
			r.AddAttrs(attrs...)
		}
	}

	{ // (b) adds logs to the active span as events.

		// code from: https://github.com/uptrace/opentelemetry-go-extra/tree/main/otellogrus
		// which is BSD 2-Clause license.

		// Unlike logrus, zerolog does not give hooks the ability to get the whole event/message with all its key-values
		// see: https://github.com/rs/zerolog/issues/300

		attrs := make([]attribute.KeyValue, 0)

		logSeverityKey := attribute.Key("log.severity")
		logMessageKey := attribute.Key("log.message")
		attrs = append(attrs, logSeverityKey.String(r.Level.String()))
		attrs = append(attrs, logMessageKey.String(r.Message))

		// todo: add caller info.

		span.AddEvent("log", trace.WithAttributes(attrs...))
		if r.Level >= slog.LevelError {
			span.SetStatus(codes.Error, r.Message)
		}
	}

	return s.h.Handle(r)
}
