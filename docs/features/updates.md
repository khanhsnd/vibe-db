# Application updates and releases

Vibe DB has one release source and two installation paths:

| Platform | Distribution | Update action |
| --- | --- | --- |
| Windows 10/11 x64 | NSIS installer from `khanhsnd/vibe-db-releases` | Settings → Updates downloads and starts the next installer |
| macOS 12+ Intel/Apple Silicon | ZIP from `khanhsnd/vibe-db-releases`, or Cask from `khanhsnd/homebrew-tap` | Homebrew runs `brew upgrade --cask vibe-db`; a direct download is replaced manually |

The source repository remains private. Release binaries are public because Homebrew cannot download a
private GitHub asset without a credential. No database profiles, secrets or history are stored inside
the installed application directory, so installing a new version does not migrate or remove them.

## One-time GitHub setup

Create these public repositories:

1. `khanhsnd/vibe-db-releases` — initialize it with a README on the `master` branch; GitHub Actions will
   create the releases and upload the installer, macOS ZIP and `checksums.txt`.
2. `khanhsnd/homebrew-tap` — initialize it with a README. The release workflow creates `Casks/vibe-db.rb`
   here.

Create a fine-grained GitHub token with `Contents: Read and write` on both public repositories. Add it to
the private source repository as the Actions secret `RELEASES_TOKEN`. Never put this token in Go, a YAML
file or a release asset.

## Publishing a version

Use a three-part semantic version and push a tag from the private source repository:

```powershell
git tag v1.0.0
git push origin v1.0.0
```

The workflow builds a Windows x64 NSIS installer and a universal macOS app, computes SHA-256 checksums,
publishes the assets to the public release repository, and updates the Homebrew Cask. A release is not
complete until both publish jobs are green.

Users can install the macOS Cask with:

```sh
brew tap khanhsnd/tap
brew install --cask vibe-db
```

They update it with:

```sh
brew upgrade --cask vibe-db
```

The app's Settings → Updates page checks the GitHub release API. A Windows update is downloaded and its
checksum is checked against `checksums.txt` before the NSIS installer starts and the app exits. macOS
only displays the Homebrew command or the release page because this project has no Apple Developer
certificate or notarization; Gatekeeper may require the first launch to be confirmed manually.

## Local verification

The Wails v2 CLI supports the NSIS installer and universal macOS target:

```powershell
wails build -platform windows/amd64 -nsis
```

```sh
wails build -platform darwin/universal
ditto -c -k --sequesterRsrc --keepParent build/bin/vibe-db.app vibe-db.zip
```

Run `go test ./...` before tagging. The updater logic is covered by focused tests in
`internal/update/service_test.go`.
