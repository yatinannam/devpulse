//go:build windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coord struct{ X, Y int16 }

type smallRect struct{ Left, Top, Right, Bottom int16 }

// consoleScreenBufferInfo mirrors CONSOLE_SCREEN_BUFFER_INFO.
type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

// enableVirtualTerminal turns on ANSI escape processing for the stdout
// console. It reports false when stdout is not a console or the console
// refuses, and color then stays off.
func enableVirtualTerminal() bool {
	if procGetConsoleMode.Find() != nil || procSetConsoleMode.Find() != nil {
		return false
	}
	handle := os.Stdout.Fd()
	var mode uint32
	if ok, _, _ := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	ok, _, _ := procSetConsoleMode.Call(handle, uintptr(mode|enableVirtualTerminalProcessing))
	return ok != 0
}

// terminalWidth reads the visible window width of the stdout console.
func terminalWidth() (int, bool) {
	if procGetConsoleScreenBufferInfo.Find() != nil {
		return 0, false
	}
	var info consoleScreenBufferInfo
	if ok, _, _ := procGetConsoleScreenBufferInfo.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0, false
	}
	width := int(info.Window.Right) - int(info.Window.Left) + 1
	return width, width > 0
}
