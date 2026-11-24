import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test, expect } from '@playwright/test';

const here = path.dirname(fileURLToPath(import.meta.url));
export const OUT = path.resolve(here, '..', '.out');
export const ROOT = path.resolve(here, '..', '..');
export const fileURL = (name) => 'file://' + path.join(OUT, name);

// Decks found by setup.mjs ({ claude, keynote }, paths relative to the repository).
export const corpus = (() => {
  try { return JSON.parse(fs.readFileSync(path.join(OUT, 'corpus.json'), 'utf8')); } catch { return {}; }
})();

// Skips the current test when there is no deck of this kind.
export function need(kind) {
  test.skip(!corpus[kind], `no ${kind} deck in fixtures/ or in the local corpus testdata/`);
}

// Watches network requests and errors on all pages of a context.
export function watch(context) {
  const net = [], errors = [];
  const hook = (p) => {
    p.on('pageerror', (e) => errors.push(e.message));
    p.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
  };
  context.pages().forEach(hook);
  context.on('page', hook);
  context.on('request', (r) => { if (!/^(file|blob|data|about):/.test(r.url())) net.push(r.url()); });
  return { net, errors };
}
export const realErrors = (w) => w.errors.filter((e) => !/requestFullscreen|user gesture/i.test(e));

// Opens the start screen of the show window (the window the file is opened in).
export async function startScreen(page, name) {
  await page.goto(fileURL(name));
  await page.waitForSelector('.df-start [data-df-start]');
}

// Starts the presentation without the presenter view; the page is the show window.
export async function solo(page, name) {
  await startScreen(page, name);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
}

// Switches the presenter view on and starts; the page is the show window, the returned popup the
// presenter view.
export async function present(page, context, name) {
  await startScreen(page, name);
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  return pres;
}

// Switches the presenter view on from the start screen; returns its window.
export async function openPresenter(page, context) {
  const popupP = context.waitForEvent('page');
  await page.check('[data-df-presenter]');
  const pres = await popupP;
  await pres.waitForSelector('.df-presenter');
  return pres;
}

// Whether the page's browser can create a WebGL context (the Firefox of the Linux CI cannot).
export const hasWebGL = (page) => page.evaluate(() => {
  const c = document.createElement('canvas');
  return !!(c.getContext('webgl2') || c.getContext('webgl'));
});
export const NO_WEBGL = 'no WebGL in this browser; the fallback is checked by the transitions test';

export const audState = (show) => show.evaluate(() => window.__deckfold.audience.state());
// the slide the presenter view shows, as "<slide> / <slides>" (data-df-at; the view shows no counter)
export const counter = (page) => page.getAttribute('.df-presenter', 'data-df-at');
export const slideCount = (page) => page.evaluate(() => window.__deckfold.slides().length);
// the number of slides the show shows: without the ones hidden from it (E21)
export const shownCount = (page) => page.evaluate(() => window.__deckfold.slides().length - window.__deckfold.hiddenSlides().length);
// the index of a slide the show shows, the k-th of them
export const shownSlide = (page, k) => page.evaluate((k) => window.__deckfold.slides().map((s, i) => i).filter((i) => window.__deckfold.isShown(i))[k], k);

// Expected state after a sequence of model operations, computed with the runtime's own model
// from the deck metadata (so the tests do not depend on a particular deck).
// ops: e.g. ['next', 'next', 'prev'] or [['goto', 6]]; starts from the first state unless from is given.
export function expected(page, ops, from) {
  return page.evaluate(([ops, from]) => {
    const m = window.__deckfold.model;
    let s = from || m.first();
    for (const op of ops) {
      const [name, arg] = Array.isArray(op) ? op : [op];
      s = name === 'first' || name === 'last' ? m[name]() : name === 'goto' ? m.goto(arg) : m[name](s);
    }
    return s;
  }, [ops, from || null]);
}

