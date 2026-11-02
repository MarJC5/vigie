#!/usr/bin/env bash
# make release VERSION=x.y.z KEY=<private key> (ADR-0038), on the owner's Mac, once the tag vx.y.z
# is pushed: the universal app, its zip and DMG, the tag's Linux builds from GitHub Actions, every
# artifact signed, SHA256SUMS and update.json checked by Wails with the key the app embeds, then a
# draft release on GitHub. Publishing the draft, and the cask in the tap, stay the owner's to do.
set -euo pipefail

fail() { echo "release: $*" >&2; exit 1; }
step() { printf '\n== %s\n' "$*"; }

: "${VERSION:?VERSION=x.y.z is needed}"
: "${KEY:?KEY=<the private key written by vigie-release keygen> is needed}"
: "${WAILS_BIN:?run through make release}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "VERSION is x.y.z, without v: $VERSION"
[[ "$(uname -s)" == Darwin ]] || fail "the macOS app is built on a Mac"
[[ -r "$KEY" ]] || fail "cannot read the key $KEY"
for tool in gh git ditto lipo plutil shasum; do
	command -v "$tool" >/dev/null || fail "$tool is needed"
done

tag="v$VERSION"
out="dist/$tag"
export PATH="$WAILS_BIN:$PATH"

step "Checks"
[[ -z "$(git status --porcelain)" ]] || fail "the working tree has changes: commit them first"
git fetch -q origin tag "$tag" 2>/dev/null || fail "the tag $tag is not on GitHub: git tag $tag && git push origin $tag"
[[ "$(git rev-parse "$tag^{commit}")" == "$(git rev-parse HEAD)" ]] || fail "check out $tag first: git switch --detach $tag"
if gh release view "$tag" >/dev/null 2>&1; then
	fail "a release $tag exists already"
fi
rm -rf "$out"
mkdir -p "$out"
go run ./cmd/vigie-release notes -version "$VERSION" CHANGELOG.md >"$out/notes.md" ||
	fail "CHANGELOG.md needs a section ## [$VERSION]"

step "Linux builds of the tag (GitHub Actions)"
run="$(gh run list --workflow release.yml --branch "$tag" --event push --limit 1 --json databaseId --jq '.[0].databaseId')"
[[ -n "$run" ]] || fail "no Release workflow ran for $tag"
gh run watch "$run" --exit-status >/dev/null || fail "the Release workflow failed for $tag: gh run view $run"
gh run download "$run" --dir "$out/linux"
find "$out/linux" -type f -exec mv {} "$out" \;
rm -rf "$out/linux"

step "The universal macOS app"
wails3 task darwin:package:universal VERSION="$VERSION"
app=bin/Vigie.app
archs="$(lipo -archs "$app/Contents/MacOS/Vigie")"
[[ "$archs" == *x86_64* && "$archs" == *arm64* ]] || fail "the app is not universal: $archs"
[[ "$(plutil -extract CFBundleShortVersionString raw "$app/Contents/Info.plist")" == "$VERSION" ]] ||
	fail "Info.plist does not carry $VERSION"
[[ "$("$app/Contents/MacOS/Vigie" version)" == "vigie $VERSION" ]] || fail "the binary does not carry $VERSION"
codesign --verify --deep --strict "$app"
# The universal binary, now in the app: go build refuses to overwrite it at the next make app.
rm -f bin/Vigie
ditto -c -k --keepParent "$app" "$out/Vigie-$VERSION-macos.zip"
wails3 task darwin:create:dmg
mv bin/Vigie.dmg "$out/Vigie-$VERSION.dmg"

step "Signatures"
artifacts=()
while IFS= read -r file; do artifacts+=("$file"); done < <(find "$out" -type f -name 'Vigie-*' | sort)
go run ./cmd/vigie-release sign -key "$KEY" -version "$VERSION" -notes "$out/notes.md" "${artifacts[@]}"
(cd "$out" && shasum -a 256 -c SHA256SUMS)
# Checked by Wails itself, with the key the app embeds: what an installed Vigie will do.
public="$(sed -n 's/^const UpdateKey = "\(.*\)"$/\1/p' internal/buildinfo/update.go)"
[[ -n "$public" ]] || fail "no UpdateKey in internal/buildinfo/update.go"
wails3 updater verify -manifest "$out/update.json" -publickey "$public"

step "Draft release on GitHub"
gh release create "$tag" --draft --verify-tag --title "Vigie $VERSION" --notes-file "$out/notes.md" \
	"${artifacts[@]}" "$out/SHA256SUMS" "$out/update.json"

dmg_sum="$(shasum -a 256 "$out/Vigie-$VERSION.dmg" | cut -d' ' -f1)"
sed -e "s/^  version \".*\"/  version \"$VERSION\"/" -e "s/^  sha256 \".*\"/  sha256 \"$dmg_sum\"/" \
	build/darwin/homebrew/vigie.rb >"$out/vigie.rb"

cat <<EOF

Draft release $tag ready: $(gh release view "$tag" --json url --jq .url)
Read it, then publish it (the updater sees only a published release):
  gh release edit $tag --draft=false
Then the cask, in the tap MarJC5/homebrew-tap (Casks/vigie.rb), from $out/vigie.rb:
  version "$VERSION"
  sha256 "$dmg_sum"
EOF
