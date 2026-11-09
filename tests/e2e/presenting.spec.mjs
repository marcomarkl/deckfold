// Presenting (decision E14): the window the file is opened in is the presentation, with a start
// screen first; the presenter view is a second window that the user drags to a screen. Where the
// windows are is not checked; full screen can always be chosen.
import { test, expect } from '@playwright/test';
import { startScreen, openPresenter, solo, present, audState, fileURL, need, visibleSlide, expected } from './helpers.mjs';

const deferred = (page) => page.evaluate(() => document.querySelectorAll('script[type="text/df-deferred"]').length);

test('start screen: title, preview, strip; a tile sets the start slide; the player runs only after Start', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await expect(page.locator('.df-start h1')).not.toBeEmpty();
  await expect(page.locator('.df-start-stage iframe[data-df-preview]')).toHaveCount(1);
  const n = await page.evaluate(() => window.__deckfold.slides().length);
  await expect(page.locator('.df-strip .df-tile')).toHaveCount(n);
  // before "Start" no source script has run
  expect(await deferred(page)).toBeGreaterThan(0);
  expect(await page.evaluate(() => !!window.__dfPptx)).toBe(false);
  await page.click('.df-strip .df-tile[data-i="2"]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await expect.poll(() => audState(page)).toEqual({ slide: 2, step: 0 });
  await expect(page.locator('.df-start')).toHaveCount(0);
  expect(await deferred(page)).toBe(0);
  // the end returns to the start screen by reloading: the player is inactive again, the last slide is chosen
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  expect(await deferred(page)).toBeGreaterThan(0);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
});

test('start screen keys: arrows choose the slide, Enter starts', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('Enter');
  await page.waitForFunction(() => window.__deckfold.audience);
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
});

test('presenter view: opens without a position, follows the start slide, keys before the start', async ({ page, context }) => {
  await page.addInitScript(() => {
    const open = window.open;
    window.__opens = [];
    window.open = function (u, n, f) { window.__opens.push([n.split(':')[0], f]); return open.call(window, u, n, f); };
  });
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  const opens = await page.evaluate(() => window.__opens);
  expect(opens.length).toBe(1);
  expect(opens[0][0]).toBe('df-presenter');
  expect(opens[0][1]).not.toMatch(/left|top|fullscreen/);
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible();
  // a key in the presenter view chooses the start slide too; Enter does not start from there
  await pres.keyboard.press('ArrowRight');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  await expect.poll(() => pres.textContent('.df-counter')).toContain('2 / ');
  await pres.keyboard.press('Enter');
  await expect(pres.locator('.df-banner')).toHaveAttribute('data-df-banner', 'startInShow');
  expect(await page.evaluate(() => !!window.__deckfold.audience)).toBe(false);
  // switching the presenter view off closes it
  await page.uncheck('[data-df-presenter]');
  await expect.poll(() => pres.isClosed()).toBe(true);
});

test('where the windows are is not checked: no screen permission, no hint, full screen can always be chosen', async ({ page, context }) => {
  await context.addInitScript(() => {
    window.__asked = [];
    window.getScreenDetails = () => { window.__asked.push('getScreenDetails'); return Promise.reject(new Error('not used')); };
    const query = navigator.permissions.query.bind(navigator.permissions);
    navigator.permissions.query = (d) => { window.__asked.push(d && d.name); return query(d); };
  });
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.waitForTimeout(1500);
  await expect(page.locator('[data-df-fs]')).toBeEnabled();
  await expect(page.locator('[data-df-hint].df-on')).toHaveCount(0);
  await expect(pres.locator('.df-banner')).toHaveAttribute('data-df-banner', '');
  const asked = [...await page.evaluate(() => window.__asked), ...await pres.evaluate(() => window.__asked)];
  expect(asked.filter((a) => a === 'getScreenDetails' || a === 'window-management')).toEqual([]);
});

test('end: Escape in the presenter view ends; the presenter view stays open and shows the start state', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  await pres.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  await expect(page.locator('[data-df-presenter]')).toBeChecked({ timeout: 5000 });
  expect(pres.isClosed()).toBe(false);
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible({ timeout: 5000 });
  // before the start Escape in the presenter view does nothing
  await pres.keyboard.press('Escape');
  await page.waitForTimeout(500);
  await expect(page.locator('.df-start [data-df-start]')).toBeVisible();
});

