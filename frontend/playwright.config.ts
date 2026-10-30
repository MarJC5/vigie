import { mkdirSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { defineConfig, devices } from "@playwright/test";

// The folder of the files the data test exports and imports, one per browser; made once, and kept
// by the workers, which read this configuration again with the same environment.
process.env.VIGIE_E2E_FILES ??= mkdtempSync(join(tmpdir(), "vigie-e2e-"));
export const e2eFiles = (browser: string) => {
  const dir = join(process.env.VIGIE_E2E_FILES!, browser);
  mkdirSync(dir, { recursive: true });
  return dir;
};

// End-to-end test of the window: the Go binary, built in server mode for the test only, serves the
// interface to a browser, on a data directory prepared by cmd/vigie/testdata/seed.
// The interface is built first (make frontend-build): the binary embeds frontend/dist.
const server = (port: number, seed: string, files?: string) => ({
  command:
    `sh -c 'd="$(mktemp -d)" && go build -tags server -o "$d/vigie" ./cmd/vigie` +
    ` && go run ./cmd/vigie/testdata/seed ${seed} "$d/data"` +
    ` && WAILS_SERVER_PORT=${port} exec "$d/vigie" --data-dir "$d/data"'`,
  cwd: "..",
  url: `http://localhost:${port}/`,
  timeout: 300_000,
  stdout: "ignore" as const,
  stderr: "ignore" as const, // the request log of Wails; a failing build still fails the run
  // The server build has no file dialogs: its files go to and come from this folder.
  env: files ? { VIGIE_E2E_FILES: files } : undefined,
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
  // The editing and security tests change their data: one server for each browser.
  webServer: [
    server(34115, "-lang en"),
    server(34116, "-lang fr -empty"),
    server(34117, "-lang en"),
    server(34118, "-lang en"),
    server(34119, "-lang en"),
    server(34120, "-lang en"),
    // The data test: an export of the seed, imported into an empty Vigie.
    server(34121, "-lang en", e2eFiles("chromium")),
    server(34122, "-lang en -empty", e2eFiles("chromium")),
    server(34123, "-lang en", e2eFiles("webkit")),
    server(34124, "-lang en -empty", e2eFiles("webkit")),
  ],
});
