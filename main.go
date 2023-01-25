package main

import (
	"context"
	"flag"
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
	{
		tp, err := setupTracing(ctx)
		if err != nil {
			panic(err)
		}
		defer func() {
			err := tp.Shutdown(ctx)
			_ = err
		}()

		mp, err := setupMetrics(ctx)
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
