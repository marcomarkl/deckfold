import { test, expect } from '@playwright/test';
import { solo, present, audState, expected, need } from './helpers.mjs';

test.beforeEach(() => need('claude'));

test('auto mode keeps running in the audience window, even without the presenter view', async ({ page, context }) => {
  const aud = await present(page, context, 'claude-auto.html');
  await expect.poll(() => aud.evaluate(() => window.__deckfold.audience.state().slide), { timeout: 8000 }).toBeGreaterThanOrEqual(1);
  await expect(page.locator('.df-auto-group button')).toHaveClass(/df-active/);
  // close the presenter view: the timer does not depend on it
  const before = await audState(aud);
  await page.close();
  await aud.waitForTimeout(3500);
  expect(await audState(aud)).not.toEqual(before);
});

test('auto mode with loop: back to the first slide after the last one', async ({ page }) => {
  await solo(page, 'claude-auto.html');
  await page.keyboard.press('End');
  const first = await expected(page, ['first']);
  await expect.poll(() => audState(page), { timeout: 15000 }).toEqual(first);
});

test('auto mode waits for playing media to end', async ({ page }) => {
  await solo(page, 'media.html');
  // the 4 s tone plays from load; start auto mode with a 1 s interval
  await page.keyboard.press('a');
  await expect.poll(async () => (await audState(page)).slide, { timeout: 8000 }).toBe(1);
  await expect.poll(() => page.evaluate(() => window.__deckfold.audience && document.getElementById('df-test-audio').paused === false)).toBe(true);
  // while the tone plays, the state stays put although the interval has passed
  const waiting = await audState(page);
  await page.waitForTimeout(1300);
  const audio = await page.evaluate(() => { const a = document.getElementById('df-test-audio'); return { playing: !a.paused && !a.ended }; });
  if (audio.playing) expect(await audState(page)).toEqual(waiting);
  await expect.poll(() => page.evaluate(() => document.getElementById('df-test-audio').ended), { timeout: 8000 }).toBe(true);
  await expect.poll(() => audState(page), { timeout: 3000 }).not.toEqual(waiting);
});
