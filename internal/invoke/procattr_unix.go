//go:build !windows

package invoke

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// processSysProcAttr returns SysProcAttr that puts the child process in its
// own process group for clean signal propagation on Unix systems.
func processSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// reclaimForeground restores the calling process's group as the terminal's
// foreground process group. Child processes (like Claude Code) may take
// over the foreground group during execution; if they don't restore it
// on exit, SIGINT from Ctrl+C goes to a stale group and the parent
// never receives it.
func reclaimForeground() {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return // not a terminal, nothing to reclaim
	}
	defer func() { _ = tty.Close() }()

	pgid := syscall.Getpgrp()
	_ = unix.IoctlSetPointerInt(int(tty.Fd()), unix.TIOCSPGRP, pgid)
}
