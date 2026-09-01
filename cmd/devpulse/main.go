package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yatinannam/devpulse/internal/config"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/doctor"
	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/status"
	"github.com/yatinannam/devpulse/internal/traffic"
)

var version = "dev"

func main() {
	if len(os.Args) == 1 {
		statusCommand(nil)
		return
	}
	switch os.Args[1] {
	case "ports":
		portsCommand(os.Args[2:])
	case "traffic":
		trafficCommand(os.Args[2:])
	case "doctor":
		doctorCommand(os.Args[2:])
	case "status":
		statusCommand(os.Args[2:])
	case "watch":
		watchCommand(os.Args[2:])
	case "config":
		configCommand(os.Args[2:])
	case "version":
		fmt.Println("devpulse " + version)
	default:
		fmt.Fprintf(os.Stderr, "devpulse: unknown command %q\nusage: devpulse [ports|traffic|doctor|status|watch|config|version]\n", os.Args[1])
		os.Exit(2)
	}
}

func loadConfig() config.Config {
	c, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	return c
}

func configCommand(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	setTarget := fs.String("target", "", "set default upstream")
	setListen := fs.String("listen", "", "set proxy listen address")
	setInterval := fs.String("watch-interval", "", "set watch refresh interval")
	_ = fs.Parse(args)

	c := loadConfig()
	changed := false
	if *setTarget != "" {
		c.Target = *setTarget
		changed = true
	}
	if *setListen != "" {
		c.Listen = *setListen
		changed = true
	}
	if *setInterval != "" {
		c.WatchInterval = *setInterval
		changed = true
	}
	if changed {
		if err := config.Save(c); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("Config: %s\nlisten=%s\ntarget=%s\nwatch_interval=%s\n", config.Path(), c.Listen, c.Target, c.WatchInterval)
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
		if err := ports.Watch(time.Second, ports.PrintChange); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
	}
}

func sessionPath() string {
	if p := os.Getenv("DEVPULSE_SESSION"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".devpulse/session.json"
	}
	return filepath.Join(home, ".devpulse", "session.json")
}

func trafficCommand(args []string) {
	c := loadConfig()
	fs := flag.NewFlagSet("traffic", flag.ExitOnError)
	listen := fs.String("listen", c.Listen, "proxy listen address")
	target := fs.String("target", c.Target, "upstream application")
	_ = fs.Parse(args)

	recorder := traffic.NewRecorder(func(r traffic.Request) {
		fmt.Printf("%s  %-6s %-32s %d  %s  → %s:%d\n",
			r.Time.Format("15:04:05"), r.Method, r.Path, r.Status,
			r.Latency.Round(time.Millisecond), r.TargetHost, r.TargetPort)
	})
	proxy, err := traffic.NewProxy(*target, recorder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("DEVPULSE TRAFFIC")
	fmt.Println("────────────────────────────────────────────────")
	fmt.Printf("Proxy: %s → %s\n", *listen, *target)
	fmt.Println("Waiting for HTTP traffic... (Ctrl+C to save and stop)")
	fmt.Println()

	server, err := traffic.Start(traffic.Handler(proxy), *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	entries := recorder.Snapshot()
	if err := traffic.SaveSession(sessionPath(), entries); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: could not save session: %v\n", err)
	} else {
		fmt.Printf("\nSaved %d requests to %s\n", len(entries), sessionPath())
	}
	_ = traffic.Shutdown(server)
}

func statusCommand(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to summarize")
	_ = fs.Parse(args)

	services, err := discovery.Discover(300 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("DEVPULSE")
	fmt.Println("────────────────────────────────────────────────")

	s, loadErr := traffic.LoadSession(*from)
	if loadErr != nil {
		fmt.Println("LOCAL SERVICES")
		for _, svc := range services {
			marker := "○"
			if svc.HTTP {
				marker = "●"
			}
			kind := svc.Kind
			if kind == "" {
				kind = "TCP service"
			}
			fmt.Printf("%s :%-5d %-18s PID %-8s\n", marker, svc.Port, kind, svc.PID)
		}
		fmt.Printf("\nHTTP TRAFFIC\nNo captured session (%s)\n", *from)
		return
	}

	groups := status.GroupByService(services, s.Requests)
	status.PrintServices(groups)
	fmt.Printf("\nSession: %s\n", *from)
	summary := status.Build(s.Requests)
	fmt.Printf("Requests %d · Errors %d · Average %s · Slow %d\n",
		summary.Total, summary.Errors, summary.Average.Round(time.Millisecond), summary.Slow)
}

func watchCommand(args []string) {
	c := loadConfig()
	defaultInterval := 2 * time.Second
	if d, err := time.ParseDuration(c.WatchInterval); err == nil {
		defaultInterval = d
	}

	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	interval := fs.Duration("interval", defaultInterval, "refresh interval")
	from := fs.String("from", sessionPath(), "traffic session to summarize")
	_ = fs.Parse(args)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	refresh := func() {
		services, err := discovery.Discover(300 * time.Millisecond)
		if err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			return
		}
		fmt.Print("\033[2J\033[H")
		fmt.Println("DEVPULSE WATCH")
		fmt.Println("────────────────────────────────────────────────")

		s, err := traffic.LoadSession(*from)
		if err != nil {
			status.PrintServices(status.GroupByService(services, nil))
			fmt.Printf("\nNo captured session: %s\n", *from)
			return
		}
		status.PrintServices(status.GroupByService(services, s.Requests))
		summary := status.Build(s.Requests)
		fmt.Printf("\nRequests %d · Errors %d · Average %s · Slow %d\n",
			summary.Total, summary.Errors, summary.Average.Round(time.Millisecond), summary.Slow)
		fmt.Printf("Updated %s · refresh %s · Ctrl+C to stop\n",
			time.Now().Format("15:04:05"), *interval)
	}

	refresh()
	for range ticker.C {
		refresh()
	}
}

func doctorCommand(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to analyze")
	_ = fs.Parse(args)

	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Session: %s · %d requests\n\n", *from, len(s.Requests))
	doctor.Print(doctor.Analyze(s.Requests))
}
