package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/doctor"
	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/status"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func main() {
	if len(os.Args)==1 { statusCommand(nil); return }
	switch os.Args[1] {
	case "ports": portsCommand(os.Args[2:])
	case "traffic": trafficCommand(os.Args[2:])
	case "doctor": doctorCommand(os.Args[2:])
	case "status": statusCommand(os.Args[2:])
	case "watch": fmt.Println("devpulse: watch is not implemented yet")
	default: fmt.Fprintf(os.Stderr,"devpulse: unknown command %q
usage: devpulse [ports|traffic|doctor|status|watch]
",os.Args[1]); os.Exit(2)
	}
}
func portsCommand(args []string) {
	fs:=flag.NewFlagSet("ports",flag.ExitOnError); watch:=fs.Bool("watch",false,"watch for port changes"); _=fs.Parse(args)
	entries,err:=ports.List(); if err!=nil {fmt.Fprintf(os.Stderr,"devpulse: %v
",err);os.Exit(1)}; ports.Print(entries)
	if *watch {fmt.Println("
Watching for port changes... (Ctrl+C to stop)");if err:=ports.Watch(time.Second,ports.PrintChange);err!=nil{fmt.Fprintf(os.Stderr,"devpulse: %v
",err);os.Exit(1)}}
}
func sessionPath() string {
	if p:=os.Getenv("DEVPULSE_SESSION");p!="" {return p}
	home,err:=os.UserHomeDir();if err!=nil{return ".devpulse/session.json"}
	return filepath.Join(home,".devpulse","session.json")
}
func trafficCommand(args []string) {
	fs:=flag.NewFlagSet("traffic",flag.ExitOnError); listen:=fs.String("listen",":9090","proxy listen address"); target:=fs.String("target","http://localhost:3000","upstream application"); _=fs.Parse(args)
	recorder:=traffic.NewRecorder(func(r traffic.Request){fmt.Printf("%s  %-6s %-32s %d  %s  → %s:%d
",r.Time.Format("15:04:05"),r.Method,r.Path,r.Status,r.Latency.Round(time.Millisecond),r.TargetHost,r.TargetPort)})
	proxy,err:=traffic.NewProxy(*target,recorder);if err!=nil{fmt.Fprintf(os.Stderr,"devpulse: %v
",err);os.Exit(1)}
	fmt.Println("DEVPULSE TRAFFIC");fmt.Println("────────────────────────────────────────────────");fmt.Printf("Proxy: %s → %s
",*listen,*target);fmt.Println("Waiting for HTTP traffic... (Ctrl+C to save and stop)");fmt.Println()
	server:=traffic.Start(traffic.Handler(proxy),*listen)
	sig:=make(chan os.Signal,1);signal.Notify(sig,os.Interrupt,syscall.SIGTERM);<-sig
	entries:=recorder.Snapshot()
	if err:=traffic.SaveSession(sessionPath(),entries);err!=nil{fmt.Fprintf(os.Stderr,"devpulse: could not save session: %v
",err)}else{fmt.Printf("
Saved %d requests to %s
",len(entries),sessionPath())}
	_ = traffic.Shutdown(server)
}
func statusCommand(args []string) {
 fs:=flag.NewFlagSet("status",flag.ExitOnError)
 from:=fs.String("from",sessionPath(),"traffic session to summarize")
 _=fs.Parse(args)
 services,err:=discovery.Discover(300*time.Millisecond)
 if err!=nil {fmt.Fprintf(os.Stderr,"devpulse: %v\n",err);os.Exit(1)}
 fmt.Println("DEVPULSE")
 fmt.Println("────────────────────────────────────────────────")
 fmt.Println("LOCAL SERVICES")
 for _,s:=range services {
  marker:="○"; if s.HTTP {marker="●"}
  if s.HTTP {fmt.Printf("%s :%-5d %-10s PID %-8s %s\n",marker,s.Port,s.Process,s.PID,s.URL)} else {fmt.Printf("%s :%-5d %-10s PID %-8s (TCP)\n",marker,s.Port,s.Process,s.PID)}
 }
 s,loadErr:=traffic.LoadSession(*from)
 if loadErr!=nil {fmt.Printf("\nHTTP TRAFFIC\nNo captured session (%s)\n",*from);return}
 fmt.Println()
 status.Print(status.Build(s.Requests),status.Endpoints(s.Requests),*from)
}

func doctorCommand(args []string) {
	fs:=flag.NewFlagSet("doctor",flag.ExitOnError); from:=fs.String("from",sessionPath(),"traffic session to analyze");_=fs.Parse(args)
	s,err:=traffic.LoadSession(*from);if err!=nil{fmt.Fprintf(os.Stderr,"devpulse: %v
",err);os.Exit(1)}
	fmt.Printf("Session: %s · %d requests

",*from,len(s.Requests));doctor.Print(doctor.Analyze(s.Requests))
}
