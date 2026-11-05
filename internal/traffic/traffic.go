package traffic

import (
	"fmt"
	"time"
	"unicode/utf8"
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

// truncate shortens s to at most n runes, ending in an ellipsis. It counts
// runes rather than bytes so a multi-byte character is never split.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
