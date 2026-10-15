package main

import (
	"context"
	"fmt"
	"os"

	"code/crawler"
)

func main() {
	ctx := context.Background()

	output, err := crawler.Analyze(ctx, crawler.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Println(string(output))
}
