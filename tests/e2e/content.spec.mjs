// The content model (docs/content-model.md) against what the source players show, and the search
// and outline built on it.
import { test, expect } from '@playwright/test';
import { solo, startScreen, present, audState, need, visibleSlide } from './helpers.mjs';

const content = (page) => page.evaluate(() => JSON.parse(document.getElementById('df-content').textContent));
const chars = (s) => s.replace(/\s+/g, '').length;

test('claude.ai content model: a block shows in the player from its step on', async ({ page }) => {
  need('claudeSteps');
  test.setTimeout(180_000);
  await solo(page, 'claude-steps.html');
  const model = await content(page);
  const slides = await page.evaluate(() => window.__deckfold.slides().map((s) => s.steps));
  expect(model.slides.length).toBe(slides.length);
  // the text the player shows on slide k: everything outside the elements it marks as not built yet
  const shownChars = (k) => page.evaluate((k) => {
    const sl = document.querySelectorAll('.deck-slide')[k];
    const w = document.createTreeWalker(sl, NodeFilter.SHOW_TEXT);
    let n = 0;
    for (let t = w.nextNode(); t; t = w.nextNode()) {
      const p = t.parentElement;
      if (p.closest('script,style,iframe,noscript,template,textarea,[hidden]') || p.closest('[data-build-hidden]')) continue;
      n += t.nodeValue.replace(/\s+/g, '').length;
    }
    return n;
  }, k);
  let checked = 0;
  for (let k = 0; k < slides.length; k++) {
    if (slides[k] < 2 && k > 0) continue; // slides with builds, and the first slide
    for (let s = 0; s < slides[k]; s++) {
      await page.evaluate(([k, s]) => window.__deckfold.audience.act('state', { slide: k, step: s }), [k, s]);
      await expect.poll(() => audState(page)).toEqual({ slide: k, step: s });
      await expect.poll(() => visibleSlide(page)).toBe(k);
      const want = model.slides[k].blocks.filter((b) => b.role !== 'alt' && b.step >= 0 && b.step <= s).reduce((a, b) => a + chars(b.text), 0);
      await expect.poll(() => shownChars(k), { message: `slide ${k + 1}, step ${s}`, timeout: 15000 }).toBe(want);
      checked++;
    }
  }
  expect(checked).toBeGreaterThan(2);
});

// ---------- search and outline (the overlay of DF.finder) ----------
const finderOpen = (page) => page.evaluate(() => { const f = document.querySelector('[data-df-finder]'); return !!f && f.classList.contains('df-on'); });
const items = (page) => page.evaluate(() => [...document.querySelectorAll('[data-df-finder] [data-df-find-item]')].map((b) => +b.getAttribute('data-i')));
const playerSlide = (page) => page.evaluate(() => [...document.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on')));
const chosen = (page) => page.evaluate(() => { const t = document.querySelector('.df-start .df-strip .df-tile.df-cur'); return t ? +t.getAttribute('data-i') : -1; });

test('start screen: outline of all slides, search in the slide text only, keys and choice', async ({ page }) => {
  await startScreen(page, 'pptx-content.html');
  const n = await page.evaluate(() => window.__deckfold.slides().length);
  expect(await finderOpen(page)).toBe(false);
  // the switch opens the outline: one entry per slide, the field has the focus
  await page.click('[data-df-find]');
  expect(await finderOpen(page)).toBe(true);
  expect(await items(page)).toEqual([...Array(n).keys()]);
  expect(await page.evaluate(() => document.activeElement.hasAttribute('data-df-find-input'))).toBe(true);
  // a slide with a title shows it; a slide without one is named by its number and starts its text
  const rows = await page.evaluate(() => [...document.querySelectorAll('[data-df-find-item]')].map((b) =>
    [b.querySelector('.df-find-title').textContent, (b.querySelector('.df-find-place') || {}).textContent || '']));
  expect(rows[1]).toEqual(['Lists and levels', '']);
  expect(rows[2][0]).toMatch(/ 3$/);
  expect(rows[2][1]).toContain('Free text on a slide');
  // typing filters: the list slide holds "entry" in four blocks; upper case and accents do not matter
  await page.keyboard.type('ENTRY');
  expect(await items(page)).toEqual([1]);
  expect(await page.getAttribute('[data-df-find-count]', 'data-df-find-count')).toBe('4');
  expect(await page.textContent('[data-df-find-item] mark')).toBe('entry');
  await page.fill('[data-df-find-input]', 'LÉVELS');
  expect(await items(page)).toEqual([1]);
  // the notes are not searched in this window (it may be on the projector)
  await page.fill('[data-df-find-input]', 'walrus');
  expect(await items(page)).toEqual([]);
  await expect(page.locator('[data-df-find-none]')).toHaveCount(1);
  // Escape closes the overlay and nothing else; the keys work again
  await page.keyboard.press('Escape');
  expect(await finderOpen(page)).toBe(false);
  await expect(page.locator('.df-start [data-df-start]')).toBeVisible();
  expect(await page.evaluate(() => document.activeElement === document.body || !document.activeElement.closest('[data-df-finder]'))).toBe(true);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => chosen(page)).toBe(1);
  // the bound key opens it; the arrows move in the list, not on the start screen; Enter chooses
  await page.keyboard.press('/');
  expect(await finderOpen(page)).toBe(true);
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  expect(await chosen(page)).toBe(1);
  await page.keyboard.press('Enter');
  expect(await finderOpen(page)).toBe(false);
  await expect.poll(() => chosen(page)).toBe(3);
  // Ctrl+F and Cmd+F open it too
  await page.keyboard.press('Control+f');
  expect(await finderOpen(page)).toBe(true);
  await page.keyboard.press('Escape');
  await page.keyboard.press('Meta+f');
  expect(await finderOpen(page)).toBe(true);
  // a click next to the box closes it
  await page.mouse.click(5, 5);
  expect(await finderOpen(page)).toBe(false);
});

test('presenter view: the search finds notes, keeps the keys to itself and goes to the slide chosen', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-content.html');
  await expect.poll(() => audState(page)).toEqual({ slide: 0, step: 0 });
  await pres.click('[data-df-find]');
  expect(await finderOpen(pres)).toBe(true);
  // a word that is only in the notes of the first slide: found here, and marked as notes
  await pres.keyboard.type('walrus');
  expect(await items(pres)).toEqual([0]);
  await expect(pres.locator('[data-df-find-item] [data-df-find-notes]')).toHaveCount(1);
  // while the overlay is open no key reaches the presentation: "b" is a letter, the arrows move
  // in the list
  await pres.fill('[data-df-find-input]', '');
  await pres.keyboard.type('b');
  await pres.keyboard.press('ArrowRight');
  await pres.keyboard.press('ArrowDown');
  await pres.keyboard.press('PageDown');
  await pres.waitForTimeout(400);
  expect(await page.evaluate(() => window.__deckfold.audience.info().blank)).toBe(null);
  expect(await audState(page)).toEqual({ slide: 0, step: 0 });
  // Escape closes the overlay; the presentation goes on
  await pres.keyboard.press('Escape');
  expect(await finderOpen(pres)).toBe(false);
  await pres.waitForTimeout(400);
  expect(await page.evaluate(() => !!window.__deckfold.audience && !document.querySelector('.df-start'))).toBe(true);
  await expect(pres.locator('.df-presenter.df-before')).toHaveCount(0);
  // afterwards the keys move the slides again
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 1 });
  // an entry goes to the start of its slide, like a tile of the overview
  await pres.keyboard.press('/');
  await pres.keyboard.type('hint only');
  expect(await items(pres)).toEqual([2]);
  await pres.keyboard.press('Enter');
  expect(await finderOpen(pres)).toBe(false);
  await expect.poll(() => audState(page)).toEqual({ slide: 2, step: 0 });
  await expect.poll(() => playerSlide(page)).toBe(2);
  // the presentation window itself has no search
  await expect(page.locator('[data-df-finder]')).toHaveCount(0);
  await page.keyboard.press('/');
  await page.keyboard.press('Control+f');
  await expect(page.locator('[data-df-finder]')).toHaveCount(0);
  expect(await audState(page)).toEqual({ slide: 2, step: 0 });
});

