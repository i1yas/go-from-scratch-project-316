package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"code/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	cmd := app.BuildCommand()

	err := cmd.Run(ctx, os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
