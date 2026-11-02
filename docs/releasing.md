# Releasing Vigie

A release is made by the owner, on a Mac: the macOS app is built there, and the key that signs
every release never leaves it. GitHub Actions builds the Linux packages from the tag, without any
secret. An installed Vigie trusts a new version only if it is signed with the key it embeds.

## What a release carries

| File | For |
| --- | --- |
| `Vigie-x.y.z.dmg` | People installing on macOS, and the Homebrew cask |
| `Vigie-x.y.z-macos.zip` | Vigie updating itself on macOS (the universal app) |
| `Vigie-x.y.z-x86_64.AppImage`, `-aarch64.AppImage` | People on Linux, and Vigie updating itself there |
| `Vigie-x.y.z-amd64.deb`, `-arm64.deb`, `.rpm`, `.pkg.tar.zst` | Linux package managers |
| `SHA256SUMS` | The checksum of every file |
| `update.json` | What Vigie reads to learn of a new version: each file it installs, signed |

Vigie reads `update.json` of the latest published release only: never a draft nor a prerelease.

## Once: the key pair

```sh
go run ./cmd/vigie-release keygen ~/somewhere-private
```

It writes the private key (`vigie-release.key`, readable by you only) and the public key, which it
prints. Keep the private key outside the repository, with a copy in your password manager: a
release signed with another key is refused by every installed Vigie. The public key goes in
`internal/buildinfo/update.go` (`UpdateKey`). A key is never overwritten.

## Making a release

1. In `CHANGELOG.md`, move what is under `Unreleased` to a new section `## [x.y.z] - YYYY-MM-DD`:
   its text becomes the release's notes, in the window too.
2. Commit, then tag and push:

   ```sh
   git tag vx.y.z
   git push origin main vx.y.z
   ```

   The tag runs the Release workflow: the CI's checks, then the Linux packages for x86_64 and
   aarch64.
3. On the Mac, from the tag:

   ```sh
   make release VERSION=x.y.z KEY=~/somewhere-private/vigie-release.key
   ```

   It waits for the workflow, builds the universal app, its zip and the DMG, fetches the Linux
   builds, signs every file, checks the signatures with Wails and the key in the app, and creates a
   **draft** release with all the files.
4. Read the draft on GitHub, then publish it: `gh release edit vx.y.z --draft=false`. From then on,
   installed copies of Vigie offer it.
5. Update the cask in the tap `MarJC5/homebrew-tap` (`Casks/vigie.rb`) with the version and the
   SHA-256 that `make release` prints, or copy `dist/vx.y.z/vigie.rb`.

## Checking an update before a release

An update can be tried from a server on your Mac, signed with the same key: build and install a
lower version, then serve a higher one.

```sh
make app VERSION=0.0.1          # copy bin/Vigie.app to /Applications
make app VERSION=0.0.2
mkdir -p dist/try && ditto -c -k --keepParent bin/Vigie.app dist/try/Vigie-0.0.2-macos.zip
go run ./cmd/vigie-release sign -key ~/somewhere-private/vigie-release.key -version 0.0.2 \
  -base http://127.0.0.1:8000 dist/try/Vigie-0.0.2-macos.zip
python3 -m http.server 8000 --bind 127.0.0.1 --directory dist/try
VIGIE_UPDATE_MANIFEST=http://127.0.0.1:8000/update.json /Applications/Vigie.app/Contents/MacOS/Vigie </dev/null
```

`VIGIE_UPDATE_MANIFEST` only changes where the manifest is read; the signature is checked with the
key in the app all the same. The input from `/dev/null` keeps the app in this process, with the
variable, instead of handing over to the Finder. In Settings, accept the outside lookups if asked,
then About, "Check for updates".

## The version in a build

`make app VERSION=x.y.z` writes the version in the binary (`vigie version`), in the app's
`Info.plist` and in the Linux packages. Without it, the build says `dev`, and never looks for
updates.
