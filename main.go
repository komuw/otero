package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
)

const tracerName = "github.com/komuw/otero"

func main() {
	var service string
	flag.StringVar(
		&service,
		"service",
		"",
		"service to run")
	flag.Parse()

	service = strings.ToLower(service)
	if service == "" {
		panic("specify a service")
	}

	ctx := context.Background()
	serviceName := fmt.Sprintf("otero-svc-%s", strings.ToUpper(service))
	{
		tp, err := setupTracing(ctx, serviceName)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := tp.Shutdown(ctx)
			_ = err
		}()

		mp, err := setupMetrics(ctx, serviceName)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := mp.Shutdown(ctx)
			_ = err
		}()
	}

	if service == "a" {
		serviceA(ctx, 8081)
	} else {
		serviceB(ctx, 8082)
	}
}
