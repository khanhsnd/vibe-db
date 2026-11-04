#!/usr/bin/env bash
# Writes the Homebrew Cask for one release: write-cask.sh <version> <sha256> <output file>.
# The ZIP is always downloaded from this repository's own releases.
set -euo pipefail

version="$1"
sha256="$2"
out="$3"

mkdir -p "$(dirname "$out")"
cat > "$out" <<CASK
cask 'vibe-db' do
  version '${version}'
  sha256 '${sha256}'

  url "https://github.com/khanhsnd/vibe-db/releases/download/v#{version}/vibe-db-#{version}-macos-universal.zip"
  name 'Vibe DB'
  desc 'Database desktop client'
  homepage 'https://github.com/khanhsnd/vibe-db'

  app 'vibe-db.app'
end
CASK
