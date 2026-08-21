package main

import (
	"fmt"
	"io"
)

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: localctl <command>")
		return 1
	}

	switch args[1] {
	case "version":
		fmt.Fprintln(stdout, "localctl dev")
		return 0

	case "runtime":
		return runRuntime(args, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		return 1
	}
}

func runRuntime(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: localctl runtime <status|infer>")
		return 1
	}

	switch args[2] {
	case "status":
		return runtimeStatus(runtimeURL, stdout, stderr)

	case "infer":
		if len(args) < 4 {
			fmt.Fprintln(stderr, "usage: localctl runtime infer <prompt>")
			return 1
		}

		return runtimeInfer(runtimeURL, args[3], stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown runtime command: %s\n", args[2])
		return 1
	}
}
