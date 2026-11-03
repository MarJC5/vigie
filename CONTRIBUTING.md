# Contributing to Vigie

Thank you for helping. Vigie is a desktop app in Go ([Wails 3](https://v3.wails.io/)) with a
React and TypeScript window. This page says how to build it, how it is tested, and what a change
needs before it is merged.

## Before you start

- **A bug**: open an issue with the bug form; say how Vigie was installed and what the server runs.
  Leave out passwords, keys, host names and site addresses you do not want public.
- **A security problem**: not in an issue; see [SECURITY.md](SECURITY.md).
- **A new feature or a larger change**: open an issue first, to agree on what it should do. Vigie
  holds to a few rules that shape every change (below), and it is easier to settle them before the
  code.

## What every change keeps

- **Agentless**: nothing is installed on the sites; Vigie works over SSH and WP-CLI.
- **Read-only by default**: anything that writes on a server says what it will do, asks first, and
  is written in the journal.
- **No secrets stored**: SSH keys stay in the user's agent; Vigie never asks for a password.
- **Nothing sent without saying so**: every outside request is listed in Settings, Outside lookups
  and in [docs/security.md](docs/security.md), with what it receives, and can be turned off.
- **Two languages**: every text of the window and the command line is in
  [`locales/en.json`](locales/en.json) and [`locales/fr.json`](locales/fr.json); every page of
  `docs/` has its French translation in `docs/fr/`, with the same headings (a test checks it).
- **Few dependencies**: a new one is discussed first; the standard library is preferred.

## Setting up

You need:

- Go, the version of [`go.mod`](go.mod);
- Node.js, the version of [`.nvmrc`](.nvmrc);
- on macOS, the Xcode command line tools; on Linux, GTK 4 and WebKitGTK 6.0 with their development
  files (`libgtk-4-dev libwebkitgtk-6.0-dev libwayland-dev build-essential pkg-config` on Debian
  and Ubuntu);
- Docker, for the integration tests only.

Then:

```sh
make frontend-install    # the window's packages
make frontend-browsers   # Chromium and WebKit, for the component and end-to-end tests
make check               # everything CI runs, below
make app                 # Vigie.app on macOS, or the Linux packages, in bin/
```

The Wails command is pinned and installed in `.task/bin` by the Makefile; `make wails` installs it
alone. To run the window while you work on it: `.task/bin/wails3 dev -config ./build/config.yml`.
To work on a component alone: `cd frontend && npm run storybook`.

## Tests

Changes come with their tests, written first when it fits.

| Command | What it runs |
| --- | --- |
| `make go-check` | `gofmt`, `go vet`, the Go tests with the race detector, `govulncheck` |
| `make frontend-lint` | ESLint and the TypeScript check |
| `make frontend-test` | Every Storybook story with its interactions and accessibility checks, in Chromium and WebKit |
| `make frontend-e2e` | The window end to end, against Vigie built in server mode |
| `make bindings-check` | The TypeScript bindings match the Go services (`make bindings` regenerates them) |
| `make check` | All of the above |
| `make test-integration` | WordPress, WP-CLI and an SSH server in Docker containers, scanned and updated for real |

Tests never reach a real server: they replay recorded WP-CLI outputs
(`internal/wpcli/testdata/recorded`), which `make record` records again from the Docker bench.

## Commits and pull requests

- Pull requests target `develop`, where the work happens. `main` holds the latest release and moves
  only when one is made.
- One change per pull request, with its tests and its docs.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/), in English:
  `fix(window): …`, `feat: …`, `docs: …`.
- Add a line to [`CHANGELOG.md`](CHANGELOG.md) under `Unreleased` for anything a user would notice.
- `make check` passes before the pull request is opened; CI runs it again, with the integration
  tests.

## License

By contributing, you agree that your contribution is licensed under the [MIT license](LICENSE) of
the project.
