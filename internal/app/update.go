package app

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/khanhsnd/vibe-db/internal/update"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const updateTimeout = 30 * time.Second

// UpdateInfo is the one public release's relationship to this copy of the app. The browser does
// not decide whether a tag is the one to be on: it only draws the answer, so a future frontend has
// the same update rule as this one.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	Platform       string `json:"platform"`
	ReleaseURL     string `json:"releaseUrl"`
	Command        string `json:"command"`
	Development    bool   `json:"development"`
}

// UpdateAPI is deliberately small: releases live on GitHub, Windows installs the downloaded NSIS
// package, macOS remains managed by Homebrew or a manually replaced app bundle, and Linux is an
// archive the reader unpacks over their own copy.
type UpdateAPI struct {
	apiContext
	service *update.Service
	version string
}

func NewUpdateAPI(version string) *UpdateAPI {
	if version == "" {
		version = "dev"
	}
	return &UpdateAPI{apiContext: newAPIContext(), service: update.New(), version: version}
}

// Check answers the published release for this operating system, which is an update whenever its
// version is not the one running. Development builds do not query the network: a local tree must
// never offer to replace itself with a published build.
func (a *UpdateAPI) Check() (UpdateInfo, error) {
	info := UpdateInfo{CurrentVersion: a.version, Platform: runtime.GOOS}
	if a.version == "dev" {
		info.Development = true
		return info, nil
	}

	ctx, cancel := a.withTimeout(updateTimeout)
	defer cancel()
	release, err := a.service.Latest(ctx, updatePlatform())
	if err != nil {
		return UpdateInfo{}, updateFailure("check for updates", err)
	}
	available, err := update.Differs(a.version, release.Version)
	if err != nil {
		return UpdateInfo{}, updateFailure("compare release versions", err)
	}
	info.LatestVersion = release.Version
	info.Available = available
	info.ReleaseURL = release.ReleaseURL
	if runtime.GOOS == "darwin" && available {
		info.Command = "brew upgrade --cask vibe-db"
	}
	return info, nil
}

// InstallWindowsUpdate downloads the public NSIS installer, verifies its published SHA-256, starts
// it through the shell's elevation prompt and then closes this process. NSIS owns the replacement
// of the running executable, and the app stays open if the prompt is declined.
func (a *UpdateAPI) InstallWindowsUpdate() error {
	if runtime.GOOS != "windows" {
		return updateFailure("install update", errors.New("automatic installation is available on Windows only"))
	}
	if a.version == "dev" {
		return updateFailure("install update", update.ErrDevelopmentBuild)
	}

	ctx, cancel := a.withTimeout(updateTimeout)
	defer cancel()
	release, err := a.service.Latest(ctx, update.Windows)
	if err != nil {
		return updateFailure("download update", err)
	}
	available, err := update.Differs(a.version, release.Version)
	if err != nil {
		return updateFailure("compare release versions", err)
	}
	if !available {
		return updateFailure("install update", errors.New("this is already the published version"))
	}

	installer, err := a.service.DownloadWindowsInstaller(ctx, release)
	if err != nil {
		return updateFailure("download update", err)
	}
	if err := startInstaller(installer); err != nil {
		return updateFailure("start installer", err)
	}
	wailsruntime.Quit(a.ctx)
	return nil
}

// OpenRelease opens the public release page for a direct macOS or Linux download. It intentionally
// does not install anything: Homebrew owns the app when that is how it was installed, and a Linux
// archive was unpacked wherever its reader chose.
func (a *UpdateAPI) OpenRelease() error {
	info, err := a.Check()
	if err != nil {
		return err
	}
	if info.ReleaseURL == "" {
		return updateFailure("open release page", errors.New("no release page is available"))
	}
	wailsruntime.BrowserOpenURL(a.ctx, info.ReleaseURL)
	return nil
}

func updatePlatform() update.Platform {
	switch runtime.GOOS {
	case "windows":
		return update.Windows
	case "linux":
		return update.Linux
	default:
		return update.MacOS
	}
}

func updateFailure(action string, err error) error {
	return wireError{
		Code:    CodeUpdateFailed,
		Message: "Could not " + action + ". Try again shortly.",
		Detail:  fmt.Sprintf("%v", err),
	}
}
