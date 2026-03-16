//go:build !windows

package invoke

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// RestoreTerminal forces the terminal back to cooked mode with signal
// generation enabled. Claude Code leaves the terminal in raw mode
// (ISIG off) after exiting, which causes Ctrl+C to print ^C instead
// of delivering SIGINT.
func RestoreTerminal() {
	fd := os.Stdin.Fd()

	// Get current termios
	var termios syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlReadTermios,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	); errno != 0 {
		fmt.Fprintf(os.Stderr, "  [terminal restore: not a terminal (errno=%d)]\n", errno)
		return
	}

	hadISIG := termios.Lflag&syscall.ISIG != 0

	// Ensure ISIG (Ctrl+C → SIGINT), ICANON (line buffering),
	// and ECHO are enabled.
	termios.Lflag |= syscall.ISIG | syscall.ICANON | syscall.ECHO

	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlWriteTermios,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	); errno != 0 {
		fmt.Fprintf(os.Stderr, "  [terminal restore: write failed (errno=%d)]\n", errno)
	} else if !hadISIG {
		fmt.Fprintf(os.Stderr, "  [terminal restored: ISIG was off, now on]\n")
	} else {
		fmt.Fprintf(os.Stderr, "  [terminal restore: ISIG was already on]\n")
	}
}
