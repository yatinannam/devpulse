package main

import (
	"fmt"
	"os"

	"github.com/yatinannam/devpulse/internal/ports"
)

func main() {
	command := "ports"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "ports":
		entries, err := ports.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		ports.Print(entries)
	case "traffic", "doctor", "watch":
		fmt.Printf("devpulse: %s is not implemented yet\n", command)
	default:
		fmt.Fprintf(os.Stderr, "devpulse: unknown command %q\n", command)
		fmt.Fprintln(os.Stderr, "usage: devpulse [ports|traffic|doctor|watch]")
		os.Exit(2)
	}
}
