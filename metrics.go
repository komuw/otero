package main

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"google.golang.org/grpc/credentials"
)

func getMeter() metric.Meter {
	/*
		// https://uptrace.dev/opentelemetry/metrics.html#instruments
		You capture measurements by creating instruments that have:
		- An unique name, for example, http.server.duration.
		- An instrument kind, for example, Histogram.
		- An optional unit of measure, for example, milliseconds or bytes.
		- An optional description.
	*/
	return global.MeterProvider().Meter(
		"instrumentation/package/name",
		metric.WithInstrumentationVersion("0.0.1"),
	)
}

// For how to use prometheus instead of stdout
// see: https://github.com/banked/GopherConUK2021/blob/0d737737dfad3c5fda08f7b730587265a36bf747/demo5/main.go#L33-L65
func setupMetrics(ctx context.Context, serviceName string) (*sdkmetric.MeterProvider, error) {
	// labels/tags that aew common to all metrics.
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String("0.0.1"),
		semconv.DeploymentEnvironmentKey.String("staging"),
		attribute.String("name", "komu"),
	)

	/*
		Alternative ways of providing an exporter:

		(a)
		import "go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
		exporter, err := stdoutmetric.New()

		(b)
		import "go.opentelemetry.io/otel/exporters/prometheus"
		exporter, err := prometheus.New()
	*/

	c, err := getTls()
	if err != nil {
		return nil, err
	}

	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint("otel_collector:4317"),
		otlpmetricgrpc.WithTLSCredentials(
			// mutual tls.
			credentials.NewTLS(c),
		),
		// You can use `WithInsecure` for non-production purposes.
		// otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(2*time.Second)),
		),

		// sdkmetric.WithView(sdkmetric.NewView(
		// 	sdkmetric.Instrument{Name: "some_latency"},
		// 	// import "go.opentelemetry.io/otel/sdk/metric/aggregation"
		// 	sdkmetric.Stream{Aggregation: aggregation.ExplicitBucketHistogram{
		// 		// this floats define the distribution bucket boundaries for the histogram of `some_latency` metric
		// 		// Bucket boundaries are 10ms, 100ms, 1s, 10s, 30s and 60s.
		// 		Boundaries: []float64{10, 100, 1000, 10000, 30000, 60000},
		// 	}},
		// )),
	)
	global.SetMeterProvider(mp)

	/*
		// https://uptrace.dev/opentelemetry/metrics.html#instruments
		You capture measurements by creating instruments that have:
		- An unique name, for example, http.server.duration.
		- An instrument kind, for example, Histogram.
		- An optional unit of measure, for example, milliseconds or bytes.
		- An optional description.
	*/

	return mp, nil
}
