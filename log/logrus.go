package log

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	onceLogrus   sync.Once
	logrusLogger *logrus.Entry
)

// usage:
//
//	ctx, span := tracer.Start(ctx, "myFuncName")
//	l := NewLogrus(ctx)
//	l.Info("hello world")
func NewLogrus(ctx context.Context) *logrus.Entry {
	onceLogrus.Do(func() {
		l := logrus.New()
		l.SetLevel(logrus.TraceLevel)
		l.Formatter = &logrus.JSONFormatter{
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "severity",
				logrus.FieldKeyMsg:   "message",
			},
			TimestampFormat: time.RFC3339Nano,
		}
		l.AddHook(logrusTraceHook{})
		l.SetReportCaller(true)
		logrusLogger = l.WithField("app", "my_demo_app")
	})

	return logrusLogger.WithContext(ctx)
}

// logrusTraceHook is a hook that;
// (a) adds TraceIds & spanIds to logs of all LogLevels
// (b) adds logs to the active span as events.
type logrusTraceHook struct{}

// Levels define on which log levels this hook would trigger
func (t logrusTraceHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire will be called when some logging function is called with current hook
// It will;
// (a) adds TraceIds & spanIds to logs of all LogLevels
// (b) adds logs to the active span as events.
func (t logrusTraceHook) Fire(entry *logrus.Entry) error {
	ctx := entry.Context
	if ctx == nil {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return nil
	}

	{ // (a) adds TraceIds & spanIds to logs.
		//
		// TODO: (komuw) add stackTraces maybe.
		//
		sCtx := span.SpanContext()
		if sCtx.HasTraceID() {
			entry.Data["traceId"] = sCtx.TraceID().String()
		}
		if sCtx.HasSpanID() {
			entry.Data["spanId"] = sCtx.SpanID().String()
		}
	}

	{ // (b) adds logs to the active span as events.

		// code from: https://github.com/uptrace/opentelemetry-go-extra/tree/main/otellogrus
		// whose license(BSD 2-Clause) can be found at: https://github.com/uptrace/opentelemetry-go-extra/blob/v0.1.18/LICENSE

		attrs := make([]attribute.KeyValue, 0, len(entry.Data)+2+3)

		logSeverityKey := attribute.Key("log.severity")
		logMessageKey := attribute.Key("log.message")
		attrs = append(attrs, logSeverityKey.String(levelString(entry.Level)))
		attrs = append(attrs, logMessageKey.String(entry.Message))

		if entry.Caller != nil {
			if entry.Caller.Function != "" {
				attrs = append(attrs, semconv.CodeFunctionKey.String(entry.Caller.Function))
			}
			if entry.Caller.File != "" {
				attrs = append(attrs, semconv.CodeFilepathKey.String(entry.Caller.File))
				attrs = append(attrs, semconv.CodeLineNumberKey.Int(entry.Caller.Line))
			}
		}

		for k, v := range entry.Data {
			if k == "error" {
				if err, ok := v.(error); ok {
					typ := reflect.TypeOf(err).String()
					attrs = append(attrs, semconv.ExceptionTypeKey.String(typ))
					attrs = append(attrs, semconv.ExceptionMessageKey.String(err.Error()))
					continue
				}
			}

			attrs = append(attrs, toAttrKV(k, v))
		}

		span.AddEvent("log", trace.WithAttributes(attrs...))

		if entry.Level <= logrus.ErrorLevel {
			span.SetStatus(codes.Error, entry.Message)
		}
	}

	return nil
}

func levelString(lvl logrus.Level) string {
	s := lvl.String()
	if s == "warning" {
		s = "warn"
	}
	return strings.ToUpper(s)
}

func toAttrKV(key string, value interface{}) attribute.KeyValue {
	switch value := value.(type) {
	case nil:
		return attribute.String(key, "<nil>")
	case string:
		return attribute.String(key, value)
	case int:
		return attribute.Int(key, value)
	case int64:
		return attribute.Int64(key, value)
	case uint64:
		return attribute.Int64(key, int64(value))
	case float64:
		return attribute.Float64(key, value)
	case bool:
		return attribute.Bool(key, value)
	case fmt.Stringer:
		return attribute.String(key, value.String())
	}

	rv := reflect.ValueOf(value)

	switch rv.Kind() {
	case reflect.Array:
		rv = rv.Slice(0, rv.Len())
		fallthrough
	case reflect.Slice:
		switch reflect.TypeOf(value).Elem().Kind() {
		case reflect.Bool:
			return attribute.BoolSlice(key, rv.Interface().([]bool))
		case reflect.Int:
			return attribute.IntSlice(key, rv.Interface().([]int))
		case reflect.Int64:
			return attribute.Int64Slice(key, rv.Interface().([]int64))
		case reflect.Float64:
			return attribute.Float64Slice(key, rv.Interface().([]float64))
		case reflect.String:
			return attribute.StringSlice(key, rv.Interface().([]string))
		default:
			return attribute.KeyValue{Key: attribute.Key(key)}
		}
	case reflect.Bool:
		return attribute.Bool(key, rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return attribute.Int64(key, rv.Int())
	case reflect.Float64:
		return attribute.Float64(key, rv.Float())
	case reflect.String:
		return attribute.String(key, rv.String())
	}
	if b, err := json.Marshal(value); b != nil && err == nil {
		return attribute.String(key, string(b))
	}
	return attribute.String(key, fmt.Sprint(value))
}
