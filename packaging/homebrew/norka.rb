cask "norka" do
  version "1.4.0"
  sha256 "1a5a985f457b9d24d4a7e28162239bf46cb5f0df65893a966578f9d16587ea2f"

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