test('overview band of the presenter view: click, arrows, switch', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  // before the start: a click and the arrows choose the start slide
  await pres.click('.df-overview .df-tile[data-i="2"]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await pres.click('.df-overview [data-df-strip-prev]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  // during the show: a click jumps, the arrows only scroll the band
  await pres.click('.df-overview .df-tile[data-i="2"]');
  await expect.poll(() => audState(page)).toEqual({ slide: 2, step: 0 });
  await pres.click('.df-overview [data-df-strip-prev]');
  await page.waitForTimeout(400);
  expect(await audState(page)).toEqual({ slide: 2, step: 0 });
  // the status bar holds the switches; "overview" hides the band, also after a reload
  await expect(pres.locator('.df-status [data-df-el]')).toHaveCount(6);
  await pres.click('.df-status [data-df-el="overview"]');
  await expect(pres.locator('.df-overview')).toBeHidden();
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  await expect(pres.locator('.df-overview')).toBeHidden();
});

test('back on the first slide does nothing; after the last slide the end slide appears and stays', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  await page.keyboard.press('ArrowLeft');
  await page.waitForTimeout(300);
  expect(await audState(page)).toEqual({ slide: 0, step: 0 });
  await expect(page.locator('.df-endslide.df-on')).toHaveCount(0);
  await page.keyboard.press('End');
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('.df-endslide.df-on')).toHaveCount(1);
  // more steps, in the presentation, by click and from the presenter view: the end slide stays
  await page.keyboard.press('ArrowRight');
  await page.mouse.click(600, 400);
  await pres.keyboard.press('ArrowRight');
  await page.waitForTimeout(800);
  await expect(page.locator('.df-endslide.df-on')).toHaveCount(1);
  expect(await page.locator('.df-start').count()).toBe(0);
  expect((await page.evaluate(() => window.__deckfold.audience.info())).ended).toBe(true);
  // back shows the last slide again; Escape still ends
  await page.keyboard.press('ArrowLeft');
  await expect(page.locator('.df-endslide.df-on')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
});

