// Decks generated from PowerPoint files (public feature decks in fixtures/pptx, no corpus needed).
import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, audState, hasWebGL, NO_WEBGL, counter, slideCount, expected, fileURL, OUT } from './helpers.mjs';

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
  const pres = await present(page, context, 'pptx-basic.html');
  await expect(pres.locator('.df-presenter')).toBeVisible();
  await expect(pres.locator('iframe[data-df-preview]')).toHaveCount(3); // current, audience, next
  const n = await slideCount(pres);
  await pres.keyboard.press('ArrowRight');
  const s1 = await expected(pres, ['next']);
  await expect.poll(() => audState(page)).toEqual(s1);
  await expect.poll(() => shown(page)).toBe(s1.slide);
  await expect.poll(() => counter(pres)).toContain(`${s1.slide + 1} / ${n}`);
  const previews = () => pres.evaluate(() => [...document.querySelectorAll('.df-current iframe[data-df-preview], .df-next iframe[data-df-preview]')].map((f) =>
    [...f.contentDocument.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on'))));
  // the current slide and the slide after it
  await expect.poll(previews, { timeout: 10000 }).toEqual([s1.slide, s1.slide + 1]);
  // notes come from the notes slides of the PowerPoint file
  const note = await pres.evaluate((i) => window.__deckfold.slides()[i].notes, s1.slide);
  expect(note).toBeTruthy();
  await expect(pres.locator('.df-notes-text')).toHaveText(note);
  // overview band: one tile per visible slide, each a clone of the rendered slide
  await expect(pres.locator('.df-overview .df-tile')).toHaveCount(n);
  await expect.poll(() => pres.locator('.df-overview .df-tile .df-clone svg').count()).toBe(n);
  await pres.click('.df-overview .df-tile[data-i="2"]');
  await expect.poll(() => audState(page)).toEqual(await expected(pres, [['goto', 2]]));
  await expect.poll(() => shown(page)).toBe(2);
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
  // record what play() returned and which media errors occurred, so that a failure names its cause
  await page.addInitScript(() => {
    window.__media = [];
    const top = window === window.top;
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      const p = play.apply(this, arguments);
      const rec = { top, tag: this.tagName, at: Math.round(performance.now()) };
      window.__media.push(rec);
      if (p && p.then) p.then(() => { rec.result = 'ok'; }, (e) => { rec.result = e.name + ': ' + e.message; });
      return p;
    };
    document.addEventListener('error', (e) => {
      const m = e.target;
      if (m instanceof HTMLMediaElement) window.__media.push({ top, error: m.error ? m.error.code + ' ' + m.error.message : '?' });
    }, true);
  });
  await solo(page, 'pptx-media.html');
  const audio = () => page.evaluate(() => {
    const a = document.querySelector('.df-pptx-slide audio');
    return { paused: a.paused, t: a.currentTime, readyState: a.readyState, networkState: a.networkState,
      error: a.error ? a.error.code + ' ' + a.error.message : null, src: (a.currentSrc || a.getAttribute('src') || '').slice(0, 5), plays: window.__media };
  });
  let st;
  for (const end = Date.now() + 5000; Date.now() < end; await page.waitForTimeout(100)) {
    st = await audio();
    if (!st.paused && st.t > 0) break;
  }
  expect(!st.paused && st.t > 0, JSON.stringify(st)).toBe(true);
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
      await page.evaluate(([s, k]) => window.__deckfold.audience.act('state', { slide: s, step: k }), [sp.slide, k]);
      expect(await states(page, sp.slide, targets), `slide ${sp.slide + 1}, jump to step ${k}`).toEqual(sp.steps[k]);
    }
    // playing: next from step 0, each step played to its end
    await page.evaluate((s) => window.__deckfold.audience.act('state', { slide: s, step: 0 }), sp.slide);
    for (let k = 1; k < sp.steps.length; k++) {
      await page.evaluate(() => window.__deckfold.audience.act('next'));
      await expect.poll(() => states(page, sp.slide, targets), { timeout: 4000 }).toEqual(sp.steps[k]);
    }
  }
});

