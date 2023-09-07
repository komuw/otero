package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc/credentials"
)

func setupTracing(ctx context.Context, serviceName string) (*trace.TracerProvider, error) {
	/*
		Alternative ways of providing an exporter:
		see: https://github.com/open-telemetry/opentelemetry-go/tree/v1.2.0/exporters

		(a)
		import "go.opentelemetry.io/otel/exporters/jaeger"
		exporter, err := jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint("http://jaeger:14268/api/traces")),
		)

		(b)
		import "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	*/

	c, err := getTls()
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint("otel_collector:4317"),
		otlptracegrpc.WithTLSCredentials(
			// mutual tls.
			credentials.NewTLS(c),
		),
		// You can use `WithInsecure` for non-production purposes.
		// otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// labels/tags that are common to all traces.
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String("0.0.1"),
		semconv.DeploymentEnvironmentKey.String("staging"),
		attribute.String("name", "komu"),
	)

	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter), // use batch in prod.
		trace.WithResource(resource),
		trace.WithSpanProcessor(loggingSpanProcessor{}),
		trace.WithSampler(
			// Sample 30%
			//
			// There's head-based sampling and tail-based sampling.
			// Tail based sampling would enable you to say something like;
			// `Sample 5% of success but 100% of all the errors.`
			//
			// What we have implemented here is head-based sampling.
			// See: https://github.com/komuw/otero/issues/11 (and the links therein)
			trace.ParentBased(trace.TraceIDRatioBased(0.3)),
		),
	)

	/*
	   When the tracer provider is created, we need to set it as the global tracer provider:
	   This ensures that if someone uses the global tracer like;
	       ctx, span := otel.Tracer("my-telemetry-library").Start(r.Context(), "get_user_cart")
	       defer span.End()
	   Then, they will always use our provider-tracer.
	*/
	otel.SetTracerProvider(provider)

	/*
		Alternative ways of providing a propagator:
		  (a)
			propagator := ot.OT{}
			otel.SetTextMapPropagator(propagator)

		  (b)
		    import "go.opentelemetry.io/contrib/propagators/b3"
			otel.SetTextMapPropagator(
			  b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader))
		    )
	*/
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return provider, nil
}

// loggingSpanProcessor logs at end of a span.
type loggingSpanProcessor struct{}

func (c loggingSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
	// TODO: (komuw) merge s.Attributes() +  s.Resource() + s.Events()[maybe]
	// attrSet := attribute.NewSet(s.Attributes()...)
	// log.Println("\n\n\t onEnd called.",
	// 	"s.Name(): ", s.Name(),
	// 	"TraceID: ", s.SpanContext().TraceID(),
	// 	"SpanID: ", s.SpanContext().SpanID(),
	// 	"duration: ", s.EndTime().Sub(s.StartTime()),
	// 	"s.Attributes(): ", attrSet.Encoded(attribute.DefaultEncoder()),
	// 	"s.Resource(): ", s.Resource(),
	// 	// events is where errorStacktraces(if any) are recorded.
	// 	"s.Events(): ", s.Events(),
	// )
}

func (c loggingSpanProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {}
func (c loggingSpanProcessor) ForceFlush(ctx context.Context) error                  { return nil }
func (c loggingSpanProcessor) Shutdown(ctx context.Context) error                    { return nil }

// getTls returns a configuration that enables the use of mutual TLS.
func getTls() (*tls.Config, error) {
	clientAuth, err := tls.LoadX509KeyPair("./confs/tls/client.crt", "./confs/tls/client.key")
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile("./confs/tls/rootCA.crt")
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	c := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientAuth},
	}

	return c, nil
}
