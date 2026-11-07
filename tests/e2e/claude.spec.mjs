import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, audState, counter, slideCount, expected, need, fileURL } from './helpers.mjs';

test.beforeEach(() => need('claude'));

test('solo: no network requests, keyboard and clicker keys', async ({ page, context }) => {
  const w = watch(context);
  await solo(page, 'claude.html');
  expect(await page.evaluate(() => document.documentElement.classList.contains('deck-stage'))).toBe(true);
  const n = await slideCount(page);
  expect(n).toBeGreaterThanOrEqual(4);
  const st = () => audState(page);
  await page.keyboard.press('PageDown'); await page.keyboard.press('ArrowRight'); await page.keyboard.press(' ');
  const s3 = await expected(page, ['next', 'next', 'next']);
  await expect.poll(st).toEqual(s3);
  await expect.poll(() => page.evaluate(() => location.hash)).toBe('#' + (s3.slide + 1));
  await page.keyboard.press('PageUp');
  await expect.poll(st).toEqual(await expected(page, ['prev'], s3));
  await page.keyboard.press('End');
  await expect.poll(st).toEqual(await expected(page, ['last']));
  await page.keyboard.press('Home');
  await expect.poll(st).toEqual(await expected(page, ['first']));
  // digits + Enter
  const target = Math.min(7, n);
  for (const d of String(target)) await page.keyboard.press(d);
  await page.keyboard.press('Enter');
  await expect.poll(st).toEqual(await expected(page, [['goto', target - 1]]));
  await expect.poll(() => page.evaluate(() => location.hash)).toBe('#' + target);
  // black / white screen
  await page.keyboard.press('b');
  await expect(page.locator('.df-blank.df-black')).toBeVisible();
  await page.keyboard.press('b');
  await expect(page.locator('.df-blank')).toBeHidden();
  await page.keyboard.press('w');
  await expect(page.locator('.df-blank.df-white')).toBeVisible();
  await page.keyboard.press('w');
  // end of the presentation
  await page.keyboard.press('End');
  const lastSteps = await page.evaluate(() => { const s = window.__deckfold.slides(); return s[s.length - 1].steps; });
  for (let i = 0; i < lastSteps; i++) await page.keyboard.press('ArrowRight');
  await expect(page.locator('.df-endslide')).toBeVisible();
  await page.keyboard.press('ArrowLeft');
  await expect(page.locator('.df-endslide')).toBeHidden();
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('presenter view: sync, previews, notes, overview', async ({ page, context }) => {
  const w = watch(context);
  const aud = await present(page, context, 'claude.html');
  await expect(page.locator('.df-presenter')).toBeVisible();
  await expect(page.locator('iframe[data-df-preview]')).toHaveCount(2);
  const n = await slideCount(page);
  for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowRight');
  const s3 = await expected(page, ['next', 'next', 'next']);
  await expect.poll(() => audState(aud)).toEqual(s3);
  await expect.poll(() => counter(page)).toContain(`${s3.slide + 1} / ${n}`);
  // previews: slide shown by the player in the current and the next preview
  const nx = await expected(page, ['next'], s3);
  const visible = () => page.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) =>
    [...f.contentDocument.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off'))));
  await expect.poll(visible, { timeout: 10000 }).toEqual([s3.slide, nx.slide]);
  expect(await aud.evaluate(() => [...document.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off')))).toBe(s3.slide);
  const note = await page.evaluate((i) => window.__deckfold.slides()[i].notes || window.__deckfold.t('noNotes'), s3.slide);
  await expect(page.locator('.df-notes-text')).toHaveText(note);
  // a key in the audience window works too
  await aud.keyboard.press('ArrowRight');
  await expect.poll(() => counter(page)).toContain(`${nx.slide + 1} / ${n}`);
  // overview: a click jumps
  await page.click('.df-bar button[data-k="overview"]');
  await expect(page.locator('.df-tile')).toHaveCount(n);
  await page.click('.df-tile[data-i="1"]');
  await expect.poll(() => audState(aud)).toEqual(await expected(page, [['goto', 1]]));
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('settings survive a reload', async ({ page }) => {
  await page.goto(fileURL('claude.html'));
  await page.click('.df-start-buttons button:nth-child(3)');
  await page.uncheck('#df-el-notes');
  await page.fill('#df-notes-size', '40');
  await page.click('.df-dialog .df-primary');
  await page.reload();
  const s = await page.evaluate(() => window.__deckfold.settings());
  expect(s.presenter.elements).not.toContain('notes');
  expect(s.presenter.notesFontSize).toBe(40);
  // only the differences are stored
  const stored = await page.evaluate(() => Object.keys(localStorage).filter((k) => k.startsWith('deckfold:')).map((k) => JSON.parse(localStorage.getItem(k))));
  expect(stored.some((v) => v.presenter && v.presenter.notesFontSize === 40 && !('auto' in v))).toBe(true);
  // reset
  await page.click('.df-start-buttons button:nth-child(3)');
  await page.click('.df-dialog-buttons button:first-child');
  expect((await page.evaluate(() => window.__deckfold.settings())).presenter.notesFontSize).toBe(22);
});

test('recovery: presenter view reloaded, audience window closed', async ({ page, context }) => {
  const aud = await present(page, context, 'claude.html');
  const n = await slideCount(page);
  await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight');
  const s2 = await expected(page, ['next', 'next']);
  await expect.poll(() => audState(aud)).toEqual(s2);
  // reload the presenter view: state and connection come back through the heartbeat
  await page.reload();
  await expect(page.locator('.df-presenter')).toBeVisible();
  await expect.poll(() => counter(page), { timeout: 8000 }).toContain(`${s2.slide + 1} / ${n}`);
  await page.keyboard.press('ArrowRight');
  const s3 = await expected(page, ['next'], s2);
  await expect.poll(() => audState(aud)).toEqual(s3);
  // close the audience window: banner, reopen with the same state
  await aud.close();
  await expect(page.locator('.df-banner.df-on')).toBeVisible({ timeout: 8000 });
  const p2 = context.waitForEvent('page');
  await page.click('.df-banner button');
  const aud2 = await p2;
  await aud2.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => audState(aud2), { timeout: 10000 }).toEqual(s3);
  await expect(page.locator('.df-banner.df-on')).toBeHidden({ timeout: 8000 });
});
