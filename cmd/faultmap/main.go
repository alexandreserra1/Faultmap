package main

import (
	"fmt"
	"os"
)

// main executa a CLI e devolve erros ao terminal com status diferente de zero.
func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
