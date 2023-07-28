package log

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

var slogLogger *slog.Logger

// Also see: https://github.com/jba/slog/blob/main/trace/trace.go

// NewSlog usage:
//
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := NewSlog(ctx)
//	l.Info("hello world")
func NewSlog(ctx context.Context) *slog.Logger {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}
	jh := slog.NewJSONHandler(os.Stdout, &opts)

	h := otelHandler{h: jh, ctx: ctx}
	l := slog.New(h).With("app", "my_demo_app")
	slogLogger = l

	traceId, spanId := getIds(ctx)
	if traceId != "" && spanId != "" {
		return slogLogger.With("traceId", traceId, "spanId", spanId)
	}

	return slogLogger
}

// otelHandler implements slog.Handler
// It adds;
// (a) TraceIds & spanIds to logs.
// (b) Logs(as events) to the active span.
type otelHandler struct {
	h slog.Handler
	// Do not store Contexts inside a struct type; https://pkg.go.dev/context
	// todo: do better in future.
	ctx context.Context
}

func (s otelHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true /* support all logging levels*/
}

func (s otelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return otelHandler{h: s.h.WithAttrs(attrs), ctx: s.ctx}
}

func (s otelHandler) WithGroup(name string) slog.Handler {
	return otelHandler{h: s.h.WithGroup(name), ctx: s.ctx}
}

func (s otelHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		span = trace.SpanFromContext(s.ctx)
	}

	if !span.IsRecording() {
		return s.h.Handle(ctx, r)
	}

	{ // (a) adds TraceIds & spanIds to logs.
		//
		// TODO:
		//   (a) (komuw) add stackTraces maybe.
		//   (b) there's a bug here, where sometimes duplicate traceId/spanId can be added.
		//
		sCtx := span.SpanContext()
		attrs := make([]slog.Attr, 0)
		traceId, spanId := getIds(ctx)
		if traceId != "" && spanId != "" {
			attrs = append(attrs,
				slog.Attr{Key: "traceId", Value: slog.StringValue(sCtx.TraceID().String())},
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
		r.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs,
				attribute.KeyValue{
					Key:   attribute.Key(a.Key),
					Value: attribute.StringValue(a.Value.String()),
				},
			)
			return true
		})
		// todo: add caller info.

		span.AddEvent("log", trace.WithAttributes(attrs...))
		if r.Level >= slog.LevelError {
			span.SetStatus(codes.Error, r.Message)
		}
	}

	return s.h.Handle(ctx, r)
}

func getIds(ctx context.Context) (traceId, spanId string) {
	if ctx == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	sCtx := span.SpanContext()

	if sCtx.HasTraceID() {
		traceId = sCtx.TraceID().String()
	}
	if sCtx.HasSpanID() {
		spanId = sCtx.SpanID().String()
	}

	return traceId, spanId
}