test('start screen: arrows at the strip, auto fields only with auto mode, no hash before the start', async ({ page }) => {
  await page.goto(fileURL('pptx-basic.html') + '#2');
  await page.waitForSelector('.df-start [data-df-start]');
  // a slide number in the address would override the chosen start slide in some players
  expect(await page.evaluate(() => location.hash)).toBe('');
  await expect(page.locator('[data-df-strip-prev]')).toBeDisabled();
  await page.click('[data-df-strip-next]');
  await page.click('[data-df-strip-next]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await expect(page.locator('[data-df-strip-next]')).toBeDisabled();
  await page.uncheck('[data-df-auto]');
  await expect(page.locator('[data-df-interval]')).toBeDisabled();
  await expect(page.locator('[data-df-loop]')).toBeDisabled();
  await page.check('[data-df-auto]');
  await expect(page.locator('[data-df-interval]')).toBeEnabled();
  await expect(page.locator('[data-df-keyhelp] kbd')).not.toHaveCount(0);
});

test('Escape in full screen ends the presentation (Keyboard Lock)', async ({ page }) => {
  await page.addInitScript(() => {
    window.__fs = false;
    window.__locks = [];
    Object.defineProperty(Document.prototype, 'fullscreenElement', { configurable: true, get() { return window.__fs ? document.documentElement : null; } });
    Element.prototype.requestFullscreen = function () { window.__fs = true; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
    Document.prototype.exitFullscreen = function () { window.__fs = false; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
    Object.defineProperty(navigator, 'keyboard', { configurable: true, value: { lock(k) { window.__locks.push(k); return Promise.resolve(); }, unlock() { window.__locks.push('unlock'); } } });
  });
  await solo(page, 'pptx-basic.html');
  expect(await page.evaluate(() => !!document.fullscreenElement)).toBe(true);
  expect(await page.evaluate(() => window.__locks)).toEqual([['Escape']]);
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
});

test('full-screen fallback when full screen fails', async ({ page }) => {
  await page.addInitScript(() => {
    Element.prototype.requestFullscreen = function () { return Promise.reject(new TypeError('refused')); };
    Object.defineProperty(window.screen, 'width', { get: () => 3000 });
    Object.defineProperty(window.screen, 'height', { get: () => 2000 });
  });
  await solo(page, 'pptx-basic.html');
  await expect(page.locator('.df-fs-overlay')).toBeVisible();
  // without full screen chosen on the start screen, no overlay
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await page.uncheck('[data-df-fs]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await page.waitForTimeout(1200);
  await expect(page.locator('.df-fs-overlay')).toBeHidden();
});

test('reloading the presenter view during the show reconnects it', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  await expect.poll(() => pres.textContent('.df-counter'), { timeout: 8000 }).toMatch(/^2 \//);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 2, step: 0 });
});

test('start screen: tiles of equal width; the preview shows the chosen slide after all its steps', async ({ page }) => {
  await startScreen(page, 'pptx-animations.html');
  const widths = await page.evaluate(() => [...document.querySelectorAll('.df-start .df-strip .df-tile')].map((t) => Math.round(t.getBoundingClientRect().width)));
  expect(new Set(widths).size).toBe(1);
  const steps = await page.evaluate(() => window.__deckfold.slides()[0].steps);
  expect(steps).toBeGreaterThan(1);
  const preview = () => page.evaluate(() => {
    const f = document.querySelector('.df-start-stage iframe'); // created right after the first frame
    return f && f.contentWindow.__dfPptx ? f.contentWindow.__dfPptx.state() : null;
  });
  await expect.poll(preview, { timeout: 10000 }).toEqual({ slide: 0, step: steps - 1 });
});

// ---------- previews, reload, full-screen button (found in a test with a real deck) ----------

const previewFrames = (pres) => pres.locator('iframe[data-df-preview]');
// [visible slide, number of hidden builds on it, running animations] in the previews "Current" and "Next"
// of the presenter view (sel: other previews, such as the audience pane)
const previewInfo = (pres, sel = '.df-current iframe[data-df-preview], .df-next iframe[data-df-preview]') => pres.evaluate((sel) => [...document.querySelectorAll(sel)].map((f) => {
  const d = f.contentDocument, i = [...d.querySelectorAll('.deck-slide')].findIndex((s) => !s.hasAttribute('data-off'));
  const sl = d.querySelectorAll('.deck-slide')[i];
  return [i, sl ? sl.querySelectorAll('[data-build-hidden]').length : -1, d.getAnimations().filter((a) => a.playState === 'running').length];
}), sel);
// first slide with at least three steps and builds that hide content at its first step
const multiStepSlide = (page) => page.evaluate(() => window.__deckfold.slides().findIndex((s, i, all) => s.steps >= 3 && i + 1 < all.length && !(s.auto || []).some(Boolean)));

test('claude.ai presenter view: a narrow pane shows only the current slide', async ({ page, context }) => {
  need('claude');
  const pres = await present(page, context, 'claude.html');
  await pres.setViewportSize({ width: 420, height: 900 });
  await pres.keyboard.press('ArrowRight');
  // wait for the step to arrive before reading which slide the presentation shows
  const s1 = await expected(page, ['next']);
  await expect.poll(() => audState(page)).toEqual(s1);
  await expect.poll(() => visibleSlide(page)).toBe(s1.slide);
  const shown = s1.slide;
  // the player switches to a scroll view with all slides in a viewport that is not wider than high
  await expect.poll(() => pres.evaluate(() => document.querySelector('iframe[data-df-preview]').contentWindow.matchMedia('(max-aspect-ratio: 1/1)').matches)).toBe(false);
  await expect.poll(async () => (await previewInfo(pres))[0][0], { timeout: 10000 }).toBe(shown);
});

test('claude.ai presenter view: both previews show their slide after all its steps, without animating', async ({ page, context }) => {
  need('claudeSteps');
  const pres = await present(page, context, 'claude-steps.html');
  const k = await multiStepSlide(page);
  expect(k).toBeGreaterThan(-1);
  await pres.click(`.df-overview .df-tile[data-i="${k}"]`);
  await expect.poll(() => visibleSlide(page)).toBe(k);
  await page.waitForTimeout(1500);
  const hiddenInShow = () => page.evaluate((k) => document.querySelectorAll('.deck-slide')[k].querySelectorAll('[data-build-hidden]').length, k);
  const atStart = await hiddenInShow();
  // while the slides change, no preview animates
  let running = 0;
  for (let t = 0; t < 15; t++) { running += (await previewInfo(pres)).reduce((a, p) => a + p[2], 0); await pres.waitForTimeout(100); }
  expect(running).toBe(0);
  await expect.poll(async () => (await previewInfo(pres)).map((p) => p[0]), { timeout: 10000 }).toEqual([k, k + 1]);
  await pres.waitForTimeout(1500);
  const inPreview = (await previewInfo(pres))[0][1];
  // the audience at the last step of the slide shows the same builds as the current preview
  const steps = await page.evaluate((k) => window.__deckfold.slides()[k].steps, k);
  for (let i = 1; i < steps; i++) await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: k, step: steps - 1 });
  await page.waitForTimeout(1500);
  const atEnd = await hiddenInShow();
  expect(atStart).not.toBe(atEnd); // the slide has builds, so step 0 and the last step differ
  expect(inPreview).toBe(atEnd);
});

test('reload: the start screen chooses the slide chosen or shown last', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await page.click('.df-strip .df-tile[data-i="2"]');
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  // shown last: the show reached slide 2 before the reload
  await page.click('.df-strip .df-tile[data-i="0"]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  // chosen after that show: the choice wins
  await page.click('.df-strip .df-tile[data-i="2"]');
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
});

test('presenter view: the full-screen button names what a click does', async ({ page, context }) => {
  await context.addInitScript(() => {
    window.__fs = false;
    Object.defineProperty(Document.prototype, 'fullscreenElement', { configurable: true, get() { return window.__fs ? document.documentElement : null; } });
    Element.prototype.requestFullscreen = function () { window.__fs = true; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
    Document.prototype.exitFullscreen = function () { window.__fs = false; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
  });
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  const label = (k) => pres.evaluate((k) => window.__deckfold.t(k), k);
  const btn = pres.locator('.df-presenter [data-df-fs]');
  await expect(btn).toHaveText(await label('fullscreen'));
  await btn.click();
  await expect(btn).toHaveText(await label('window'));
  await btn.click();
  await expect(btn).toHaveText(await label('fullscreen'));
});

// fake full screen with Keyboard Lock for all windows of a context
const fakeFullscreen = () => {
  window.__fs = false;
  window.__locks = [];
  Object.defineProperty(Document.prototype, 'fullscreenElement', { configurable: true, get() { return window.__fs ? document.documentElement : null; } });
  Element.prototype.requestFullscreen = function () { window.__fs = true; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
  Document.prototype.exitFullscreen = function () { window.__fs = false; document.dispatchEvent(new Event('fullscreenchange')); return Promise.resolve(); };
  Object.defineProperty(navigator, 'keyboard', { configurable: true, value: { lock(k) { window.__locks.push(k); return Promise.resolve(); }, unlock() { window.__locks.push('unlock'); } } });
};

test('presenter view in full screen: one Escape ends the presentation', async ({ page, context }) => {
  await context.addInitScript(fakeFullscreen);
  await startScreen(page, 'pptx-basic.html');
  await page.uncheck('[data-df-fs]');
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  await pres.click('.df-presenter [data-df-fs]');
  expect(await pres.evaluate(() => !!document.fullscreenElement)).toBe(true);
  expect(await pres.evaluate(() => window.__locks)).toEqual([['Escape']]);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  await pres.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible({ timeout: 5000 });
});

// time in the presenter view: tested with real time in short spans (a fake clock would let the
// heartbeat of the show window look lost and hold the commands back)
const timeOf = (pres) => pres.textContent('.df-elapsed');
const secs = (t) => { const p = t.replace('−', '').split(':').map(Number); return p.reduce((a, x) => a * 60 + x, 0); };

test('time: runs from Start; pause, resume and reset; target and time left', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  // before the start the time stands still
  await pres.waitForTimeout(1500);
  expect(await timeOf(pres)).toBe('00:00');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(async () => secs(await timeOf(pres)), { timeout: 5000 }).toBeGreaterThanOrEqual(2);
  // pause: the time stands still; resume: it runs on
  await pres.click('.df-time-group button:has-text("❚❚")');
  const held = await timeOf(pres);
  await pres.waitForTimeout(2000);
  expect(await timeOf(pres)).toBe(held);
  await pres.click('.df-time-group button:has-text("▶")');
  await expect.poll(async () => secs(await timeOf(pres)), { timeout: 4000 }).toBeGreaterThan(secs(held));
  // reset by button and by key
  await pres.click('.df-time-group button:has-text("↺")');
  expect(secs(await timeOf(pres))).toBeLessThanOrEqual(1);
  await pres.waitForTimeout(2200);
  await pres.keyboard.press('t');
  expect(secs(await timeOf(pres))).toBeLessThanOrEqual(1);
  // target 0:05: time left shows, with a warning, and turns into overtime
  await pres.fill('[data-df-target]', '0:5');
  await pres.press('[data-df-target]', 'Enter');
  await expect(pres.locator('[data-df-target]')).toHaveValue('00:05');
  await expect(pres.locator('.df-remain')).toBeVisible();
  await expect(pres.locator('.df-remain')).toHaveClass(/df-warn/);
  await expect(pres.locator('.df-remain')).toHaveClass(/df-over/, { timeout: 8000 });
});

test('target duration: typed as minutes:seconds and completed', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  const parse = (v) => pres.evaluate((v) => { const t = window.__deckfold.parseMinSec(v); return t === null ? null : window.__deckfold.fmtMinSec(t); }, v);
  const table = { '20': '20:00', '5:3': '05:03', '1:75': '02:15', ':30': '00:30', '0': '00:00', '': '00:00', '700': '600:00', '599:99': '600:00', 'abc': null, '-5': null, '1:2:3': null };
  for (const [k, v] of Object.entries(table)) expect(await parse(k), k).toBe(v);
  const box = pres.locator('[data-df-target]');
  await box.fill('5:3');
  await box.press('Enter');
  await expect(box).toHaveValue('05:03');
  // input it cannot read brings back the value before
  await box.fill('abc');
  await box.press('Enter');
  await expect(box).toHaveValue('05:03');
  // empty: no target
  await box.fill('');
  await box.press('Enter');
  await expect(box).toHaveValue('');
  await expect(pres.locator('.df-remain')).toBeHidden();
  // the value is kept in seconds and survives a reload of the presenter view
  await box.fill('5:3');
  await box.press('Enter');
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  await expect(pres.locator('[data-df-target]')).toHaveValue('05:03');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => pres.textContent('.df-remain'), { timeout: 5000 }).toMatch(/^0[45]:0\d$/);
});

test('claude.ai auto steps: moving on while they play keeps the player and deckfold together', async ({ page, context }) => {
  need('claudeSteps');
  const pres = await present(page, context, 'claude-steps.html');
  // a slide whose steps after the first all run by themselves, with a slide after it
  const k = await page.evaluate(() => window.__deckfold.slides().findIndex((s, i, all) => s.auto && s.auto.slice(1).every(Boolean) && s.steps > 2 && i + 1 < all.length));
  expect(k).toBeGreaterThan(0);
  // reach slide k from the slide before, then move on at once, while its auto run plays
  await pres.click(`.df-overview .df-tile[data-i="${k - 1}"]`);
  await expect.poll(() => visibleSlide(page)).toBe(k - 1);
  const steps = await page.evaluate((i) => window.__deckfold.slides()[i].steps, k - 1);
  for (let i = 1; i < steps; i++) { await pres.keyboard.press('ArrowRight'); await page.waitForTimeout(400); }
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => visibleSlide(page)).toBe(k);
  await page.waitForTimeout(300);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(pres, [['goto', k + 1]]));
  await expect.poll(() => visibleSlide(page)).toBe(k + 1);
  await page.waitForTimeout(3000);
  expect(await visibleSlide(page)).toBe(k + 1);
  // back from there and once more: the slide before k + 1 is k with all its steps, then k - 1
  await pres.keyboard.press('ArrowLeft');
  await expect.poll(() => visibleSlide(page)).toBe(k);
  await pres.keyboard.press('ArrowLeft');
  await expect.poll(() => visibleSlide(page)).toBe(k - 1);
  await page.waitForTimeout(3000);
  expect(await visibleSlide(page)).toBe(k - 1);
  expect((await audState(page)).slide).toBe(k - 1);
});

