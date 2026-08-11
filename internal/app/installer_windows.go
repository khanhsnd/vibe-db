package app

import "golang.org/x/sys/windows"

// The NSIS installer's manifest requests administrator, and CreateProcess — which exec.Command
// starts a program with — refuses such a program outright rather than asking for elevation. Only
// the shell raises the prompt, so an installer is started the way Explorer would start it.
func startInstaller(path string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
