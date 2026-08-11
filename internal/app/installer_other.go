//go:build !windows

package app

import "errors"

func startInstaller(string) error {
	return errors.New("automatic installation is available on Windows only")
}
