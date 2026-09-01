package traffic

import (
	"fmt"
	"time"
)

func Print(entries []Request) {
	fmt.Println("DEVPULSE TRAFFIC")
	fmt.Println("────────────────────────────────────────────────")
	fmt.Printf("%-9s %-7s %-32s %-8s %s\n", "TIME", "METHOD", "PATH", "STATUS", "LATENCY")
	for _, e := range entries {
		fmt.Printf("%-9s %-7s %-32s %-8d %s\n",
			e.Time.Format("15:04:05"), e.Method, truncate(e.Path, 32), e.Status, e.Latency.Round(time.Millisecond))
	}
	if len(entries) == 0 {
		fmt.Println("No requests recorded.")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