test('PowerPoint animations: a fade is in progress halfway through', async ({ page }) => {
  await solo(page, 'pptx-animations.html');
  await page.evaluate(() => window.__deckfold.audience.act('state', { slide: 0, step: 0 }));
  const id = Object.keys(animSpec[0].steps[0])[0];
  await page.evaluate(() => window.__deckfold.audience.act('next'));
  await page.waitForTimeout(200);
  const op = await page.evaluate((id) => parseFloat(getComputedStyle(document.querySelector(`.df-pptx-slide g[data-sid="${id}"]`)).opacity), id);
  expect(op).toBeGreaterThan(0.05);
  expect(op).toBeLessThan(0.95);
});

test('PowerPoint presenter view: previews show the current and the next slide after all their steps', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-animations.html');
  const ends = animSpec.slice(0, 2).map((sp) => sp.steps.length - 1);
  const id = Object.keys(animSpec[0].steps[0])[0];
  const prevState = () => pres.evaluate((id) => [...document.querySelectorAll('.df-current iframe[data-df-preview], .df-next iframe[data-df-preview]')].map((f) => {
    const p = f.contentWindow.__dfPptx, el = f.contentDocument.querySelector(`.df-pptx-slide.df-on g[data-sid="${id}"]`);
    return [p ? p.state() : null, el ? getComputedStyle(el).visibility : 'none'];
  }), id);
  // the audience is at step 0 of slide 1; both previews show their slide after its last step
  await expect.poll(prevState, { timeout: 10000 }).toEqual([
    [{ slide: 0, step: ends[0] }, animSpec[0].steps[ends[0]][id]],
    [{ slide: 1, step: ends[1] }, animSpec[1].steps[ends[1]][id]]]);
  // a step within the slide does not change them
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 0, step: 1 });
  await pres.waitForTimeout(500);
  expect((await prevState()).map((p) => p[0])).toEqual([{ slide: 0, step: ends[0] }, { slide: 1, step: ends[1] }]);
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

test('PowerPoint GIF: starts again from its first frame when its slide is entered', async ({ page }) => {
  await solo(page, 'pptx-gifs.html');
  const img = page.locator('.df-pptx-slide.df-on image[data-df-pptx-gif]');
  await expect(img).toHaveCount(1);
  // The GIF shows red, green and blue for half a second each.
  const color = async () => {
    const box = await img.boundingBox();
    const { PNG } = await import('pngjs');
    const shot = PNG.sync.read(await page.screenshot({ clip: { x: box.x + box.width / 2, y: box.y + box.height / 2, width: 1, height: 1 } }));
    const [r, g, b] = shot.data;
    return r > 150 && g < 100 ? 'red' : g > 120 && r < 100 ? 'green' : b > 150 ? 'blue' : `${r},${g},${b}`;
  };
  await expect.poll(color, { timeout: 3000 }).toBe('green');
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => shown(page)).toBe(1);
  await page.waitForTimeout(400);
  await page.keyboard.press('ArrowLeft');
  await expect.poll(() => shown(page)).toBe(0);
  expect(await color()).toBe('red');
  expect(await img.getAttribute('href')).toMatch(/^blob:.*#df-run\d+$/);
});

test('PowerPoint trigger: a click on the trigger shape plays its next step', async ({ page }) => {
  await solo(page, 'pptx-triggers.html');
  const trig = await page.evaluate(() => window.__dfPptx.deck.slides[0].anim.triggers[0]);
  const star = page.locator(`.df-pptx-slide.df-on g[data-sid="${trig.steps[0].effects[0].target}"]`);
  const button = page.locator(`.df-pptx-slide.df-on g[data-sid="${trig.shape}"]`);
  const visible = () => star.evaluate((g) => getComputedStyle(g).visibility !== 'hidden' && parseFloat(getComputedStyle(g).opacity || '1') > 0.99);
  expect(await visible()).toBe(false);
  expect(await button.evaluate((g) => g.style.cursor)).toBe('pointer');
  await button.click();
  await expect.poll(visible).toBe(true);
  await button.click();
  await expect.poll(visible).toBe(false);
  // the sequence starts over after its last step
  await button.click();
  await expect.poll(visible).toBe(true);
  // the trigger does not change the slide or step
  expect(await page.evaluate(() => window.__dfPptx.state())).toEqual({ slide: 0, step: 0 });
  // leaving and coming back resets the slide
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowLeft');
  await expect.poll(() => shown(page)).toBe(0);
  expect(await visible()).toBe(false);
});

