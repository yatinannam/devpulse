//go:build linux || darwin

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

// enableVirtualTerminal has nothing to do: Unix terminals interpret ANSI
// escapes natively.
func enableVirtualTerminal() bool { return true }

// winsize mirrors struct winsize from <sys/ioctl.h>.
type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

// terminalWidth asks the stdout terminal for its size with TIOCGWINSZ.
func terminalWidth() (int, bool) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 {
		return 0, false
	}
	return int(ws.Col), true
}
