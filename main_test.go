package main

import (
	"context"
	"testing"
	"time"
)

func TestSmokeTest(t *testing.T) {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Millisecond)
		mainCancel()
	}()
	main()
}
