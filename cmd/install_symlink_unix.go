//go:build !windows

package cmd

func canSymlink() bool { return true }