// Index of the slide the claude.ai player currently shows (non-current slides carry data-off).
export const visibleSlide = (doc) => doc.evaluate(() => [...document.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off')));

// assetState reads what became of the pictures and fonts of a PowerPoint deck (decision E17):
// the MIME types of the embedded assets, whether every picture on the slides decodes, and the
// state of every font face.
export const assetState = (page) => page.evaluate(async () => {
  const mimes = [...document.querySelectorAll('script[id^="df-a-"][data-mime]')].map((s) => s.getAttribute('data-mime'));
  const pictures = await Promise.all([...document.querySelectorAll('.df-pptx-slide image')].map(async (im) => {
    const probe = new Image();
    probe.src = (im.getAttribute('href') || '').replace(/#.*$/, '');
    let error = '';
    try { await probe.decode(); } catch (e) { error = String(e); }
    return { width: probe.naturalWidth, error };
  }));
  await document.fonts.ready;
  return { mimes, pictures, fonts: [...document.fonts].map((f) => f.family + ':' + f.status) };
});

// The colour in the middle of an element, as a name where it is one of the three colours of the
// animated test picture.
export async function middleColor(page, locator) {
  const box = await locator.boundingBox();
  const { PNG } = await import('pngjs');
  const shot = PNG.sync.read(await page.screenshot({ clip: { x: box.x + box.width / 2, y: box.y + box.height / 2, width: 1, height: 1 } }));
  const [r, g, b] = shot.data;
  return r > 150 && g < 100 ? 'red' : g > 120 && r < 100 ? 'green' : b > 150 ? 'blue' : `${r},${g},${b}`;
}

// The Base64 text of an asset block is emptied once the asset is decoded, and its URLs come from
// the kept Blob (10-loader.js). Starts `name` without the presenter view and checks: at least one
// asset was decoded and its text emptied; every asset, decoded by the show or now by url(), gives
// through url() and two fresh() URLs the bytes its Base64 held; has() stays true; and after url()
// its text is empty too. Returns the number of assets and of those emptied by the start.
export async function checkFreedBase64(page, name) {
  // the decoded size of every asset, read from the file: the start screen decodes some at once
  const sizes = {};
  const html = fs.readFileSync(path.join(OUT, name), 'latin1');
  for (const m of html.matchAll(/<script type="application\/octet-stream" id="df-a-([0-9a-f]+)-\d+"[^>]*>([^<]*)<\/script>/g)) {
    sizes[m[1]] = (sizes[m[1]] || 0) + Buffer.from(m[2], 'base64').length;
  }
  await startScreen(page, name);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  return page.evaluate(async (sizes) => {
    const a = window.__deckfold.assets;
    const empty = (h) => [...document.querySelectorAll('script[type="application/octet-stream"][id^="df-a-' + h + '-"]')].every((b) => b.textContent === '');
    const emptied = Object.keys(sizes).filter(empty);
    const bad = [];
    for (const [h, size] of Object.entries(sizes)) {
      const urls = [a.url(h), a.fresh(h), a.fresh(h)];
      for (const u of urls) {
        const n = (await (await fetch(u)).arrayBuffer()).byteLength;
        if (n !== size) bad.push(h + ': ' + n + ' bytes, want ' + size);
      }
      if (!a.has(h)) bad.push(h + ': has() is false');
      if (!empty(h)) bad.push(h + ': text kept after url()');
    }
    return { assets: Object.keys(sizes).length, emptied: emptied.length, bad };
  }, sizes);
}

// ---------- touch and the slide strip of the presentation (decision E22) ----------

// A touch along the given points ([x, y], ...) in Chrome, through CDP: the last point is where the
// finger lifts. opts.cancel ends it with touchCancel; opts.second adds a second finger at the start.
export async function touchPath(page, pts, opts = {}) {
  const cdp = await page.context().newCDPSession(page);
  const at = ([x, y]) => ({ x, y });
  const first = [at(pts[0])].concat(opts.second ? [at(opts.second)] : []);
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: first });
  for (const p of pts.slice(1)) {
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [at(p)].concat(opts.second ? [at(opts.second)] : []) });
  }
  await cdp.send('Input.dispatchTouchEvent', { type: opts.cancel ? 'touchCancel' : 'touchEnd', touchPoints: [] });
  await cdp.detach();
}
// A straight swipe from one point to another, in four moves.
export const swipe = (page, from, to, opts) => touchPath(page, [0, 1, 2, 3, 4].map((k) => [from[0] + (to[0] - from[0]) * k / 4, from[1] + (to[1] - from[1]) * k / 4]), opts);

