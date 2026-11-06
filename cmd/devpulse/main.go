package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yatinannam/devpulse/internal/baseline"
	"github.com/yatinannam/devpulse/internal/config"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/diff"
	"github.com/yatinannam/devpulse/internal/doctor"
	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/project"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

var version = "dev"

func main() {
	if len(os.Args) == 1 {
		printHelp()
		return
	}
	switch os.Args[1] {
	case "init":
		initCommand(os.Args[2:])
	case "baseline":
		baselineCommand(os.Args[2:])
	case "diff":
		diffCommand(os.Args[2:])
	case "summary":
		summaryCommand(os.Args[2:])
	case "timeline":
		timelineCommand(os.Args[2:])
	case "anomalies":
		anomaliesCommand(os.Args[2:])
	case "validate":
		validateCommand(os.Args[2:])
	case "report":
		reportCommand(os.Args[2:])
	case "export":
		exportCommand(os.Args[2:])
	case "ports":
		portsCommand(os.Args[2:])
	case "traffic":
		trafficCommand(os.Args[2:])
	case "doctor":
		doctorCommand(os.Args[2:])
	case "recent":
		recentCommand(os.Args[2:])
	case "status":
		statusCommand(os.Args[2:])
	case "watch":
		watchCommand(os.Args[2:])
	case "config":
		configCommand(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("devpulse " + version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "devpulse: unknown command %q\n\n", os.Args[1])
		printHelp()
		os.Exit(2)
	}
}

func initCommand(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	mustParseArgs(fs, args)

	projectInfo, err := project.Detect(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	services, err := discovery.Discover(250 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	target, ok := project.SelectTarget(projectInfo, services)
	if !ok {
		renderInit(os.Stdout, ui.TermWidth(), projectInfo, "", "")
		return
	}

	c := loadConfig()
	c.Target = target.URL
	if err := config.SaveProject(c, projectInfo.Root); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	renderInit(os.Stdout, ui.TermWidth(), projectInfo, target.URL, config.ProjectPath(projectInfo.Root))
}

func baselineCommand(args []string) {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to snapshot")
	to := fs.String("to", baseline.Path("."), "baseline output path")
	mustParseArgs(fs, args)

	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	services, err := discovery.Discover(300 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	if err := baseline.Save(*to, baseline.Build(services, s.Requests)); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Baseline saved: %s\n", *to)
}

func diffCommand(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	from := fs.String("from", baseline.Path("."), "baseline snapshot to compare against")
	session := fs.String("session", sessionPath(), "current traffic session")
	jsonOutput := fs.Bool("json", false, "print changes as JSON")
	mustParseArgs(fs, args)

	base, err := baseline.Load(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	trafficSession, err := traffic.LoadSession(*session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	services, err := discovery.Discover(300 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	current := baseline.Build(services, trafficSession.Requests)
	changes := diff.Compare(base, current)
	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, changes); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}
	renderDiff(os.Stdout, ui.TermWidth(), *session, *from, changes)
}

func loadConfig() config.Config {
	c, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	return c
}

func recentRequests(entries []traffic.Request, n int) []traffic.Request {
	if n < 1 {
		return nil
	}
	if n > len(entries) {
		n = len(entries)
	}
	start := len(entries) - n
	out := make([]traffic.Request, n)
	copy(out, entries[start:])
	return out
}

func recentCommand(args []string) {
	fs := flag.NewFlagSet("recent", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to inspect")
	n := fs.Int("n", 10, "number of recent requests to show")
	jsonOutput := fs.Bool("json", false, "print recent requests as JSON")
	mustParseArgs(fs, args)

	if *n < 1 {
		fmt.Fprintln(os.Stderr, "devpulse: -n must be greater than 0")
		os.Exit(2)
	}

	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}

	recent := recentRequests(s.Requests, *n)
	if *jsonOutput {
		ordered := make([]traffic.Request, len(recent))
		for i := range recent {
			ordered[i] = recent[len(recent)-1-i]
		}
		payload := struct {
			Total    int               `json:"total"`
			Shown    int               `json:"shown"`
			Requests []traffic.Request `json:"requests"`
		}{Total: len(s.Requests), Shown: len(ordered), Requests: ordered}
		if err := output.WriteJSON(os.Stdout, payload); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}
	renderRecent(os.Stdout, ui.TermWidth(), *from, len(s.Requests), recent)
}

func configCommand(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	setTarget := fs.String("target", "", "set default upstream")
	setListen := fs.String("listen", "", "set proxy listen address")
	setInterval := fs.String("watch-interval", "", "set watch refresh interval")
	mustParseArgs(fs, args)

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
		if _, err := parseWatchInterval(*setInterval); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(2)
		}
		c.WatchInterval = *setInterval
		changed = true
	}
	if changed {
		if err := config.Save(c); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
	}
	source := config.Path()
	_, statErr := os.Stat(source)
	renderConfig(os.Stdout, ui.TermWidth(), c, source, statErr == nil, changed)
}

func portsCommand(args []string) {
	fs := flag.NewFlagSet("ports", flag.ExitOnError)
	watch := fs.Bool("watch", false, "watch for port changes")
	mustParseArgs(fs, args)

	entries, err := ports.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	ports.Print(entries)
	if *watch {
		fmt.Println()
		ui.Writeln(os.Stdout, ui.Styled("Watching for port changes. Press Ctrl+C to stop.", ui.Muted))
		onError := func(err error) { fmt.Fprintf(os.Stderr, "devpulse: %v (retrying)\n", err) }
		if err := ports.Watch(time.Second, ports.PrintChange, onError); err != nil {
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
	maxRequests := fs.Int("max-requests", traffic.DefaultMaxRequests, "keep at most this many requests; older ones are dropped")
	mustParseArgs(fs, args)
	if *maxRequests < 1 {
		fmt.Fprintf(os.Stderr, "devpulse traffic: --max-requests must be at least 1, got %d\n", *maxRequests)
		os.Exit(2)
	}

	recorder := traffic.NewLimitedRecorder(*maxRequests, func(r traffic.Request) {
		fmt.Printf("%s  %-6s %-32s %d  %s  → %s:%d\n",
			r.Time.Format("15:04:05"), r.Method, r.Path, r.Status,
			r.Latency.Round(time.Millisecond), r.TargetHost, r.TargetPort)
	}, func() {
		fmt.Fprintf(os.Stderr, "devpulse: captured more than %d requests; dropping the oldest (raise --max-requests to keep more)\n", *maxRequests)
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

	server, serveErr, err := traffic.Start(traffic.Handler(proxy), *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	exitCode := 0
	select {
	case <-sig:
	case err := <-serveErr:
		fmt.Fprintf(os.Stderr, "devpulse: proxy stopped: %v\n", err)
		exitCode = 1
	}
	// Restore the default signal handling, so a second Ctrl+C quits at once
	// if shutdown is held up by a long-lived connection.
	signal.Stop(sig)

	fmt.Printf("\nStopping proxy: waiting up to %s for in-flight requests (Ctrl+C again to quit without saving)\n", traffic.DefaultShutdownTimeout)
	if err := traffic.Shutdown(server, traffic.DefaultShutdownTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
	}
	// Snapshot only after shutdown, so requests that were in flight are saved.
	entries := recorder.Snapshot()
	if err := traffic.SaveSession(sessionPath(), entries); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: could not save session: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved %d requests to %s", len(entries), sessionPath())
	if dropped := recorder.Dropped(); dropped > 0 {
		fmt.Printf(" (%d older requests were dropped; raise --max-requests to keep more)", dropped)
	}
	fmt.Println()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func statusCommand(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to summarize")
	mustParseArgs(fs, args)

	services, err := discovery.Discover(300 * time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}

	width := ui.TermWidth()
	s, loadErr := traffic.LoadSession(*from)
	var context ui.Line
	if loadErr == nil {
		context = sessionContext(*from, len(s.Requests))
	}
	ui.Title(os.Stdout, width, "status", context)
	renderStatusBody(os.Stdout, width, services, s, loadErr, *from)
}

func watchCommand(args []string) {
	// A broken config must not stop watch, or even `watch --help`: warn
	// and fall back to the defaults.
	c, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v; using default settings\n", err)
		c = config.Default()
	}
	defaultInterval := 2 * time.Second
	if d, err := parseWatchInterval(c.WatchInterval); err == nil {
		defaultInterval = d
	} else if c.WatchInterval != "" {
		fmt.Fprintf(os.Stderr, "devpulse: config watch_interval: %v; using %s\n", err, defaultInterval)
	}

	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	interval := fs.Duration("interval", defaultInterval, "refresh interval")
	from := fs.String("from", sessionPath(), "traffic session to summarize")
	mustParseArgs(fs, args)
	if *interval <= 0 {
		fmt.Fprintf(os.Stderr, "devpulse watch: --interval must be positive, got %s\n", *interval)
		os.Exit(2)
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	pulse := ratePulse{limit: 30}
	frames := 0
	refresh := func() {
		services, err := discovery.Discover(300 * time.Millisecond)
		if err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			return
		}
		s, loadErr := traffic.LoadSession(*from)
		if loadErr == nil {
			pulse.observe(len(s.Requests))
		} else {
			pulse.observe(0)
		}
		// Render the whole frame first so the screen is cleared and
		// redrawn in one write.
		var frame bytes.Buffer
		renderWatchFrame(&frame, ui.TermWidth(), time.Now(), *interval, &pulse, services, s, loadErr, *from)
		if !ui.ClearScreen() && frames > 0 {
			fmt.Println()
		}
		frames++
		_, _ = os.Stdout.Write(frame.Bytes())
	}

	refresh()
	for range ticker.C {
		refresh()
	}
}

func doctorCommand(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to analyze")
	profileName := fs.String("profile", string(doctor.ProfileDefault), "diagnostic thresholds: default, strict, permissive")
	jsonOutput := fs.Bool("json", false, "print diagnostic findings as JSON")
	mustParseArgs(fs, args)

	profile := doctor.Profile(strings.ToLower(strings.TrimSpace(*profileName)))
	if _, err := doctor.ThresholdsFor(profile); err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(2)
	}
	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	findings := doctor.AnalyzeWithProfile(s.Requests, profile)
	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, struct { Session string `json:"session"`; Profile doctor.Profile `json:"profile"`; Requests int `json:"requests"`; Findings []doctor.Finding `json:"findings"` }{*from, profile, len(s.Requests), findings}); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}
	width := ui.TermWidth()
	ui.Title(os.Stdout, width, "doctor", sessionContext(*from, len(s.Requests), string(profile)+" profile"))
	if len(s.Requests) == 0 {
		writeNoTraffic(os.Stdout, width)
		return
	}
	doctor.Print(os.Stdout, findings, len(s.Requests), width)
}

func printHelp() {
	renderHelp(os.Stdout, ui.TermWidth())
}
