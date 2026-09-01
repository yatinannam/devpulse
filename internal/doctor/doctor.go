package doctor

import (
	"fmt"
	"time"
	"github.com/yatinannam/devpulse/internal/traffic"
)

type Finding struct { Level, Title, Detail string }

func Analyze(entries []traffic.Request) []Finding {
	var findings []Finding
	if len(entries)==0 { return findings }
	var errors, slow int
	for _, e := range entries {
		if e.Status >= 400 { errors++ }
		if e.Latency >= 500*time.Millisecond { slow++ }
	}
	if errors > 0 { findings=append(findings, Finding{"WARN","HTTP errors",fmt.Sprintf("%d of %d requests returned 4xx/5xx",errors,len(entries))}) }
	if slow > 0 { findings=append(findings, Finding{"WARN","Slow requests",fmt.Sprintf("%d request(s) took 500ms or more",slow)}) }
	return findings
}

func Print(findings []Finding) {
	fmt.Println("DEVPULSE DOCTOR")
	fmt.Println("────────────────────────────────────────────────")
	if len(findings)==0 { fmt.Println("✓ No obvious HTTP issues detected."); return }
	for _, f := range findings { fmt.Printf("%s  %s — %s\n",f.Level,f.Title,f.Detail) }
}
