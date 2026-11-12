cask 'vibe-db' do
  version '1.4.0'
  sha256 '17c1f9aa0b10828785645b4a59e5376e4a775c3b9d86ecacbae5cc2a95e2162f'

  url "https://github.com/khanhsnd/vibe-db/releases/download/v#{version}/vibe-db-#{version}-macos-universal.zip"
  name 'Vibe DB'
  desc 'Database desktop client'
  homepage 'https://github.com/khanhsnd/vibe-db'

  app 'vibe-db.app'
end
