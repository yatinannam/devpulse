package ports

import (
	"fmt"
	"sort"
)

func Print(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Port < entries[j].Port })

	fmt.Println("DEVPULSE")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("%-8s %-20s %-10s %s\n", "PORT", "PROCESS", "PID", "STATUS")

	for _, e := range entries {
		process := e.Process
		if process == "" {
			process = "-"
		}
		fmt.Printf("%-8d %-20s %-10s ● %s\n", e.Port, process, e.PID, e.State)
	}

	if len(entries) == 0 {
		fmt.Println("No listening TCP ports found.")
	}
}
