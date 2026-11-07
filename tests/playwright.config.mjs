// Browser tests for deckfold outputs (over file://). Firefox is not configured: the Playwright
// build does not start on macOS 27.2 (see docs/spikes/S2-file-protocol.md).
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 90_000,
  workers: 2,
  reporter: [['list']],
  globalSetup: './e2e/setup.mjs',
  use: { viewport: { width: 1400, height: 820 } },
  projects: [
    {
      name: 'chrome',
      use: { browserName: 'chromium', channel: 'chrome', launchOptions: { args: ['--autoplay-policy=no-user-gesture-required'] } },
      testIgnore: /webkit\.spec\.mjs/,
    },
    { name: 'webkit', use: { browserName: 'webkit' }, testMatch: /webkit\.spec\.mjs/ },
  ],
});
