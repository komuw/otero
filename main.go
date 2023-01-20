package main

import (
	"context"
)

// TODO: rename
const tracerName = "github.com/komuw/otero"

func main() {
	ctx := context.Background()
	{
		tp, err := setupTracing()
		if err != nil {
			panic(err)
		}
		defer func() {
			err := tp.Shutdown(ctx)
			_ = err
		}()

		mp, err := setupMetrics()
		if err != nil {
			panic(err)
		}
		defer func() {
			err := mp.Shutdown(ctx)
			_ = err
		}()
	}

	go func() { serviceA(ctx, 8081, tracerName) }()
	serviceB(ctx, 8082, tracerName)
}
