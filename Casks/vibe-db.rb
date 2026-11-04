cask 'vibe-db' do
  version '1.3.0'
  sha256 '79a31f747da9e6d6c23f47c9974351eb2ff1f96ed850356360e7b704db958645'

  url "https://github.com/khanhsnd/vibe-db/releases/download/v#{version}/vibe-db-#{version}-macos-universal.zip"
  name 'Vibe DB'
  desc 'Database desktop client'
  homepage 'https://github.com/khanhsnd/vibe-db'

  app 'vibe-db.app'
end
