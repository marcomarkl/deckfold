import { test, expect } from '@playwright/test';
import { solo, startScreen } from './helpers.mjs';

test('generic: no presenter view on the start screen', async ({ page }) => {
  await startScreen(page, 'generic.html');
  await expect(page.locator('[data-df-presenter]')).toHaveCount(0);
  await expect(page.locator('[data-df-auto]')).toHaveCount(0);
  await expect(page.locator('[data-df-strip]')).toHaveCount(0);
});

test('generic: the source gets keys directly, the live embed stays live', async ({ page }) => {
  await solo(page, 'generic.html');
  await page.keyboard.press('ArrowDown');
  await expect.poll(() => page.evaluate(() => window.keys)).toEqual(['ArrowDown']);
  expect(await page.getAttribute('#live', 'src')).toBe('https://example.com/');
  await expect(page.locator('.df-offline')).toHaveCount(0);
});

test('live embed without network: placeholder', async ({ page, context }) => {
  await solo(page, 'generic.html');
  await context.setOffline(true);
  await expect(page.locator('.df-offline')).toBeVisible();
  await expect(page.locator('#live')).toBeHidden();
  await context.setOffline(false);
  await expect(page.locator('.df-offline')).toHaveCount(0);
});
