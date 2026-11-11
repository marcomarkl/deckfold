// Browser tests for deckfold outputs (over file://). Firefox runs only outside macOS: the
// Playwright build does not start on macOS 27.2 (see docs/spikes/S2-file-protocol.md).
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
      // SwiftShader gives WebGL (3D shapes) on machines without a GPU, such as the CI runners
      use: { browserName: 'chromium', channel: 'chrome', launchOptions: { args: ['--autoplay-policy=no-user-gesture-required', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] } },
      testIgnore: /webkit\.spec\.mjs/,
    },
    // the start screen tests run in every project: its layout rests on CSS the browsers could size differently
    { name: 'webkit', use: { browserName: 'webkit' }, testMatch: /(webkit|start)\.spec\.mjs/ },
    // Firefox only where its Playwright build starts (not on macOS 27.2): the PowerPoint tests
    // and the start screen
    ...(process.platform === 'darwin' ? [] : [{
      name: 'firefox',
      use: { browserName: 'firefox', launchOptions: { firefoxUserPrefs: { 'media.autoplay.default': 0, 'webgl.force-enabled': true } } },
      testMatch: /(pptx|start)\.spec\.mjs/,
    }]),
  ],
});
