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

// Also see: https://github.com/jba/slog/blob/main/trace/trace.go

// usage:
//
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := NewSlog(ctx)
//	l.Info("hello world")
func NewSlog(ctx context.Context) *slog.Logger {
	onceSlog.Do(func() {
		opts := slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}
		jh := opts.NewJSONHandler(os.Stdout)

		h := otelHandler{h: jh}
		l := slog.New(h).With("app", "my_demo_app")
		slogLogger = l
	})

	return slogLogger.WithContext(ctx)
}

// otelHandler implements slog.Handler
// It adds;
// (a) TraceIds & spanIds to logs.
// (b) Logs(as events) to the active span.
type otelHandler struct{ h slog.Handler }

func (s otelHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true /* support all logging levels*/
}

func (s otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &otelHandler{h: s.h.WithAttrs(attrs)}
}

func (s otelHandler) WithGroup(name string) slog.Handler {
	return &otelHandler{h: s.h.WithGroup(name)}
}

func (s otelHandler) Handle(r slog.Record) (err error) {
	ctx := r.Context
	if ctx == nil {
		return s.h.Handle(r)
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return s.h.Handle(r)
	}

	{ // (a) adds TraceIds & spanIds to logs.
		//
		// TODO: (komuw) add stackTraces maybe.
		//
		sCtx := span.SpanContext()
		attrs := make([]slog.Attr, 0)
		if sCtx.HasTraceID() {
			attrs = append(attrs,
				slog.Attr{Key: "traceId", Value: slog.StringValue(sCtx.TraceID().String())},
			)
		}
		if sCtx.HasSpanID() {
			attrs = append(attrs,
				slog.Attr{Key: "spanId", Value: slog.StringValue(sCtx.SpanID().String())},
			)
		}
		if len(attrs) > 0 {
			r.AddAttrs(attrs...)
		}
	}

	{ // (b) adds logs to the active span as events.

		// code from: https://github.com/uptrace/opentelemetry-go-extra/tree/main/otellogrus
		// which is BSD 2-Clause license.

		attrs := make([]attribute.KeyValue, 0)

		logSeverityKey := attribute.Key("log.severity")
		logMessageKey := attribute.Key("log.message")
		attrs = append(attrs, logSeverityKey.String(r.Level.String()))
		attrs = append(attrs, logMessageKey.String(r.Message))

		// TODO: Obey the following rules form the slog documentation:
		//
		// Handle methods that produce output should observe the following rules:
		//   - If r.Time is the zero time, ignore the time.
		//   - If an Attr's key is the empty string, ignore the Attr.
		//
		r.Attrs(func(a slog.Attr) {
			attrs = append(attrs,
				attribute.KeyValue{
					Key:   attribute.Key(a.Key),
					Value: attribute.StringValue(a.Value.String()),
				},
			)
		})
		// todo: add caller info.

		span.AddEvent("log", trace.WithAttributes(attrs...))
		if r.Level >= slog.LevelError {
			span.SetStatus(codes.Error, r.Message)
		}
	}

	return s.h.Handle(r)
}