test('PowerPoint media: a click on the icon of an audio clip plays and pauses it', async ({ page }) => {
  await solo(page, 'pptx-triggers.html');
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => shown(page)).toBe(1);
  const audio = page.locator('.df-pptx-slide.df-on audio[data-df-pptx-media="click"]');
  const icon = page.locator('.df-pptx-slide.df-on g[data-sid]', { has: page.locator('audio[data-df-pptx-media]') });
  const paused = () => audio.evaluate((a) => a.paused);
  expect(await paused()).toBe(true);
  await icon.click();
  await expect.poll(paused).toBe(false);
  await icon.click();
  await expect.poll(paused).toBe(true);
});

// Mouse and wheel in the presentation (decision E15): a click on the slide moves on through
// deckfold, the wheel does nothing; "Click for full screen" never moves on. Clicks on triggers and
// media icons keep working (tests above).
test('PowerPoint pointer input: a click moves on, the wheel does nothing, the full-screen overlay does not move on', async ({ page }) => {
  await page.addInitScript(() => {
    Element.prototype.requestFullscreen = function () { return Promise.reject(new TypeError('refused')); };
    Object.defineProperty(window.screen, 'width', { get: () => 3000 });
    Object.defineProperty(window.screen, 'height', { get: () => 2000 });
  });
  await solo(page, 'pptx-basic.html');
  await expect(page.locator('.df-fs-overlay')).toBeVisible();
  // the overlay button: full screen is asked for again, the slide stays
  await page.click('[data-df-fs-overlay]');
  await page.waitForTimeout(400);
  expect(await audState(page)).toEqual({ slide: 0, step: 0 });
  expect(await shown(page)).toBe(0);
  // again without full screen, so that no overlay covers the slide
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await page.uncheck('[data-df-fs]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await page.waitForTimeout(300);
  await expect(page.locator('.df-fs-overlay')).toBeHidden();
  await page.mouse.click(200, 300);
  const s1 = await expected(page, ['next']);
  await expect.poll(() => audState(page)).toEqual(s1);
  await expect.poll(() => shown(page)).toBe(s1.slide);
  await page.mouse.move(400, 300);
  await page.mouse.wheel(0, 400);
  await page.waitForTimeout(600);
  expect(await audState(page)).toEqual(s1);
  expect(await shown(page)).toBe(s1.slide);
  expect(await page.evaluate(() => [scrollX, scrollY])).toEqual([0, 0]);
});

test('PowerPoint 3D: shapes are drawn with WebGL, the front keeps its fill color', async ({ page, context }) => {
  const w = watch(context);
  await solo(page, 'pptx-shapes3d.html');
  test.skip(!(await hasWebGL(page)), NO_WEBGL);
  const counts = async () => page.evaluate(() => ({
    all: document.querySelectorAll('[data-df-pptx-3d]').length,
    done: document.querySelectorAll('[data-df-pptx-3d-done=""]').length,
    images: document.querySelectorAll('image[data-df-pptx-3d-image]').length,
  }));
  // the shapes of a slide are drawn when the slide is shown
  for (let i = 0; i < 3; i++) await page.evaluate((i) => window.__deckfold.audience.act('state', { slide: i, step: 0 }), i);
  await expect.poll(counts).toEqual({ all: 11, done: 11, images: 11 });
  // a rectangle with a bevel, lit from the top: its middle has the fill color (70AD47)
  await page.evaluate(() => window.__deckfold.audience.act('state', { slide: 0, step: 0 }));
  const shape = page.locator('.df-pptx-slide.df-on g[data-sid]', { has: page.locator('path[data-df-pptx-3d]') }).nth(2);
  const box = await shape.boundingBox();
  const { PNG } = await import('pngjs');
  const shot = PNG.sync.read(await page.screenshot({ clip: { x: box.x + box.width * 0.75, y: box.y + box.height * 0.75, width: 1, height: 1 } }));
  const [r, g, b] = shot.data;
  expect(Math.abs(r - 0x70) + Math.abs(g - 0xad) + Math.abs(b - 0x47)).toBeLessThan(12);
  expect(realErrors(w)).toEqual([]);
});

test('PowerPoint 3D: without WebGL and in previews the SVG approximation stays', async ({ page, context }) => {
  await context.addInitScript(() => Object.defineProperty(window, '__dfThree', { get() { return undefined; }, set() {}, configurable: false }));
  await solo(page, 'pptx-shapes3d.html');
  for (let i = 0; i < 3; i++) await page.evaluate((i) => window.__deckfold.audience.act('state', { slide: i, step: 0 }), i);
  expect(await page.locator('image[data-df-pptx-3d-image]').count()).toBe(0);
  const filtered = await page.evaluate(() => [...document.querySelectorAll('path[data-df-pptx-3d]')].filter((p) => p.getAttribute('filter')).length);
  expect(filtered).toBeGreaterThanOrEqual(8);
});

test('PowerPoint 3D: previews in the presenter view keep the approximation', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-shapes3d.html');
  test.skip(!(await hasWebGL(page)), NO_WEBGL);
  await expect(pres.locator('iframe[data-df-preview]')).toHaveCount(3); // current, audience, next
  await expect.poll(() => page.evaluate(() => document.querySelectorAll('image[data-df-pptx-3d-image]').length)).toBeGreaterThan(0);
  await pres.waitForTimeout(500);
  const inPreviews = await pres.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')]
    .map((f) => f.contentDocument.querySelectorAll('image[data-df-pptx-3d-image]').length));
  expect(inPreviews).toEqual([0, 0, 0]);
});