test('claude.ai auto runs: "next" moves to the next slide at once, by key, click and from the presenter view', async ({ page, context }) => {
  need('claudeSteps');
  const pres = await present(page, context, 'claude-steps.html');
  // slides with an auto run and a slide after them, from the metadata
  const runs = await page.evaluate(() => window.__deckfold.slides().map((s, i, all) => (s.auto && s.auto.some(Boolean) && i + 1 < all.length ? i : -1)).filter((i) => i >= 0));
  expect(runs.length).toBeGreaterThan(0);
  for (const k of runs) {
    for (const [how, delay] of [['key', 50], ['click', 300], ['presenter', 1000], ['key', 1000], ['click', 50], ['presenter', 300]]) {
      await page.evaluate((k) => window.__deckfold.audience.act('goto', k), k);
      await expect.poll(() => visibleSlide(page)).toBe(k);
      await page.waitForTimeout(delay);
      if (how === 'key') await page.keyboard.press('ArrowRight');
      else if (how === 'click') await page.mouse.click(900, 400);
      else await pres.keyboard.press('ArrowRight');
      await expect.poll(() => audState(page), { message: `${how} after ${delay} ms on slide ${k}` }).toEqual(await expected(page, [['goto', k + 1]]));
      await expect.poll(() => visibleSlide(page), { message: `${how} after ${delay} ms on slide ${k}` }).toBe(k + 1);
    }
  }
});