export const stripOpen = (page) => page.evaluate(() => { const s = document.querySelector('[data-df-showstrip]'); return !!s && s.classList.contains('df-on'); });
// the slide the strip marks
export const stripMark = (page) => page.evaluate(() => { const t = document.querySelector('[data-df-showstrip] .df-tile.df-cur'); return t ? +t.getAttribute('data-i') : null; });

// Starts `name` without full screen and without the presenter view.
export async function soloWindowed(page, name) {
  await startScreen(page, name);
  await page.uncheck('[data-df-fs]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
}

// The slide strip with the key and the mouse, on a deck with a slide hidden from the show:
// the key shows and closes it, the keys of "next" and "back" move its mark and not the slides,
// Enter goes to the marked slide, Escape closes it without ending the show, a click beside it
// closes it and moves nothing, a hidden slide is greyed out and can be chosen, and its button
// ends the show. Runs in every browser.
export async function checkStripKeys(page, name) {
  await soloWindowed(page, name);
  const st = () => audState(page);
  const n = await slideCount(page);
  const hidden = await page.evaluate(() => window.__deckfold.hiddenSlides());
  expect(hidden.length, 'the deck needs a slide hidden from the show').toBeGreaterThan(0);
  const s0 = await st();
  await page.keyboard.press('g');
  await expect.poll(() => stripOpen(page)).toBe(true);
  expect(await page.locator('[data-df-showstrip] .df-tile').count()).toBe(n);
  expect(await page.evaluate(() => [...document.querySelectorAll('[data-df-showstrip] .df-tile.df-off')].map((t) => +t.getAttribute('data-i')))).toEqual(hidden);
  expect(await stripMark(page)).toBe(s0.slide);
  // the keys move the mark, not the slides
  await page.keyboard.press('ArrowRight');
  expect(await stripMark(page)).toBe(s0.slide + 1);
  await page.keyboard.press('PageUp');
  expect(await stripMark(page)).toBe(s0.slide);
  await page.keyboard.press('ArrowRight');
  expect(await st()).toEqual(s0);
  await page.keyboard.press('Enter');
  const s1 = await expected(page, [['goto', s0.slide + 1]]);
  await expect.poll(st).toEqual(s1);
  expect(await stripOpen(page)).toBe(false);
  // the key again closes it; Escape closes it and does not end the show
  await page.keyboard.press('g');
  await expect.poll(() => stripOpen(page)).toBe(true);
  await page.keyboard.press('G');
  expect(await stripOpen(page)).toBe(false);
  await page.keyboard.press('g');
  await page.keyboard.press('Escape');
  expect(await stripOpen(page)).toBe(false);
  await page.waitForTimeout(300);
  expect(await page.locator('[data-df-start]').count()).toBe(0);
  expect(await st()).toEqual(s1);
  // a click beside it closes it and moves nothing
  await page.keyboard.press('g');
  await page.mouse.click(600, 200);
  expect(await stripOpen(page)).toBe(false);
  await page.waitForTimeout(300);
  expect(await st()).toEqual(s1);
  // a hidden slide can be chosen; the show goes on with the next shown slide
  await page.keyboard.press('g');
  await page.locator(`[data-df-showstrip] .df-tile[data-i="${hidden[0]}"]`).click();
  const sh = await expected(page, [['goto', hidden[0]]]);
  await expect.poll(st).toEqual(sh);
  expect(await stripOpen(page)).toBe(false);
  await page.keyboard.press('ArrowRight');
  await expect.poll(st).toEqual(await expected(page, ['next'], sh));
  // its button ends the show
  await page.keyboard.press('g');
  await page.locator('[data-df-showstrip] [data-df-end]').click();
  await page.waitForSelector('[data-df-start]');
  // after the strip was closed with Escape, the next Escape ends the show
  await page.uncheck('[data-df-fs]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await page.keyboard.press('g');
  await expect.poll(() => stripOpen(page)).toBe(true);
  await page.keyboard.press('Escape');
  expect(await stripOpen(page)).toBe(false);
  await page.keyboard.press('Escape');
  await page.waitForSelector('[data-df-start]');
}

// Swipes in the presentation (Chrome with hasTouch): to the left is "next", to the right "back",
// both through deckfold, so the player shows what deckfold's state says (shownFn reads the slide the
// player shows). What is no swipe moves nothing; a tap still moves on once. A swipe up shows the
// strip, also above the black screen and the end slide; a swipe down or a tap beside it closes it.
export async function checkSwipes(page, name, shownFn) {
  await soloWindowed(page, name);
  await page.waitForTimeout(600);
  const st = () => audState(page);
  const same = async (s, what) => { await page.waitForTimeout(500); expect(await st(), what).toEqual(s); };
  const y = 300;
  await swipe(page, [900, y], [500, y]);
  const s1 = await expected(page, ['next']);
  await expect.poll(st).toEqual(s1);
  await expect.poll(() => shownFn(page)).toBe(s1.slide);
  await swipe(page, [900, y], [500, y]);
  const s2 = await expected(page, ['next'], s1);
  await expect.poll(st).toEqual(s2);
  await expect.poll(() => shownFn(page)).toBe(s2.slide);
  await swipe(page, [500, y], [900, y]);
  await expect.poll(st).toEqual(s1);
  await expect.poll(() => shownFn(page)).toBe(s1.slide);
  // no swipe: too short across, slanted, two fingers, cancelled
  await swipe(page, [700, y], [730, y + 20]);
  // (the short one is a tap for the browser or nothing; it must not count twice)
  const afterShort = await st();
  expect([JSON.stringify(s1), JSON.stringify(s2)]).toContain(JSON.stringify(afterShort));
  await swipe(page, [500, 200], [700, 400]);
  await same(afterShort, 'a slanted move');
  await swipe(page, [900, y], [500, y], { second: [400, 500] });
  await same(afterShort, 'a move with two fingers');
  await swipe(page, [900, y], [500, y], { cancel: true });
  await same(afterShort, 'a cancelled touch');
  // a tap moves on once
  await page.touchscreen.tap(700, y);
  const s3 = await expected(page, ['next'], afterShort);
  await expect.poll(st).toEqual(s3);
  await same(s3, 'after one tap');
  // nothing scrolled
  expect(await page.evaluate(() => [scrollX, scrollY])).toEqual([0, 0]);
  // up: the strip; a swipe to the side moves nothing while it is open; down closes it
  await swipe(page, [700, 500], [700, 250]);
  await expect.poll(() => stripOpen(page)).toBe(true);
  expect(await stripMark(page)).toBe(s3.slide);
  await swipe(page, [900, y], [500, y]);
  await same(s3, 'a swipe while the strip is open');
  expect(await stripOpen(page)).toBe(true);
  // a touch that begins on the strip belongs to it
  const band = await page.locator('[data-df-showstrip] [data-df-strip]').boundingBox();
  await swipe(page, [band.x + band.width - 40, band.y + band.height / 2], [band.x + 40, band.y + band.height / 2]);
  await same(s3, 'a swipe on the strip');
  expect(await stripOpen(page)).toBe(true);
  await swipe(page, [700, 150], [700, 400]);
  await expect.poll(() => stripOpen(page)).toBe(false);
  await same(s3, 'after the strip was closed');
  // a tap beside the open strip closes it and moves nothing
  await swipe(page, [700, 500], [700, 250]);
  await expect.poll(() => stripOpen(page)).toBe(true);
  await page.touchscreen.tap(700, 200);
  await expect.poll(() => stripOpen(page)).toBe(false);
  await same(s3, 'a tap beside the strip');
  // a tap on a tile goes to the start of its slide
  await swipe(page, [700, 500], [700, 250]);
  await expect.poll(() => stripOpen(page)).toBe(true);
  await page.locator('[data-df-showstrip] .df-tile[data-i="0"]').tap();
  const first = await expected(page, [['goto', 0]]);
  await expect.poll(st).toEqual(first);
  await expect.poll(() => shownFn(page)).toBe(0);
  expect(await stripOpen(page)).toBe(false);
  // above the black screen
  await page.keyboard.press('b');
  await expect(page.locator('.df-blank.df-black')).toBeVisible();
  await swipe(page, [700, 500], [700, 250]);
  await expect.poll(() => stripOpen(page)).toBe(true);
  const top = () => page.evaluate(() => {
    const r = document.querySelector('[data-df-showstrip]').getBoundingClientRect();
    const el = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return !!(el && el.closest('[data-df-showstrip]'));
  });
  expect(await top(), 'the strip is above the black screen').toBe(true);
  await swipe(page, [700, 150], [700, 400]);
  await expect.poll(() => stripOpen(page)).toBe(false);
  await page.keyboard.press('b');
  // on the end slide: the strip, and its button ends the show
  await page.evaluate(() => { const d = window.__deckfold; d.audience.act('last'); });
  const lastSteps = await page.evaluate(() => { const d = window.__deckfold; const l = d.model.last(); return d.slides()[l.slide].steps; });
  for (let i = 0; i < lastSteps + 1 && !(await st()).ended; i++) { await page.evaluate(() => window.__deckfold.audience.act('next')); await page.waitForTimeout(150); }
  expect((await st()).ended).toBe(true);
  await swipe(page, [700, 500], [700, 250]);
  await expect.poll(() => stripOpen(page)).toBe(true);
  expect(await top(), 'the strip is above the end slide').toBe(true);
  await page.locator('[data-df-showstrip] [data-df-end]').tap();
  await page.waitForSelector('[data-df-start]');
}

// The tiles of the strip beside the running player (E22). The tiles are copies of the slides from
// the document before activation. This reads every tile on the start screen, starts the show, goes
// into the deck, opens the strip and fills every tile: each shows what it showed on the start
// screen; the copies are not counted as slides (slideSel finds as many as the deck has); and with
// the strip open the whole show still runs step by step, the player showing the slide deckfold's
// state names and the runtime still able to read it.
export async function checkStripTiles(page, name, slideSel, shownFn) {
  const fill = (root) => page.evaluate(async (root) => {
    const tiles = [...document.querySelectorAll(root + ' .df-tile')];
    for (const t of tiles) {
      t.scrollIntoView({ block: 'nearest', inline: 'center' });
      const box = t.querySelector('.df-thumb');
      for (let k = 0; k < 100 && !box.firstElementChild; k++) await new Promise((r) => setTimeout(r, 30));
    }
    return tiles.map((t) => {
      const box = t.querySelector('.df-thumb'), img = box.querySelector('img'), c = box.querySelector('.df-clone');
      // a slide copied whole loses its slide class in the show: compare what is inside it
      return img ? 'img:' + img.getAttribute('src') : c ? c.firstElementChild.innerHTML : '';
    });
  }, root);
  await startScreen(page, name);
  const before = await fill('.df-start');
  expect(before.every((t) => t !== ''), 'every tile of the start screen is filled').toBe(true);
  await page.uncheck('[data-df-fs]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await page.waitForTimeout(800);
  const n = await slideCount(page);
  // into the deck: the slide with the most steps, half way through them
  const mid = await page.evaluate(() => {
    const s = window.__deckfold.slides();
    let at = 0;
    s.forEach((x, i) => { if (window.__deckfold.isShown(i) && x.steps > s[at].steps) at = i; });
    return { slide: at, step: Math.floor((s[at].steps - 1) / 2) };
  });
  await page.evaluate((mid) => window.__deckfold.audience.act('state', mid), mid);
  await expect.poll(() => shownFn(page)).toBe(mid.slide);
  await page.waitForTimeout(600);
  await page.evaluate(() => window.__deckfold.audience.act('strip'));
  await expect.poll(() => stripOpen(page)).toBe(true);
  const during = await fill('[data-df-showstrip]');
  for (let i = 0; i < n; i++) expect(during[i] === before[i], `tile ${i + 1} in the show is the tile of the start screen`).toBe(true);
  if (slideSel) expect(await page.locator(slideSel).count(), 'elements found as slides').toBe(n);
  // the whole show with the strip open and filled
  let s = await expected(page, ['first']);
  await page.evaluate((s) => window.__deckfold.audience.act('state', s), s);
  for (let k = 0; k < 400; k++) {
    await expect.poll(() => audState(page)).toEqual(s);
    await expect.poll(() => shownFn(page), `the player on step ${k}`).toBe(s.slide);
    expect(await page.evaluate(() => { const a = window.__deckfold.adapter(); return a.shown ? a.shown() : 0; }), 'what the runtime reads from the player').toBe(s.slide);
    const next = await page.evaluate((s) => window.__deckfold.model.next(s), s);
    if (!next) break;
    s = next;
    await page.evaluate(() => window.__deckfold.audience.act('next'));
  }
  expect(await stripOpen(page)).toBe(true);
}
