import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, startScreen, audState, counter, slideCount, expected, need, fileURL } from './helpers.mjs';

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
  const pres = await present(page, context, 'claude.html');
  await expect(pres.locator('.df-presenter')).toBeVisible();
  await expect(pres.locator('iframe[data-df-preview]')).toHaveCount(2);
  const n = await slideCount(pres);
  for (let i = 0; i < 3; i++) await pres.keyboard.press('ArrowRight');
  const s3 = await expected(pres, ['next', 'next', 'next']);
  await expect.poll(() => audState(page)).toEqual(s3);
  await expect.poll(() => counter(pres)).toContain(`${s3.slide + 1} / ${n}`);
  // previews: slide shown by the player in the current and the next preview (the slide after it)
  const nx = { slide: s3.slide + 1 };
  const visible = () => pres.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) =>
    [...f.contentDocument.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off'))));
  await expect.poll(visible, { timeout: 10000 }).toEqual([s3.slide, nx.slide]);
  await expect.poll(() => page.evaluate(() => [...document.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off')))).toBe(s3.slide);
  const note = await pres.evaluate((i) => window.__deckfold.slides()[i].notes || window.__deckfold.t('noNotes'), s3.slide);
  await expect(pres.locator('.df-notes-text')).toHaveText(note);
  // a key in the show window works too
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => counter(pres)).toContain(`${nx.slide + 1} / ${n}`);
  // overview band: a click jumps
  await expect(pres.locator('.df-overview .df-tile')).toHaveCount(n);
  await pres.click('.df-overview .df-tile[data-i="1"]');
  await expect.poll(() => audState(page)).toEqual(await expected(pres, [['goto', 1]]));
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('settings of the presenter view change while it runs and survive a reload', async ({ page, context }) => {
  const pres = await present(page, context, 'claude.html');
  await pres.click('[data-df-el="notes"]');
  await expect(pres.locator('.df-notes')).toBeHidden();
  await pres.click('[data-df-el="notes"]');
  await pres.click('[data-df-font="+"]');
  await pres.click('[data-df-font="+"]');
  await expect.poll(() => pres.evaluate(() => getComputedStyle(document.querySelector('.df-notes-text')).fontSize)).toBe('26px');
  await pres.click('[data-df-el="time"]');
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  const s = await pres.evaluate(() => window.__deckfold.settings());
  expect(s.presenter.elements).not.toContain('time');
  expect(s.presenter.notesFontSize).toBe(26);
  // only the differences are stored
  const stored = await pres.evaluate(() => Object.keys(localStorage).filter((k) => k.startsWith('deckfold:')).map((k) => JSON.parse(localStorage.getItem(k))));
  expect(stored.some((v) => v.presenter && v.presenter.notesFontSize === 26 && !('auto' in v))).toBe(true);
});

test('recovery: presenter view reloaded, show window reloaded, presenter view closed', async ({ page, context }) => {
  const pres = await present(page, context, 'claude.html');
  const n = await slideCount(pres);
  await pres.keyboard.press('ArrowRight'); await pres.keyboard.press('ArrowRight');
  const s2 = await expected(pres, ['next', 'next']);
  await expect.poll(() => audState(page)).toEqual(s2);
  // reload the presenter view: state and connection come back through the heartbeat
  await pres.reload();
  await expect(pres.locator('.df-presenter')).toBeVisible();
  await expect.poll(() => counter(pres), { timeout: 8000 }).toContain(`${s2.slide + 1} / ${n}`);
  await pres.keyboard.press('ArrowRight');
  const s3 = await expected(pres, ['next'], s2);
  await expect.poll(() => audState(page)).toEqual(s3);
  // reload the show window: start screen with the last slide chosen, the presenter view reconnects
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('[data-df-presenter]')).toBeChecked({ timeout: 8000 });
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', String(s3.slide));
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible();
  // close the presenter view: the switch goes off
  await pres.close();
  await expect(page.locator('[data-df-presenter]')).not.toBeChecked({ timeout: 8000 });
});

test('start slide: three starts in a row each begin at the chosen slide', async ({ page }) => {
  await startScreen(page, 'claude.html');
  // the start slide is the point here; full screen would add a window change to every start
  await page.uncheck('[data-df-fs]');
  const visible = () => page.evaluate(() => [...document.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off')));
  const n = await slideCount(page);
  for (const pick of [Math.min(3, n - 1), 1, Math.min(5, n - 1)]) {
    await page.waitForSelector('[data-df-start]');
    await page.click(`.df-strip .df-tile[data-i="${pick}"]`);
    await page.click('[data-df-start]');
    await page.waitForFunction(() => window.__deckfold.audience && !document.querySelector('.df-start'));
    await expect.poll(visible, { timeout: 5000 }).toBe(pick);
    await page.keyboard.press('Escape');
  }
  // let the reload after the last end finish before the test closes the browser context
  await page.waitForSelector('[data-df-start]');
});
