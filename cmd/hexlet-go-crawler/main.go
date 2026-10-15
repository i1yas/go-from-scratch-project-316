package main

import (
	"context"
	"fmt"
	"os"

	"code/internal/app"
)

func main() {
	ctx := context.Background()

	cmd := app.BuildCommand()

	err := cmd.Run(ctx, os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
