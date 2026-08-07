//go:build !windows

package dbtool

import "os/exec"

// hideWindow is Windows' alone: nothing else gives a console application a window of its own.
func hideWindow(*exec.Cmd) {}
