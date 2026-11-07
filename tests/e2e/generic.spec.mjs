import { test, expect } from '@playwright/test';
import { present, fileURL } from './helpers.mjs';

test('generic: keys from the presenter view reach the source', async ({ page, context }) => {
  const aud = await present(page, context, 'generic.html');
  await expect(page.locator('.df-generic')).toBeVisible();
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('x');
  await expect.poll(() => aud.evaluate(() => window.keys)).toEqual(['ArrowRight', 'x']);
});

test('generic solo: the source gets keys directly, the live embed stays live', async ({ page }) => {
  await page.goto(fileURL('generic.html'));
  await page.click('.df-start-buttons button:nth-child(2)');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await page.keyboard.press('ArrowDown');
  await expect.poll(() => page.evaluate(() => window.keys)).toEqual(['ArrowDown']);
  expect(await page.getAttribute('#live', 'src')).toBe('https://example.com/');
  await expect(page.locator('.df-offline')).toHaveCount(0);
});

test('live embed without network: placeholder', async ({ page, context }) => {
  await page.goto(fileURL('generic.html'));
  await page.click('.df-start-buttons button:nth-child(2)');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await context.setOffline(true);
  await expect(page.locator('.df-offline')).toBeVisible();
  await expect(page.locator('#live')).toBeHidden();
  await context.setOffline(false);
  await expect(page.locator('.df-offline')).toHaveCount(0);
});
