# The cask of the tap MarJC5/tap (ADR-0012), copied there at each release with its version and the
# DMG's SHA-256; the release fills them in. It links the vigie command (ADR-0036).
cask "vigie" do
  version "0.0.0"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"

  url "https://github.com/MarJC5/vigie/releases/download/v#{version}/Vigie-#{version}.dmg"
  name "Vigie"
  desc "Monitor and update WordPress sites over SSH and WP-CLI"
  homepage "https://github.com/MarJC5/vigie"

  depends_on macos: ">= :monterey"

  app "Vigie.app"
  binary "#{appdir}/Vigie.app/Contents/MacOS/Vigie", target: "vigie"

  zap trash: "~/Library/Application Support/Vigie"
end
