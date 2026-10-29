import { defineConfig, devices } from "@playwright/test";

// End-to-end test of the window: the Go binary, built in server mode for the test only, serves the
// interface to a browser, on a data directory prepared by cmd/vigie/testdata/seed.
// The interface is built first (make frontend-build): the binary embeds frontend/dist.
const server = (port: number, seed: string) => ({
  command:
    `sh -c 'd="$(mktemp -d)" && go build -tags server -o "$d/vigie" ./cmd/vigie` +
    ` && go run ./cmd/vigie/testdata/seed ${seed} "$d/data"` +
    ` && WAILS_SERVER_PORT=${port} exec "$d/vigie" --data-dir "$d/data"'`,
  cwd: "..",
  url: `http://localhost:${port}/`,
  timeout: 300_000,
  stdout: "ignore" as const,
  stderr: "ignore" as const, // the request log of Wails; a failing build still fails the run
});

export default defineConfig({
  testDir: "e2e",
  forbidOnly: Boolean(process.env.CI),
  reporter: process.env.CI ? "github" : "list",
  use: { viewport: { width: 1280, height: 800 } },
  // WebKit stands for the macOS and Linux windows, Chromium for WebView2 on Windows.
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } } },
    { name: "webkit", use: { ...devices["Desktop Safari"], viewport: { width: 1280, height: 800 } } },
  ],
  // The editing tests change their data: one server for each browser.
  webServer: [
    server(34115, "-lang en"),
    server(34116, "-lang fr -empty"),
    server(34117, "-lang en"),
    server(34118, "-lang en"),
  ],
});
