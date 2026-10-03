cask "norka" do
  version "1.3.0"
  sha256 "ad0ceb47144e8ea584c9ba0484dc8fe10459da835e173cfa069eb23b957c7327"

  url "https://github.com/norka-app/Norka/releases/download/v#{version}/norka.dmg"
  name "Norka"
  desc "SSH tunnel manager"
  homepage "https://github.com/norka-app/Norka"

  # Сборка ad-hoc, без Apple Developer ID и нотаризации.
  # Gatekeeper помечает скачанный .dmg карантином; caveats объясняет, как его снять.
  app "norka.app"

  caveats <<~EOS
    Norka is ad-hoc signed and is not notarized. macOS quarantines the app
    after download, and the first launch may be blocked.

    If Gatekeeper refuses to open it:

      xattr -dr com.apple.quarantine /Applications/norka.app

    Or: System Settings → Privacy & Security → Open Anyway.
  EOS

  zap trash: "~/.norka"

  livecheck do
    url :url
    strategy :github_latest
  end
end
