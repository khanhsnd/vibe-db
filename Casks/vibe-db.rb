cask 'vibe-db' do
  version '1.5.0'
  sha256 'bc4e33cc4149bf22a60e1c12f52d0f93b3480215fa8f8988e0af8abacc5b516c'

  url "https://github.com/khanhsnd/vibe-db/releases/download/v#{version}/vibe-db-#{version}-macos-universal.zip"
  name 'Vibe DB'
  desc 'Database desktop client'
  homepage 'https://github.com/khanhsnd/vibe-db'

  app 'vibe-db.app'
end
