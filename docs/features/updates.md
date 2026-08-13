# Application updates and releases

Vibe DB has one release source and three installation paths:

| Platform | Distribution | Update action |
| --- | --- | --- |
| Windows 10/11 x64 | NSIS installer from `khanhsnd/vibe-db-releases` | Settings → Updates downloads and starts the next installer |
| macOS 12+ Intel/Apple Silicon | ZIP from `khanhsnd/vibe-db-releases`, or Cask from `khanhsnd/homebrew-tap` | Homebrew runs `brew upgrade --cask vibe-db`; a direct download is replaced manually |
| Linux x64 | `.tar.gz` from `khanhsnd/vibe-db-releases` | Settings → Updates opens the release page; the archive is unpacked over the previous copy |

The source repository remains private. Release binaries are public because Homebrew cannot download a
private GitHub asset without a credential. No database profiles, secrets or history are stored inside
the installed application directory, so installing a new version does not migrate or remove them.

## One-time GitHub setup

Create these public repositories:

1. `khanhsnd/vibe-db-releases` — initialize it with a README on the `master` branch; GitHub Actions will
   create the releases and upload the installer, the macOS ZIP, the Linux archive and `checksums.txt`.
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

The workflow builds a Windows x64 NSIS installer, a universal macOS app and a Linux x64 archive, computes
SHA-256 checksums, publishes the assets to the public release repository, and updates the Homebrew Cask.
A release is not complete until every publish job is green.

What a reader does with those assets is the Install section of [the README](../../README.md#install).
A Homebrew install is updated with:

```sh
brew upgrade --cask vibe-db
```

The app's Settings → Updates page checks the GitHub release API. A Windows update is downloaded and its
checksum is checked against `checksums.txt` before the NSIS installer starts and the app exits. That
installer's manifest requests administrator, which `CreateProcess` refuses outright, so it is started
through the shell — Windows shows its elevation prompt, and declining it leaves the app running. macOS
only displays the Homebrew command or the release page because this project has no Apple Developer
certificate or notarization; Gatekeeper may require the first launch to be confirmed manually. Linux
only displays the release page: the archive holds a binary rather than a package, so where the previous
copy lives is the reader's business and not this app's to guess.

**Nothing published here is signed.** The Windows installer carries no Authenticode signature, so
Defender SmartScreen shows *"Windows protected your PC"* on a freshly downloaded build until the reader
chooses **More info → Run anyway**. That is the absence of a code-signing certificate, not a fault in the
build — see [Why Windows warns](#why-windows-warns).

## Why Windows warns

Two different Windows dialogs appear around an install, and they have different causes:

| Dialog | Cause | What removes it |
| --- | --- | --- |
| *"Windows protected your PC" — Unknown publisher* | Defender SmartScreen: the downloaded file carries the Mark of the Web and the binary has no Authenticode signature and no download reputation | An OV or EV code-signing certificate, `signtool sign /fd sha256 /tr <timestamp server> /td sha256` over the installer **and** the packaged `vibe-db.exe` |
| *User Account Control — Do you want to allow this app to make changes?* | The NSIS installer's manifest requests administrator, deliberately | Nothing; it is the elevation prompt an installer is supposed to raise |

An **OV** certificate still starts with no reputation, so SmartScreen may keep warning until enough
copies of that signature have been downloaded; an **EV** certificate is trusted immediately. Neither is
free, and neither is set up in this repository — there is no signing step in `.github/workflows/release.yml`
and no certificate in the Actions secrets.

## Local verification

The Wails v2 CLI supports the NSIS installer and universal macOS target:

```powershell
wails build -platform windows/amd64 -nsis
```

```sh
wails build -platform darwin/universal
ditto -c -k --sequesterRsrc --keepParent build/bin/vibe-db.app vibe-db.zip
```

```sh
wails build -platform linux/amd64
tar -czf vibe-db-linux-x64.tar.gz -C build/bin vibe-db
```

The Linux build needs GTK 3 and WebKit2GTK headers present — `libgtk-3-dev libwebkit2gtk-4.0-dev` on
Ubuntu 22.04, which is the runner the release job uses so the binary keeps that glibc floor. A
distribution carrying only WebKit2GTK 4.1 needs `wails build -tags webkit2_41` instead.

Run `go test ./...` before tagging. The updater logic is covered by focused tests in
`internal/update/service_test.go`.
