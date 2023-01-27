package log

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	onceZerolog   sync.Once
	zerologLogger zerolog.Logger
)

// usage:
//
//	ctx, span := tracer.Start(ctx, "multiply")
//	l := NewZerolog(ctx)
//	l.Info().Msg("hello world")
func NewZerolog(ctx context.Context) zerolog.Logger {
	onceZerolog.Do(func() {
		zerolog.TimeFieldFormat = time.RFC3339Nano
		l := zerolog.
			New(os.Stdout).
			With().
			Timestamp().
			Caller().
			Str("app", "my_demo_app").
			Logger()
		zerologLogger = l
	})

	return zerologLogger.Hook(zerologTraceHook(ctx))
}

// zerologTraceHook is a hook that;
// (a) adds TraceIds & spanIds to logs of all LogLevels
// (b) adds logs to the active span as events.
func zerologTraceHook(ctx context.Context) zerolog.HookFunc {
	return func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.NoLevel {
			return
		}
		if !e.Enabled() {
			return
		}

		if ctx == nil {
			return
		}

		span := trace.SpanFromContext(ctx)
		if !span.IsRecording() {
			return
		}

		{ // (a) adds TraceIds & spanIds to logs
			//
			// TODO: (komuw) add stackTraces maybe.
			//
			sCtx := span.SpanContext()
			if sCtx.HasTraceID() {
				e.Str("traceId", sCtx.TraceID().String())
			}
			if sCtx.HasSpanID() {
				e.Str("spanId", sCtx.SpanID().String())
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
			attrs = append(attrs, logSeverityKey.String(level.String()))
			attrs = append(attrs, logMessageKey.String(message))

			// todo: add caller info.

			span.AddEvent("log", trace.WithAttributes(attrs...))
			if level >= zerolog.ErrorLevel {
				span.SetStatus(codes.Error, message)
			}
		}
	}
}
