//go:build !windows

package invoke

import (
	"os"
	"syscall"
	"unsafe"
)

// RestoreTerminal forces the terminal back to cooked mode with signal
// generation enabled. Child processes may leave the terminal in raw mode
// (ISIG off) after exiting, which causes Ctrl+C to print ^C instead
// of delivering SIGINT.
func RestoreTerminal() {
	fd := os.Stdin.Fd()

	var termios syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlReadTermios,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	); errno != 0 {
		return
	}

	termios.Lflag |= syscall.ISIG | syscall.ICANON | syscall.ECHO

	_, _, _ = syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		ioctlWriteTermios,
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
}