test('PowerPoint builds by word and by letter: the units appear one after the other', async ({ page }) => {
  await solo(page, 'pptx-text2.html');
  await page.evaluate(() => window.__deckfold.audience.act('state', { slide: 1, step: 0 }));
  const vis = (sel) => page.evaluate((sel) => [...document.querySelectorAll(`.df-pptx-slide.df-on ${sel}`)]
    .map((g) => getComputedStyle(g).visibility !== 'hidden' && parseFloat(getComputedStyle(g).opacity || '1') > 0.5), sel);
  const words = await vis('g[data-w]'), letters = await vis('g[data-c]');
  expect(words.length).toBe(6);
  expect(letters.length).toBe('Andthisoneletterbyletter'.length);
  expect(words.concat(letters).every((v) => !v)).toBe(true);
  await page.keyboard.press('ArrowRight');
  // each word starts 10 percent of the effect (500 ms) after the one before: 6 words take 1 s,
  // so there is a moment when the first word is visible and the last is not
  await expect.poll(async () => { const m = await vis('g[data-w]'); return m[0] && !m[5]; }, { timeout: 2000, intervals: [20] }).toBe(true);
  await expect.poll(() => vis('g[data-w]')).toEqual([true, true, true, true, true, true]);
  // a jump shows the end state of every unit
  await page.evaluate(() => window.__deckfold.audience.act('state', { slide: 1, step: 2 }));
  expect((await vis('g[data-c]')).every((v) => v)).toBe(true);
});

test('PowerPoint morph: paired objects move from their old to their new place', async ({ page }) => {
  await solo(page, 'pptx-morph.html');
  expect(await page.evaluate(() => window.__dfPptx.deck.slides[1].anim.transition.type)).toBe('morph');
  // the circle is paired by its "!!" name: x from 120 to 820, width from 160 to 320 (slide px)
  const ball = () => page.evaluate(() => {
    const s = document.querySelectorAll('.df-pptx-slide')[1];
    const g = [...s.querySelectorAll('g[data-mk]')].find((g) => g.querySelector('ellipse, path'));
    const r = g.getBoundingClientRect(), st = document.getElementById('df-pptx-stage').getBoundingClientRect();
    const k = st.width / window.__dfPptx.deck.width;
    return { x: (r.left - st.left) / k, w: r.width / k, moving: g.hasAttribute('transform') };
  });
  await page.keyboard.press('ArrowRight');
  // during the transition (2 s) the circle is somewhere between its two places and sizes (with
  // SwiftShader, as on the CI runners, the frames come slowly)
  await expect.poll(async () => {
    const m = await ball();
    return m.moving && m.x > 125 && m.x < 815 && m.w > 161 && m.w < 319;
  }, { timeout: 2500, intervals: [20] }).toBe(true);
  await expect.poll(async () => (await ball()).moving, { timeout: 3000 }).toBe(false);
  expect(Math.abs((await ball()).x - 820)).toBeLessThan(2);
});