test('search: slide text with markup stays text in the list', async ({ page }) => {
  await startScreen(page, 'pptx-hostile.html');
  await page.keyboard.press('/');
  await page.keyboard.type('onerror');
  expect(await items(page)).toEqual([0]);
  expect(await page.textContent('[data-df-find-item] .df-find-place')).toContain('<img src=x onerror=');
  // the only elements in an entry are the ones deckfold builds
  expect(await page.evaluate(() => [...document.querySelectorAll('[data-df-finder] [data-df-find-item] *')].map((e) => e.tagName).filter((t) => t !== 'SPAN' && t !== 'MARK'))).toEqual([]);
  await page.fill('[data-df-find-input]', 'script');
  expect(await page.evaluate(() => document.querySelectorAll('[data-df-finder] script, [data-df-finder] img, [data-df-finder] b').length)).toBe(0);
});

test('search: a deck without a slide model has none', async ({ page }) => {
  await startScreen(page, 'generic.html');
  await expect(page.locator('[data-df-find]')).toHaveCount(0);
  await page.keyboard.press('/');
  await page.keyboard.press('Control+f');
  await expect(page.locator('[data-df-finder]')).toHaveCount(0);
});

test('search: the key from the configuration opens it, the default key no longer', async ({ page }) => {
  await startScreen(page, 'pptx-content-keys.html');
  await page.keyboard.press('/');
  expect(await finderOpen(page)).toBe(false);
  await page.keyboard.press('s');
  expect(await finderOpen(page)).toBe(true);
  // the key help shows the key as configured
  await page.keyboard.press('Escape');
  expect(await page.getAttribute('[data-df-keyhelp] [data-df-action="find"]', 'data-df-keys')).toBe('["s"]');
});

test('claude.ai: the outline names every slide by its label', async ({ page }) => {
  need('claude');
  await startScreen(page, 'claude.html');
  const labels = await page.evaluate(() => window.__deckfold.slides().map((s) => s.label));
  await page.click('[data-df-find]');
  expect(await page.evaluate(() => [...document.querySelectorAll('[data-df-find-item] .df-find-title')].map((e) => e.textContent))).toEqual(labels);
});

test('Keynote: the outline names the slides by number and starts their text', async ({ page }) => {
  need('keynote');
  await startScreen(page, 'keynote.html');
  const n = await page.evaluate(() => window.__deckfold.slides().length);
  await page.click('[data-df-find]');
  const rows = await page.evaluate(() => [...document.querySelectorAll('[data-df-find-item]')].map((b) =>
    [b.querySelector('.df-find-title').textContent, !!b.querySelector('.df-find-place')]));
  expect(rows.length).toBe(n);
  rows.forEach((r, i) => expect(r[0]).toMatch(new RegExp(` ${i + 1}$`)));
  const model = await content(page);
  expect(rows.map((r) => r[1])).toEqual(model.slides.map((s) => s.blocks.length > 0));
});