test('Enter: starts on the start screen, does nothing in the presentation and in the presenter view', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.bringToFront();
  await page.keyboard.press('Enter');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  await page.waitForTimeout(500);
  const s = await audState(page);
  await page.keyboard.press('Enter');
  await page.waitForTimeout(600);
  expect(await audState(page)).toEqual(s);
  // in the presenter view a switch that was clicked keeps the focus; Enter must not press it
  const notes = pres.locator('.df-status [data-df-el="notes"]');
  const pressed = () => notes.getAttribute('aria-pressed');
  await notes.click();
  const before = await pressed();
  await pres.keyboard.press('Enter');
  await pres.waitForTimeout(600);
  expect(await pressed()).toBe(before);
  expect(await audState(page)).toEqual(s);
  await notes.click(); // back on for later tests
});

test('start screen: the key help lists every bound key, grouped, and the fixed keys', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  const help = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-df-keyhelp] [data-df-action]')];
    return { groups: document.querySelectorAll('[data-df-keyhelp] [data-df-keygroup]').length,
      rows: rows.map((r) => [r.getAttribute('data-df-action'), JSON.parse(r.getAttribute('data-df-keys'))]),
      keys: window.__deckfold.settings().keys };
  });
  expect(help.groups).toBeGreaterThanOrEqual(4);
  // every key of every action is shown in a row of that action, and no row shows a key the action does not have
  for (const [act, keys] of Object.entries(help.keys)) {
    const shown = help.rows.filter(([a]) => a === act).flatMap(([, k]) => k);
    expect(shown.length, act).toBeGreaterThan(0);
    for (const k of keys) expect(shown, `${act}: ${JSON.stringify(k)}`).toContain(k);
    for (const k of shown) expect(keys, `${act}: ${JSON.stringify(k)}`).toContain(k);
  }
  // the fixed keys: Enter starts, digits + Enter choose a slide, a click moves on
  const acts = help.rows.map(([a]) => a);
  for (const a of ['start', 'goto', 'click']) expect(acts).toContain(a);
});

