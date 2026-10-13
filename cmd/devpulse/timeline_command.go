package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/timeline"
	"github.com/yatinannam/devpulse/internal/traffic"
)

type timelinePayload struct {
	Session string            `json:"session"`
	Bucket  time.Duration     `json:"bucket_width"`
	Buckets []timeline.Bucket `json:"buckets"`
}

func timelineCommand(args []string) {
	fs := flag.NewFlagSet("timeline", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to inspect")
	width := fs.Duration("bucket", time.Minute, "bucket width, for example 30s or 5m")
	jsonOutput := fs.Bool("json", false, "print timeline as JSON")
	_ = fs.Parse(args)

	if *width <= 0 {
		fmt.Fprintln(os.Stderr, "devpulse: --bucket must be greater than zero")
		os.Exit(2)
	}
	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	buckets := timeline.Build(s.Requests, *width)

	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, timelinePayload{
			Session: *from,
			Bucket:  *width,
			Buckets: buckets,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("DEVPULSE TIMELINE")
	fmt.Println("------------------------------------------------")
	fmt.Printf("Session: %s  Bucket: %s\n", *from, *width)
	fmt.Printf("%-25s %8s %8s %12s\n", "START", "REQUESTS", "ERRORS", "AVG LATENCY")
	if len(buckets) == 0 {
		fmt.Println("No requests captured.")
		return
	}
	for _, bucket := range buckets {
		fmt.Printf("%-25s %8d %8d %12s\n",
			bucket.Start.Format(time.RFC3339), bucket.Requests, bucket.Errors,
			bucket.Average.Round(time.Millisecond))
	}
}
