// Decks generated from PowerPoint files (public feature decks in fixtures/pptx, no corpus needed).
import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, audState, counter, slideCount, expected, fileURL, OUT } from './helpers.mjs';

// Index of the slide the PowerPoint player shows.
const shown = (doc) => doc.evaluate(() => [...document.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on')));

test('PowerPoint solo: hidden slide skipped, keys, no network', async ({ page, context }) => {
  const w = watch(context);
  await solo(page, 'pptx-basic.html');
  const n = await slideCount(page);
  // the basic deck has four slides in the file, one of them hidden
  expect(n).toBe(3);
  expect(await page.locator('.df-pptx-slide').count()).toBe(3);
  expect(await page.evaluate(() => window.__deckfold.slides().map((s) => s.label))).not.toContain('This slide is hidden');
  const st = () => audState(page);
  await page.keyboard.press('PageDown');
  await expect.poll(st).toEqual(await expected(page, ['next']));
  await expect.poll(() => shown(page)).toBe(1);
  await page.keyboard.press('End');
  await expect.poll(() => shown(page)).toBe(n - 1);
  await page.keyboard.press('Home');
  await expect.poll(() => shown(page)).toBe(0);
  // Escape never ends anything inside the deck
  await page.keyboard.press('Escape');
  await page.waitForTimeout(300);
  expect(page.url()).toContain('pptx-basic.html');
  // fonts: every text element uses an embedded face that has loaded
  const fonts = await page.evaluate(async () => {
    await document.fonts.ready;
    const fams = new Set([...document.querySelectorAll('.df-pptx-slide text')].map((t) => t.getAttribute('font-family')));
    return [...fams].map((f) => [f, document.fonts.check('16px ' + f)]);
  });
  expect(fonts.length).toBeGreaterThan(0);
  for (const [fam, ok] of fonts) expect(ok, fam).toBe(true);
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('PowerPoint presenter view: previews, notes, overview', async ({ page, context }) => {
  const w = watch(context);
  const aud = await present(page, context, 'pptx-basic.html');
  await expect(page.locator('.df-presenter')).toBeVisible();
  await expect(page.locator('iframe[data-df-preview]')).toHaveCount(2);
  const n = await slideCount(page);
  await page.keyboard.press('ArrowRight');
  const s1 = await expected(page, ['next']);
  await expect.poll(() => audState(aud)).toEqual(s1);
  await expect.poll(() => shown(aud)).toBe(s1.slide);
  await expect.poll(() => counter(page)).toContain(`${s1.slide + 1} / ${n}`);
  const previews = () => page.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) =>
    [...f.contentDocument.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on'))));
  const nx = await expected(page, ['next'], s1);
  await expect.poll(previews, { timeout: 10000 }).toEqual([s1.slide, nx.slide]);
  // notes come from the notes slides of the PowerPoint file
  const note = await page.evaluate((i) => window.__deckfold.slides()[i].notes, s1.slide);
  expect(note).toBeTruthy();
  await expect(page.locator('.df-notes-text')).toHaveText(note);
  // overview: one tile per visible slide, each a clone of the rendered slide
  await page.click('.df-bar button[data-k="overview"]');
  await expect(page.locator('.df-tile')).toHaveCount(n);
  expect(await page.locator('.df-tile .df-clone svg').count()).toBe(n);
  await page.click('.df-tile[data-i="2"]');
  await expect.poll(() => audState(aud)).toEqual(await expected(page, [['goto', 2]]));
  await expect.poll(() => shown(aud)).toBe(2);
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('PowerPoint --refresh replaces the player', async ({ page }) => {
  const old = fs.readFileSync(path.join(OUT, 'pptx-old.html'), 'utf8');
  const refreshed = fs.readFileSync(path.join(OUT, 'pptx-refreshed.html'), 'utf8');
  expect(old).toContain('window.__dfOldPlayer=true;');
  expect(refreshed).not.toContain('window.__dfOldPlayer=true;');
  await solo(page, 'pptx-refreshed.html');
  await expect.poll(() => page.evaluate(() => !!window.__dfPptx)).toBe(true);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => shown(page)).toBe(1);
});

test('PowerPoint media: audio starts with its slide, stops on the next', async ({ page }) => {
  await solo(page, 'pptx-media.html');
  const audio = () => page.evaluate(() => { const a = document.querySelector('.df-pptx-slide audio'); return { paused: a.paused, t: a.currentTime }; });
  await expect.poll(async () => (await audio()).paused === false && (await audio()).t > 0, { timeout: 5000 }).toBe(true);
  await page.keyboard.press('ArrowRight');
  await expect.poll(async () => (await audio()).paused).toBe(true);
});

// Expected states per step come from the generator (fixtures/pptx/src/animations.spec.json).
const animSpec = JSON.parse(fs.readFileSync(path.join(OUT, '..', '..', 'fixtures', 'pptx', 'src', 'animations.spec.json'), 'utf8'));

const states = (doc, slide, targets) => doc.evaluate(([slide, targets]) => {
  const sec = document.querySelectorAll('.df-pptx-slide')[slide];
  const out = {};
  for (const t of targets) {
    const [id, p] = t.split(':p');
    let el = sec.querySelector(`g[data-sid="${id}"]`);
    if (p !== undefined) el = el.querySelector(`g[data-p="${p}"]`);
    const cs = getComputedStyle(el);
    const tr = el.style.transform || '';
    if (cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.01) out[t] = 'hidden';
    else if (/scale\((?!1,1\))/.test(tr)) out[t] = 'scaled';
    else if (/translate\(/.test(tr)) out[t] = 'moved';
    else out[t] = parseFloat(cs.opacity) > 0.99 ? 'visible' : 'partial';
  }
  return out;
}, [slide, targets]);

test('PowerPoint animations: every step by jump and by playing', async ({ page }) => {
  await solo(page, 'pptx-animations.html');
  for (const sp of animSpec) {
    const targets = Object.keys(sp.steps[0]);
    // jumps (previews, back, recovery): no animation, the state at once
    for (let k = 0; k < sp.steps.length; k++) {
      await page.evaluate(([s, k]) => window.__dfPptx.goTo(s, k), [sp.slide, k]);
      expect(await states(page, sp.slide, targets), `slide ${sp.slide + 1}, jump to step ${k}`).toEqual(sp.steps[k]);
    }
    // playing: next from step 0, each step played to its end
    await page.evaluate((s) => window.__dfPptx.goTo(s, 0), sp.slide);
    for (let k = 1; k < sp.steps.length; k++) {
      await page.evaluate(() => window.__dfPptx.next());
      await expect.poll(() => states(page, sp.slide, targets), { timeout: 4000 }).toEqual(sp.steps[k]);
    }
  }
});

test('PowerPoint animations: a fade is in progress halfway through', async ({ page }) => {
  await solo(page, 'pptx-animations.html');
  await page.evaluate(() => window.__dfPptx.goTo(0, 0));
  const id = Object.keys(animSpec[0].steps[0])[0];
  await page.evaluate(() => window.__dfPptx.next());
  await page.waitForTimeout(200);
  const op = await page.evaluate((id) => parseFloat(getComputedStyle(document.querySelector(`.df-pptx-slide g[data-sid="${id}"]`)).opacity), id);
  expect(op).toBeGreaterThan(0.05);
  expect(op).toBeLessThan(0.95);
});

test('PowerPoint presenter view: previews show the next step without animating', async ({ page, context }) => {
  const aud = await present(page, context, 'pptx-animations.html');
  const id = Object.keys(animSpec[0].steps[0])[0];
  const prevState = () => page.evaluate((id) => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) => {
    const el = f.contentDocument.querySelector(`.df-pptx-slide g[data-sid="${id}"]`);
    return el ? getComputedStyle(el).visibility : 'none';
  }), id);
  // current preview: step 0 (hidden), next preview: step 1 (visible)
  await expect.poll(prevState, { timeout: 10000 }).toEqual(['hidden', 'visible']);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(aud)).toEqual({ slide: 0, step: 1 });
});

test('PowerPoint auto mode: source timing (advTm) after the last step', async ({ page }) => {
  await solo(page, 'pptx-animations-auto.html');
  // wait for the last step of slide 3, then the slide advances after its 3 s, not after 1 s
  await page.waitForFunction(() => { const s = window.__deckfold.audience.state(); return s.slide === 2 && s.step === 3; }, null, { timeout: 30000 });
  const t0 = Date.now();
  await page.waitForFunction(() => window.__deckfold.audience.state().slide === 3, null, { timeout: 10000 });
  const waited = Date.now() - t0;
  expect(waited).toBeGreaterThan(2500);
  expect(waited).toBeLessThan(5000);
});