// Every frame a preview paints, recorded inside the preview: the slide the player shows, its hidden
// builds and whether the preview is visible at all (the iframe is hidden until it shows its first
// state).
const recordPreviewFrames = () => {
  let fe = null;
  try { fe = window.frameElement; } catch (e) { }
  if (!fe || !fe.hasAttribute('data-df-preview')) return;
  window.__frames = [];
  const loop = () => {
    const all = [...document.querySelectorAll('.deck-slide')];
    const on = all.filter((s) => !s.hasAttribute('data-off'));
    const i = on.length === 1 ? all.indexOf(on[0]) : -1;
    const vis = getComputedStyle(fe).visibility !== 'hidden' && getComputedStyle(fe.parentElement).visibility !== 'hidden';
    const k = [i, i >= 0 ? on[0].querySelectorAll('[data-build-hidden]').length : -1, vis];
    const l = window.__frames[window.__frames.length - 1];
    if (!l || l.join() !== k.join()) window.__frames.push(k);
    requestAnimationFrame(loop);
  };
  requestAnimationFrame(loop);
};
const paintedFrames = (doc, sel) => doc.evaluate((sel) => [...document.querySelectorAll(sel)].map((f) => f.contentWindow.__frames || []), sel);
// a painted, visible frame shows one slide in one state: per slide only one count of hidden builds
const intermediate = (frames) => {
  const per = {};
  for (const [s, h, vis] of frames) if (vis && s >= 0) (per[s] = per[s] || new Set()).add(h);
  return Object.entries(per).filter(([, set]) => set.size > 1).map(([s, set]) => `slide ${s}: ${[...set].join('/')} hidden builds`);
};

