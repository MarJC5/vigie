package buildinfo

// UpdateKey is the public key a new version of Vigie must be signed with (ADR-0038), base64 of the
// Ed25519 key vigie-release keygen made; its private key never leaves the owner's Mac.
const UpdateKey = "I8BAemHIXhTRX1gg1FbnA6vg/KkP9QzGxjSAeV62sKQ="

// UpdateManifest is where Vigie reads its latest version: the update.json of GitHub's latest
// release, never a draft nor a prerelease.
const UpdateManifest = "https://github.com/MarJC5/vigie/releases/latest/download/update.json"