test('PowerPoint transitions: every 2D, 3D and tile transition plays and ends on the next slide', async ({ page, context }) => {
  test.setTimeout(300_000);
  const w = watch(context);
  await solo(page, 'pptx-transitions3d.html');
  const types = await page.evaluate(() => window.__dfPptx.deck.slides.map((s) => (s.anim.transition || {}).type));
  expect(types.slice(1).every((t) => t && t !== 'fade')).toBe(true);
  const starts = [];
  for (let i = 1; i < types.length; i++) {
    await page.evaluate((i) => window.__deckfold.audience.act('state', { slide: i - 1, step: 0 }), i);
    // something moves while the transition runs (1.5 s); the time until it starts is measured in
    // the page, so that a failure tells a late start from none (tile transitions first draw both
    // slides into pictures). The steps go through deckfold, which keeps the player on its state.
    const ms = await page.evaluate(async () => {
      const moving = () => !!document.querySelector('#df-pptx-stage > canvas') ||
        [...document.querySelectorAll('.df-pptx-slide')].some((s) => s.style.transform || s.style.clipPath || s.style.opacity || s.style.filter);
      await new Promise((r) => setTimeout(r, 0));
      const t0 = performance.now();
      window.__deckfold.audience.act('next');
      while (!moving() && performance.now() - t0 < 20000) await new Promise((r) => setTimeout(r, 10));
      return moving() ? Math.round(performance.now() - t0) : -1;
    });
    starts.push(`${types[i]} ${ms} ms`);
    expect(ms >= 0 && ms < 6000, `${types[i]}: moving after ${ms} ms (-1: not within 20 s); all: ${starts.join(', ')}`).toBe(true);
    await expect.poll(() => shown(page), { timeout: 10000, message: types[i] }).toBe(i);
    await expect.poll(() => page.evaluate(() => !document.querySelector('#df-pptx-stage > canvas') &&
      [...document.querySelectorAll('.df-pptx-slide')].every((s) => !s.style.transform && !s.style.clipPath && !s.style.opacity)), { timeout: 10000, message: types[i] }).toBe(true);
  }
  console.log('transitions start moving after:', starts.join(', '));
  expect(realErrors(w)).toEqual([]);
});

test('PowerPoint 3D charts: the plots are drawn with WebGL', async ({ page, context }) => {
  const w = watch(context);
  await solo(page, 'pptx-charts2.html');
  test.skip(!(await hasWebGL(page)), NO_WEBGL);
  const n = await page.evaluate(() => window.__dfPptx.deck.slides.length);
  for (let i = 0; i < n; i++) await page.evaluate((i) => window.__deckfold.audience.act('state', { slide: i, step: 0 }), i);
  // five 3D charts in the generated deck; PowerPoint saves the surface chart as a 3D one too
  await expect.poll(() => page.evaluate(() => {
    const all = document.querySelectorAll('[data-df-pptx-chart3d]').length;
    return all >= 5 && document.querySelectorAll('[data-df-pptx-chart3d][data-df-pptx-3d-done=""]').length === all;
  })).toBe(true);
  expect(realErrors(w)).toEqual([]);
});

// Player and deckfold kept together (decision E15): a player that went elsewhere is brought back.
test('PowerPoint: a player moved past deckfold is brought back to deckfold\'s state', async ({ page }) => {
  await solo(page, 'pptx-basic.html');
  await page.waitForTimeout(1500);
  const s = await audState(page);
  await page.evaluate(() => window.__dfPptx.goTo(2, 0));
  expect(await shown(page)).toBe(2);
  await expect.poll(() => shown(page), { timeout: 3000 }).toBe(s.slide);
  expect(await page.evaluate(() => window.__dfPptx.state())).toEqual(s);
  expect(await audState(page)).toEqual(s);
});
