cask 'vibe-db' do
  version '1.4.1'
  sha256 '3dd9f34ba37b94f6a189ddb286ee549c40f7071d93f9ead03304e6efb9f7687b'

  url "https://github.com/khanhsnd/vibe-db/releases/download/v#{version}/vibe-db-#{version}-macos-universal.zip"
  name 'Vibe DB'
  desc 'Database desktop client'
  homepage 'https://github.com/khanhsnd/vibe-db'

  app 'vibe-db.app'
end
