//go:build !windows && !linux

package ports

import "fmt"

func List() ([]Entry, error) {
	return nil, fmt.Errorf("unsupported operating system")
}
