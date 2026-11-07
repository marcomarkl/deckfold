import { test, expect } from '@playwright/test';
import { fileURL } from './helpers.mjs';

// Screen choice with a mocked Window Management API: call order, coordinates, full-screen fallback.
test('screen choice: requestFullscreen before window.open, popup on the target screen', async ({ page, context }) => {
  await page.addInitScript(() => {
    const screens = [
      { label: 'Built-in', width: 1728, height: 1117, left: 0, top: 0, availLeft: 0, availTop: 25, availWidth: 1728, availHeight: 1092, isPrimary: false },
      { label: 'Projector', width: 1920, height: 1080, left: 1728, top: 0, availLeft: 1728, availTop: 0, availWidth: 1920, availHeight: 1080, isPrimary: true },
    ];
    window.__calls = [];
    window.getScreenDetails = async () => ({ screens, currentScreen: screens[0], addEventListener() {} });
    Element.prototype.requestFullscreen = function (opts) { window.__calls.push(['fullscreen', opts && opts.screen && opts.screen.label]); return Promise.resolve(); };
    const origOpen = window.open;
    window.open = function (url, name, feat) { window.__calls.push(['open', name.split(':')[0], feat]); return origOpen.call(window, url, name, feat); };
  });
  await page.goto(fileURL('generic.html'));
  await page.click('.df-start-buttons button:nth-child(1)');
  await expect(page.locator('#df-aud')).toBeVisible();
  // preselection: audience on the screen that is not the current one
  expect(await page.inputValue('#df-aud')).toBe('1');
  expect(await page.inputValue('#df-pre')).toBe('0');
  const pp = context.waitForEvent('page');
  await page.click('.df-dialog .df-primary');
  await pp;
  const calls = await page.evaluate(() => window.__calls);
  expect(calls[0]).toEqual(['fullscreen', 'Built-in']);
  expect(calls[1][0]).toBe('open');
  expect(calls[1][1]).toBe('df-audience');
  expect(calls[1][2]).toContain('left=1728');
  expect(calls[1][2]).toContain('width=1920');
  expect(calls[1][2]).toContain('fullscreen');
  // the choice is remembered
  expect(await page.evaluate(() => JSON.parse(localStorage.getItem('deckfold:screens')).aud.label)).toBe('Projector');
});

test('full-screen fallback in the audience window when full screen fails without a gesture', async ({ page }) => {
  await page.addInitScript(() => {
    Element.prototype.requestFullscreen = function () { return Promise.reject(new TypeError('no gesture')); };
    Object.defineProperty(window.screen, 'width', { get: () => 3000 });
    Object.defineProperty(window.screen, 'height', { get: () => 2000 });
  });
  await page.goto(fileURL('generic.html'));
  await page.click('.df-start-buttons button:nth-child(2)');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect(page.locator('.df-fs-overlay')).toBeVisible();
});
