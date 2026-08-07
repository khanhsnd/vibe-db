package dbtool

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a console from flashing up over the app. Every one of these programs is a
// console application, and Windows gives one its own window unless it is told not to — which on a
// desktop app means a black rectangle appearing and vanishing for every dump.
func hideWindow(child *exec.Cmd) {
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
