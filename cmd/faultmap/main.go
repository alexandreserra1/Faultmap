package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// main executa a CLI e devolve erros ao terminal com status diferente de zero.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCommand().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
