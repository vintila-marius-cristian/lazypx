package main

import (
	"fmt"
	"os"

	"lazypx/commands"
)

func main() {
	root := commands.Root()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
