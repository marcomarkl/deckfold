import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, audState, counter, slideCount, expected, need } from './helpers.mjs';

test.beforeEach(() => need('keynote'));

test('Keynote solo: the player runs standalone, navigation', async ({ page, context }) => {
  const w = watch(context);
  await solo(page, 'keynote.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 15000 });
  await page.waitForTimeout(1500);
  const n = await slideCount(page);
  const st = () => audState(page);
  await page.keyboard.press('ArrowRight'); await page.waitForTimeout(900);
  await page.keyboard.press('ArrowRight'); await page.waitForTimeout(900);
  const s2 = await expected(page, ['next', 'next']);
  await expect.poll(st).toEqual(s2);
  // the player's own slide change event confirms the state
  const idx = await page.evaluate(() => new Promise((res) => {
    document.addEventListener('ShowController:SlideIndexDidChangeEvent', (e) => res(e.detail.slideIndex), { once: true });
    window.__deckfold.audience.act('next');
  }));
  expect(idx).toBe((await expected(page, ['next'], s2)).slide);
  const target = Math.min(12, n);
  for (const d of String(target)) await page.keyboard.press(d);
  await page.keyboard.press('Enter');
  await expect.poll(st).toEqual(await expected(page, [['goto', target - 1]]));
  // Escape never reaches the player (it would leave the show)
  await page.keyboard.press('Escape');
  await page.waitForTimeout(500);
  expect(page.url()).toContain('keynote.html');
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('Keynote presenter view: previews render, no notes', async ({ page, context }) => {
  const w = watch(context);
  const aud = await present(page, context, 'keynote.html');
  await expect(page.locator('.df-notes')).toBeHidden();
  const canvases = () => page.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) => f.contentDocument.querySelectorAll('#stage canvas').length));
  await expect.poll(async () => (await canvases()).every((c) => c > 0), { timeout: 20000 }).toBe(true);
  await page.waitForTimeout(1000);
  const n = await slideCount(page);
  for (let i = 0; i < 3; i++) { await page.keyboard.press('ArrowRight'); await page.waitForTimeout(700); }
  const s3 = await expected(page, ['next', 'next', 'next']);
  await expect.poll(() => audState(aud)).toEqual(s3);
  await expect.poll(() => counter(page)).toContain(`${s3.slide + 1} / ${n}`);
  await page.click('.df-bar button[data-k="overview"]');
  await expect(page.locator('.df-tile img')).toHaveCount(n);
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});
