// Fallback browser (E3): WebKit is close to Safari and has no Window Management API.
import { test, expect } from '@playwright/test';
import { solo, present, audState, counter, slideCount, expected, need } from './helpers.mjs';

test('WebKit: solo navigation in a claude.ai deck', async ({ page }) => {
  need('claude');
  await solo(page, 'claude.html');
  await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next', 'next']));
});

test('WebKit: presenter view with the hint to drag the window', async ({ page, context }) => {
  need('claude');
  const aud = await present(page, context, 'claude.html');
  await expect(page.locator('.df-banner.df-on')).toBeVisible();
  await page.keyboard.press('ArrowRight');
  const s1 = await expected(page, ['next']);
  await expect.poll(() => audState(aud)).toEqual(s1);
  await expect.poll(() => counter(page)).toContain(`${s1.slide + 1} / ${await slideCount(page)}`);
});

test('WebKit: Keynote deck runs', async ({ page }) => {
  need('keynote');
  await solo(page, 'keynote.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 20000 });
  await page.waitForTimeout(1000);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next']));
});
