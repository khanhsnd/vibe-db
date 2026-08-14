// Package update reads the public release feed and downloads a verified Windows installer.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const ReleasesURL = "https://api.github.com/repos/khanhsnd/vibe-db-releases/releases/latest"

var ErrDevelopmentBuild = errors.New("development builds do not update")

type Platform string

const (
	Windows Platform = "windows"
	MacOS   Platform = "darwin"
	Linux   Platform = "linux"
)

type Release struct {
	Version    string
	ReleaseURL string
	Installer  Asset
	Checksums  Asset
}

type Asset struct {
	Name string
	URL  string
}

type Service struct {
	client      *http.Client
	releasesURL string
}

func New() *Service {
	return &Service{
		client:      &http.Client{Timeout: 30 * time.Second},
		releasesURL: ReleasesURL,
	}
}

func (s *Service) Latest(ctx context.Context, platform Platform) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.releasesURL, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := s.client.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("read latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("read latest release: %s", response.Status)
	}

	var githubRelease githubRelease
	if err := json.NewDecoder(response.Body).Decode(&githubRelease); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %w", err)
	}

	version := strings.TrimPrefix(githubRelease.TagName, "v")
	if !validVersion(version) {
		return Release{}, fmt.Errorf("latest release has invalid version %q", githubRelease.TagName)
	}

	installerName := installerName(version, platform)
	installer, ok := githubRelease.asset(installerName)
	if !ok {
		return Release{}, fmt.Errorf("latest release does not include %s", installerName)
	}
	checksums, ok := githubRelease.asset("checksums.txt")
	if !ok {
		return Release{}, errors.New("latest release does not include checksums.txt")
	}

	return Release{
		Version:    version,
		ReleaseURL: githubRelease.HTMLURL,
		Installer:  installer,
		Checksums:  checksums,
	}, nil
}

func (s *Service) DownloadWindowsInstaller(ctx context.Context, release Release) (string, error) {
	if release.Installer.Name != installerName(release.Version, Windows) {
		return "", errors.New("release does not contain the Windows installer")
	}

	checksum, err := s.checksum(ctx, release.Checksums, release.Installer.Name)
	if err != nil {
		return "", err
	}

	response, err := s.get(ctx, release.Installer.URL)
	if err != nil {
		return "", fmt.Errorf("download installer: %w", err)
	}
	defer response.Body.Close()

	file, err := os.CreateTemp("", "vibe-db-*.exe")
	if err != nil {
		return "", fmt.Errorf("create installer file: %w", err)
	}
	path := file.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(path)
		}
	}()

	hash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hash), response.Body); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("save installer: %w", err)
	}
	if err = file.Close(); err != nil {
		return "", fmt.Errorf("close installer: %w", err)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(actual, checksum) {
		return "", errors.New("installer checksum does not match the published release")
	}
	keep = true
	return path, nil
}

func (s *Service) checksum(ctx context.Context, asset Asset, name string) (string, error) {
	response, err := s.get(ctx, asset.URL)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	defer response.Body.Close()

	contents, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == sha256.Size*2 {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksums.txt does not include %s", name)
}

func (s *Service) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("%s", response.Status)
	}
	return response, nil
}

func installerName(version string, platform Platform) string {
	switch platform {
	case Windows:
		return "vibe-db-" + version + "-windows-x64-setup.exe"
	case Linux:
		return "vibe-db-" + version + "-linux-x64.tar.gz"
	default:
		return "vibe-db-" + version + "-macos-universal.zip"
	}
}

// Differs answers whether the published release is a different version from the one running, which
// is the whole of what makes it an update. It deliberately does not ask whether the published one is
// *higher*: the release channel decides which version this app is meant to be on, so a version line
// corrected downwards reaches its readers like any other release rather than stranding every copy
// that was built from the mistake.
func Differs(current, latest string) (bool, error) {
	if current == "dev" {
		return false, ErrDevelopmentBuild
	}
	currentParts, ok := parseVersion(current)
	if !ok {
		return false, fmt.Errorf("invalid current version %q", current)
	}
	latestParts, ok := parseVersion(latest)
	if !ok {
		return false, fmt.Errorf("invalid latest version %q", latest)
	}
	return currentParts != latestParts, nil
}

func validVersion(version string) bool {
	_, ok := parseVersion(version)
	return ok
}

func parseVersion(version string) ([3]int, bool) {
	var parsed [3]int
	parts := strings.Split(version, ".")
	if len(parts) != len(parsed) {
		return parsed, false
	}
	for index, part := range parts {
		if part == "" {
			return parsed, false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return parsed, false
			}
			parsed[index] = parsed[index]*10 + int(character-'0')
		}
	}
	return parsed, true
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (release githubRelease) asset(name string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return Asset{Name: asset.Name, URL: asset.URL}, true
		}
	}
	return Asset{}, false
}
