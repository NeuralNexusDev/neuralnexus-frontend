import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = path.dirname(path.dirname(path.dirname(fileURLToPath(import.meta.url))));

// BASE_URL is set by docker-compose.test.yml, pointing at the containerized
// "frontend" service - in that mode the server is already running as its
// own container, so we skip webServer below. With no BASE_URL (a plain
// `npx playwright test` on a host with Go + Node installed), we fall back
// to spawning the real server ourselves.
const baseURL = process.env.BASE_URL || 'http://localhost:8099';

// The single source of truth for the backend origin the app (and every
// spec file, via process.env.API_BASE_URL) mocks against - resolved once
// here so both sides can never drift apart.
const apiBaseUrl = process.env.API_BASE_URL || 'https://api.neuralnexus.dev';
process.env.API_BASE_URL = apiBaseUrl;

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL,
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: process.env.BASE_URL
    ? undefined
    : {
        command:
          'go tool templ generate && ' +
          'go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --minify && ' +
          'go run .',
        cwd: repoRoot,
        url: baseURL,
        env: { ADDRESS: '0.0.0.0:8099', API_BASE_URL: apiBaseUrl },
        reuseExistingServer: !process.env.CI,
        timeout: 60_000,
      },
});
