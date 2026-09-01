package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func main() {
	if len(os.Args) == 1 {
		portsCommand(nil)
		return
	}

	switch os.Args[1] {
	case "ports":
		portsCommand(os.Args[2:])
	case "traffic":
		trafficCommand(os.Args[2:])
	case "doctor", "watch":
		fmt.Printf("devpulse: %s is not implemented yet\n", os.Args[1])
	default:
		fmt.Fprintf(os.Stderr, "devpulse: unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: devpulse [ports|traffic|doctor|watch]")
		os.Exit(2)
	}
}

func portsCommand(args []string) {
	fs := flag.NewFlagSet("ports", flag.ExitOnError)
	watch := fs.Bool("watch", false, "watch for port changes")
	_ = fs.Parse(args)

	entries, err := ports.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	ports.Print(entries)

	if *watch {
		fmt.Println("\nWatching for port changes... (Ctrl+C to stop)")
		if err := ports.Watch(1e9, ports.PrintChange); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
	}
}

func trafficCommand(args []string) {
	fs := flag.NewFlagSet("traffic", flag.ExitOnError)
	listen := fs.String("listen", ":9090", "proxy listen address")
	target := fs.String("target", "http://localhost:3000", "upstream application")
	_ = fs.Parse(args)

	recorder := &traffic.Recorder{}
	proxy, err := traffic.NewProxy(*target, recorder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("DevPulse traffic proxy listening on %s → %s\n", *listen, *target)
	fmt.Println("Configure your client to send HTTP traffic through this proxy.")
	fmt.Println("Press Ctrl+C to stop.")

	if err := traffic.Serve(traffic.Handler(proxy), *listen); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
}
