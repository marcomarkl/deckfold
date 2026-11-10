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
  const pres = await present(page, context, 'keynote.html');
  await expect(pres.locator('.df-notes')).toBeHidden();
  const canvases = () => pres.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) => f.contentDocument.querySelectorAll('#stage canvas').length));
  await expect.poll(async () => (await canvases()).every((c) => c > 0), { timeout: 20000 }).toBe(true);
  await pres.waitForTimeout(1000);
  const n = await slideCount(pres);
  for (let i = 0; i < 3; i++) { await pres.keyboard.press('ArrowRight'); await pres.waitForTimeout(700); }
  const s3 = await expected(pres, ['next', 'next', 'next']);
  await expect.poll(() => audState(page)).toEqual(s3);
  await expect.poll(() => counter(pres)).toContain(`${s3.slide + 1} / ${n}`);
  // overview band: one tile per slide; the thumbnails of the source load as the tiles become visible
  await expect(pres.locator('.df-overview .df-tile')).toHaveCount(n);
  await expect.poll(() => pres.locator('.df-overview .df-tile img').count()).toBeGreaterThan(0);
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

// An export with links to other slides (the content deck of fixtures/pptx, exported by Keynote).
// The player draws on a canvas, so the test clicks at the place of a link: its rectangle in the
// deck metadata, mapped to the stage element.
test('Keynote links to other slides: followed through deckfold, the presenter view in step', async ({ page, context }) => {
  need('keynoteLinks');
  test.setTimeout(180_000);
  const pres = await present(page, context, 'keynote-links.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 15000 });
  // the slide the player shows: the index of its own slide change event
  await page.evaluate(() => document.addEventListener('ShowController:SlideIndexDidChangeEvent', (e) => { window.__knAt = e.detail.slideIndex; }));
  const n = await slideCount(page);
  const all = await page.evaluate(() => JSON.parse(document.getElementById('df-meta').textContent).deck.extra.links);
  const L = all.findIndex((l) => l && l.length);
  expect(L).toBeGreaterThan(0);
  const links = all[L].filter((l) => l.step === 0);
  const point = (x, y) => page.evaluate(([x, y]) => {
    const r = document.getElementById('stage').getBoundingClientRect();
    const k = r.width / JSON.parse(document.getElementById('df-meta').textContent).deck.width;
    return [r.left + x * k, r.top + y * k];
  }, [x, y]);
  const clickLink = async (to) => {
    const l = links.find((l) => l.to === to);
    const p = await point(l.x + l.w / 2, l.y + l.h / 2);
    await page.mouse.click(p[0], p[1]);
  };
  const go = (k) => page.evaluate((k) => window.__deckfold.audience.act('state', { slide: k, step: 0 }), k);
  const at = async (k) => {
    await expect.poll(() => audState(page)).toEqual({ slide: k, step: 0 });
    await expect.poll(() => page.evaluate(() => window.__knAt), { timeout: 10000 }).toBe(k);
    await expect.poll(() => counter(pres)).toContain(`${k + 1} / ${n}`);
  };
  const targets = { next: L + 1, prev: L - 1, first: 0, last: n - 1 };
  const kinds = [...new Set(links.map((l) => l.to))];
  expect(kinds.filter((t) => t in targets).length).toBeGreaterThan(0);
  for (const to of kinds) {
    const target = to in targets ? targets[to] : (/^\d+$/.test(to) ? +to : null);
    if (target === null || target === L) continue;
    await go(L); await at(L);
    await clickLink(to);
    await at(target);
    // deckfold and the player stay there: nothing pulls one of them elsewhere afterwards
    await page.waitForTimeout(1500);
    await at(target);
  }
  // "the slide viewed before"
  if (kinds.includes('back')) {
    const from = L > 1 ? 1 : n - 1;
    await go(from); await at(from);
    await go(L); await at(L);
    await clickLink('back');
    await at(from);
  }
  // a click next to every link moves on like any other click
  await go(L); await at(L);
  let free = null;
  for (let y = 20; y < 520 && !free; y += 25) for (let x = 20; x < 940 && !free; x += 40) {
    if (!links.some((l) => x >= l.x - 5 && x <= l.x + l.w + 5 && y >= l.y - 5 && y <= l.y + l.h + 5)) free = [x, y];
  }
  const p = await point(free[0], free[1]);
  await page.mouse.click(p[0], p[1]);
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next'], { slide: L, step: 0 }));
  // "end show" ends the presentation: back at the start screen
  if (kinds.includes('end')) {
    await go(L); await at(L);
    await clickLink('end');
    await page.waitForSelector('.df-start [data-df-start]');
  }
});

// deckfold jumps in the Keynote player by typing the slide number and Enter. The player shows its
// own slide number display while it takes the digits; the audience never sees it.
test('Keynote: the player\'s slide number display stays hidden in the presentation', async ({ page }) => {
  await solo(page, 'keynote.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 15000 });
  const n = await slideCount(page);
  // watch the display while deckfold jumps: once per animation frame, whether it can be seen
  await page.evaluate(() => {
    window.__hud = { frames: 0, seen: 0, found: false };
    const tick = () => {
      window.__hud.frames++;
      for (const id of ['slideNumberControl', 'slideNumberDisplay']) {
        const el = document.getElementById(id);
        if (!el) continue;
        window.__hud.found = true;
        const cs = getComputedStyle(el), r = el.getBoundingClientRect();
        if (cs.display !== 'none' && cs.visibility !== 'hidden' && parseFloat(cs.opacity) > 0.01 && r.width > 0 && r.height > 0) window.__hud.seen++;
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
  for (const k of [Math.min(2, n - 1), 0, n - 1]) {
    await page.evaluate((k) => window.__deckfold.audience.act('state', { slide: k, step: 0 }), k);
    await expect.poll(() => audState(page)).toEqual({ slide: k, step: 0 });
    await page.waitForTimeout(900);
  }
  const hud = await page.evaluate(() => window.__hud);
  expect(hud.frames).toBeGreaterThan(20);
  // the player has the display (otherwise this test checks nothing), and no frame showed it
  expect(hud.found).toBe(true);
  expect(hud.seen).toBe(0);
});