test('previews paint no step between two states and nothing before their first state', async ({ page, context }) => {
  need('claudeSteps');
  await context.addInitScript(recordPreviewFrames);
  await page.goto(fileURL('claude-steps.html'));
  await page.waitForSelector('.df-start [data-df-start]');
  const k = await multiStepSlide(page);
  expect(k).toBeGreaterThan(0);
  await page.waitForTimeout(2500);
  // first load: the first visible frame already shows the chosen slide
  let [start] = await paintedFrames(page, '.df-start-stage iframe');
  const firstVisible = start.find(([, , vis]) => vis);
  expect(firstVisible && firstVisible[0]).toBe(0);
  // choosing slides: no step in between
  for (const i of [k, 1, k, 0, k]) { await page.click(`.df-strip .df-tile[data-i="${i}"]`); await page.waitForTimeout(900); }
  [start] = await paintedFrames(page, '.df-start-stage iframe');
  expect(intermediate(start)).toEqual([]);
  // presenter view: current and next while the show moves through the slides with builds
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  // the states the audience sees (forward, and back within slides), as the presentation shows them
  const seen = new Set();
  const note = async () => seen.add((await page.evaluate(() => {
    const all = [...document.querySelectorAll('.deck-slide')], i = all.findIndex((x) => !x.hasAttribute('data-off'));
    return [i, all[i].querySelectorAll('[data-build-hidden]').length];
  })).join());
  for (const key of ['ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowLeft', 'ArrowLeft', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'ArrowRight', 'ArrowRight']) {
    await pres.keyboard.press(key);
    await pres.waitForTimeout(700);
    await note();
  }
  const [current, audience, next] = await paintedFrames(pres, 'iframe[data-df-preview]');
  // "Current" and "Next" show whole slides only
  for (const f of [current, next]) {
    expect(intermediate(f)).toEqual([]);
    const first = f.find(([, , vis]) => vis);
    expect(first && first[0]).toBeGreaterThanOrEqual(0);
  }
  // the audience pane paints only states the audience saw, nothing in between
  const painted = audience.filter(([s, , vis]) => vis && s >= 0).map(([s, h]) => [s, h].join());
  expect(painted.length).toBeGreaterThan(0);
  expect(painted.filter((p) => !seen.has(p))).toEqual([]);
});

// The audience pane (below "Current"): what the audience sees when it is not the whole slide.
test('presenter view: the audience pane shows the step, black, white and the end, and is empty at the last step', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-animations.html');
  const aud = pres.locator('.df-audience');
  const view = () => aud.getAttribute('data-df-audience');
  const audFrame = () => pres.evaluate(() => { const f = document.querySelector('.df-audience iframe'); return f.contentWindow.__dfPptx ? f.contentWindow.__dfPptx.state() : null; });
  const steps = await page.evaluate(() => window.__deckfold.slides()[0].steps);
  expect(steps).toBeGreaterThan(1);
  // step 0 of a slide with steps: the audience pane shows that step
  await expect.poll(view).toBe('slide');
  await expect.poll(audFrame, { timeout: 10000 }).toEqual({ slide: 0, step: 0 });
  await pres.keyboard.press('ArrowRight');
  await expect.poll(audFrame).toEqual({ slide: 0, step: 1 });
  // back within the slide
  await pres.keyboard.press('ArrowLeft');
  await expect.poll(audFrame).toEqual({ slide: 0, step: 0 });
  // the last step: the audience sees what "Current" shows, the pane is empty
  for (let i = 1; i < steps; i++) await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 0, step: steps - 1 });
  await expect.poll(view).toBe('empty');
  await pres.keyboard.press('b');
  await expect.poll(view).toBe('black');
  await pres.keyboard.press('b');
  await expect.poll(view).toBe('empty');
  await pres.keyboard.press('w');
  await expect.poll(view).toBe('white');
  await pres.keyboard.press('w');
  await pres.keyboard.press('End');
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => page.evaluate(() => window.__deckfold.audience.info().ended)).toBe(true);
  await expect.poll(view).toBe('end');
  // the switch hides and shows it; the divider above it is dragged and the share is kept
  await pres.click('.df-status [data-df-el="audience"]');
  await expect(aud).toBeHidden();
  await pres.click('.df-status [data-df-el="audience"]');
  await expect(aud).toBeVisible();
  const box = await pres.locator('[data-df-split="audience"]').boundingBox();
  await pres.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await pres.mouse.down();
  await pres.mouse.move(box.x + box.width / 2, box.y - 80, { steps: 5 });
  await pres.mouse.up();
  const share = await pres.evaluate(() => window.__deckfold.settings().presenter.layout.audience);
  expect(share).toBeLessThan(0.6);
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  expect(await pres.evaluate(() => window.__deckfold.settings().presenter.layout.audience)).toBeCloseTo(share, 5);
});

