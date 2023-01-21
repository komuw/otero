package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/trace"
)

// curl -vkL http://127.0.0.1:8081/serviceA
func serviceA(ctx context.Context, port int, tracerName string) {
	serverPort := fmt.Sprintf(":%d", port)
	address := fmt.Sprintf("127.0.0.1%s", serverPort)
	var mux http.ServeMux

	mux.HandleFunc("/serviceA", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer(tracerName).Start(r.Context(), "serviceA_HttpHandler")
		defer span.End()

		counter, _ := getMeter().SyncInt64().Counter(
			"service_a_called_counter",
			instrument.WithDescription("how many time the serviceA handler has been called."),
		)
		counter.Add(ctx, 1)

		log := New(ctx)
		log.Info("serviceA called")

		// When serviceA is called, it calls serviceB
		cli := &http.Client{
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				// if you use the alternative way of setting global propagator as shown in `tracing.go`
				// then you do not need to provide this one
				// otelhttp.WithPropagators(propagator),
			),
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8082/serviceB", nil)
		// req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8082/serviceB", nil)
		if err != nil {
			panic(err)
		}
		resp, err := cli.Do(req)
		if err != nil {
			panic(err)
		}
		log.Info("serviceA called serviceB and got resp.StatusCode: ", resp.StatusCode)

		fmt.Fprintf(w, "hello from serviceA")
		// response header contains, `Ot-Tracer-Spanid` & `Ot-Tracer-Traceid` headers that are added by the otel propagator.
		// upstream services can then consume those.
		log.Info("request.Header serviceA: ", r.Header)
		log.Info("response.Header serviceA: ", w.Header())
	})

	handler := otelhttp.NewHandler(
		&mux,
		"server.http",
		// if you use the alternative way of setting global propagator as shown in `tracing.go`
		// then you do not need to provide this one
		// otelhttp.WithPropagators(propagator),
	)
	server := &http.Server{
		Addr:    serverPort,
		Handler: handler,
		// It is very important to add this `BaseContext``
		// so that it gets the one from OpenTelemetry
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	log := New(ctx)
	log.Info("serviceA listening on: ", address)
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

// curl -vkL http://127.0.0.1:8082/serviceB
func serviceB(ctx context.Context, port int, tracerName string) {
	serverPort := fmt.Sprintf(":%d", port)
	address := fmt.Sprintf("127.0.0.1%s", serverPort)
	var mux http.ServeMux
	mux.HandleFunc("/serviceB", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer(tracerName).Start(r.Context(), "serviceB_HttpHandler")
		defer span.End()

		log := New(ctx)
		log.Info("serviceB called")

		answer := add(ctx, 42, 1813)

		fmt.Fprintf(w, "hello from serviceB: Answer is: %d", answer)
		// response header contains, `Ot-Tracer-Spanid` & `Ot-Tracer-Traceid` headers that are added by the otel propagator.
		// upstream services can then consume those.
		log.Info("request.Header serviceB: ", r.Header)
		log.Info("response.Header serviceB: ", w.Header())
	})

	handler := otelhttp.NewHandler(
		&mux,
		"server.http",
		// if you use the alternative way of setting global propagator as shown in `tracing.go`
		// then you do not need to provide this one
		// otelhttp.WithPropagators(propagator),
	)
	server := &http.Server{
		Addr:    serverPort,
		Handler: handler,
		// It is very important to add this `BaseContext``
		// so that it gets the one from OpenTelemetry
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	log := New(ctx)
	log.Info("serviceB listening on: ", address)
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func add(ctx context.Context, x, y int64) int64 {
	// otel.Tracer("instrumentation/package/name", trace.WithStackTrace(true)) // can also take other opts
	ctx, span := otel.Tracer(tracerName).Start(
		ctx,
		"add",
		// add labels/tags(if any) that are specific to this scope.
		trace.WithAttributes(attribute.String("method", "GET")),
		trace.WithAttributes(attribute.String("endpoint", "/foo/user")),
	)
	defer span.End()

	err := errors.New("oops, 99 problems")
	span.RecordError(err, trace.WithStackTrace(true))

	log := New(ctx)
	log.Println("add called.")

	return x + y
}