test('claude.ai presenter view: the audience pane shows the builds the audience sees', async ({ page, context }) => {
  need('claudeSteps');
  const pres = await present(page, context, 'claude-steps.html');
  const k = await multiStepSlide(page);
  await pres.click(`.df-overview .df-tile[data-i="${k}"]`);
  await expect.poll(() => visibleSlide(page)).toBe(k);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next'], { slide: k, step: 0 }));
  await page.waitForTimeout(1500);
  const inShow = await page.evaluate((k) => document.querySelectorAll('.deck-slide')[k].querySelectorAll('[data-build-hidden]').length, k);
  await expect.poll(async () => (await previewInfo(pres, '.df-audience iframe[data-df-preview]'))[0].slice(0, 2), { timeout: 10000 }).toEqual([k, inShow]);
});

// Presenter view in windows of every size (the maintainer's narrow window included): no preview
// squeezed to nothing, no control out of reach, a bar of one line.
test('presenter view: the layout fits windows from 1400 x 900 down to 300 x 900', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  for (const [w, h] of [[1400, 900], [1200, 760], [1000, 650], [800, 600], [600, 800], [400, 900], [300, 900]]) {
    await pres.setViewportSize({ width: w, height: h });
    await pres.waitForTimeout(400);
    const m = await pres.evaluate(() => {
      const ui = document.querySelector('.df-presenter');
      const bar = document.querySelector('.df-bar');
      const stages = [...document.querySelectorAll('.df-pane:not(.df-hidden) .df-stage')].map((s) => Math.round(s.getBoundingClientRect().height));
      // a control out of the window must sit in a container that scrolls to it
      const lost = [...document.querySelectorAll('.df-bar button, .df-bar input, .df-status button')].filter((b) => {
        const r = b.getBoundingClientRect();
        if (r.right <= innerWidth + 1 && r.bottom <= innerHeight + 1) return false;
        const c = b.closest('.df-bar, .df-status');
        return !(c && c.scrollWidth > c.clientWidth);
      }).map((b) => b.textContent || b.getAttribute('data-df-el'));
      return { narrow: ui.classList.contains('df-narrow'), barH: Math.round(bar.getBoundingClientRect().height), stages, lost };
    });
    expect(m.narrow, `${w}x${h}`).toBe(w < 760);
    expect(m.barH, `${w}x${h} bar`).toBeLessThan(60);
    for (const s of m.stages) expect(s, `${w}x${h} stages ${m.stages}`).toBeGreaterThanOrEqual(60);
    expect(m.lost, `${w}x${h}`).toEqual([]);
  }
});

test('presenter view: closes with the window of the presentation, stays when that window reloads', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  // a reload of the presentation window ends the show and keeps the presenter view
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await page.waitForTimeout(2500);
  expect(pres.isClosed()).toBe(false);
  await pres.waitForSelector('.df-presenter');
  // closing it closes the presenter view
  const closed = pres.waitForEvent('close', { timeout: 4000 });
  await page.close();
  await closed;
  expect(pres.isClosed()).toBe(true);
});
