// Presenting (decision E14): the window the file is opened in is the presentation, with a start
// screen first; the presenter view is a second window that the user drags to a screen. Where the
// windows are is not checked; full screen can always be chosen.
import { test, expect } from '@playwright/test';
import { startScreen, openPresenter, solo, present, audState, fileURL, need, visibleSlide, expected, counter } from './helpers.mjs';

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
  // the last slide (the third is hidden from the show, E21)
  await page.click('.df-strip .df-tile[data-i="3"]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '3');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
  await expect(page.locator('.df-start')).toHaveCount(0);
  expect(await deferred(page)).toBe(0);
  // the end returns to the start screen by reloading: the player is inactive again, the last slide is chosen
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  expect(await deferred(page)).toBeGreaterThan(0);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '3');
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
  await expect.poll(() => counter(pres)).toContain('2 / ');
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

test('overview band of the presenter view: click, arrows; time and band are always shown', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  // before the start: a click and the arrows choose the start slide (the third slide is hidden
  // from the show and not in this strip, E21)
  await pres.click('.df-overview .df-tile[data-i="3"]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '3');
  await pres.click('.df-overview [data-df-strip-prev]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  // during the show: a click jumps, the arrows only scroll the band
  await pres.click('.df-overview .df-tile[data-i="3"]');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
  await pres.click('.df-overview [data-df-strip-prev]');
  await page.waitForTimeout(400);
  expect(await audState(page)).toEqual({ slide: 3, step: 0 });
  // the status bar holds three switches; the time and the band have none and are always shown (E20)
  const switches = () => pres.evaluate(() => [...document.querySelectorAll('.df-status [data-df-el]')].map((b) => b.getAttribute('data-df-el')));
  expect(await switches()).toEqual(['audience', 'next', 'notes']);
  await expect(pres.locator('.df-overview')).toBeVisible();
  await expect(pres.locator('.df-time-group')).toBeVisible();
  // a list stored by v0.9.1 that hid "Live" (then "audience"), the notes, the time and the
  // band: "current" was the large slide on the left and becomes "Live"; time and band stay
  await pres.evaluate(() => localStorage.setItem('deckfold:' + window.__deckfold.meta.deckHash, JSON.stringify({ presenter: { elements: ['current', 'next'] } })));
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  await expect(pres.locator('.df-audience')).toBeVisible();
  await expect(pres.locator('.df-next')).toBeVisible();
  await expect(pres.locator('.df-notes')).toBeHidden();
  await expect(pres.locator('.df-overview')).toBeVisible();
  await expect(pres.locator('.df-time-group')).toBeVisible();
  // a switch stores today's names
  await pres.click('.df-status [data-df-el="notes"]');
  await expect(pres.locator('.df-notes')).toBeVisible();
  expect(await pres.evaluate(() => window.__deckfold.settings().presenter.elements)).toEqual(['audience', 'next', 'notes']);
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
  // a slide number in the address would override the chosen start slide in some players; the
  // start screen keeps only its own start slide there (#df-last=n), which it drops before a player runs
  expect(await page.evaluate(() => location.hash)).toMatch(/^(#df-last=\d+)?$/);
  await expect(page.locator('[data-df-strip-prev]')).toBeDisabled();
  // the arrows of the start screen go through every slide, the hidden third one too (E21)
  await page.click('[data-df-strip-next]');
  await page.click('[data-df-strip-next]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await page.click('[data-df-strip-next]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '3');
  await expect(page.locator('[data-df-strip-next]')).toBeDisabled();
  await page.uncheck('[data-df-auto]');
  await expect(page.locator('[data-df-interval]')).toBeDisabled();
  await expect(page.locator('[data-df-loop]')).toBeDisabled();
  await page.check('[data-df-auto]');
  await expect(page.locator('[data-df-interval]')).toBeEnabled();
  await expect(page.locator('[data-df-keyhelp] kbd')).not.toHaveCount(0);
});

// A tab keeps its session storage when it opens another file. The token of the session and the
// slide chosen last were stored there under fixed names, so a second deck in the same tab took
// over the presenter view of the first (its switch was on without a click, while that window
// still showed the first deck) and started at the slide chosen in the first.
test('another deck in the same tab does not take over the presenter view or the chosen slide of the first', async ({ page, context }) => {
  await startScreen(page, 'pptx-animations.html');
  const first = await openPresenter(page, context);
  await page.click('.df-strip .df-tile[data-i="2"]');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  // the second deck, with the presenter view of the first still open
  await page.goto(fileURL('pptx-content.html'));
  await page.waitForSelector('.df-start [data-df-start]');
  await page.waitForTimeout(2500); // longer than the heartbeat of a presenter view
  expect(first.isClosed()).toBe(false);
  expect(await page.isChecked('[data-df-presenter]'), 'the presenter view of the other deck is not this deck\'s').toBe(false);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '0');
  // its own presenter view is a window of its own and shows this deck
  const second = await openPresenter(page, context);
  expect(second).not.toBe(first);
  expect(await second.textContent('.df-title')).toBe(await page.textContent('.df-start h1'));
  expect(await first.textContent('.df-title')).not.toBe(await page.textContent('.df-start h1'));
  await page.uncheck('[data-df-presenter]');
  await expect.poll(() => second.isClosed()).toBe(true);
  // back in the first deck: its presenter view connects again, and its slide is still chosen
  await page.goto(fileURL('pptx-animations.html'));
  await page.waitForSelector('.df-start [data-df-start]');
  await expect.poll(() => page.isChecked('[data-df-presenter]'), { timeout: 8000 }).toBe(true);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '2');
  await page.uncheck('[data-df-presenter]');
  await expect.poll(() => first.isClosed()).toBe(true);
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
  await expect.poll(() => counter(pres), { timeout: 8000 }).toMatch(/^2 \//);
  // the reloaded view knows the slides hidden from the show (E21): the third is not in its strip,
  // and "next" goes past it
  await expect(pres.locator('.df-overview .df-tile[data-i="2"]')).toBeHidden();
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
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
// [visible slide, number of hidden builds on it, running animations] in the previews "Live" and
// "Preview" of the presenter view (sel: other previews)
const previewInfo = (pres, sel = '.df-audience iframe[data-df-preview], .df-next iframe[data-df-preview]') => pres.evaluate((sel) => [...document.querySelectorAll(sel)].map((f) => {
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

test('claude.ai presenter view: the previews show the step on screen and the next click, without animating', async ({ page, context }) => {
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
  // the slide has no auto steps, so the next click is its second step
  await expect.poll(async () => (await previewInfo(pres)).map((p) => p[0]), { timeout: 10000 }).toEqual([k, k]);
  await pres.waitForTimeout(1500);
  const [onScreen, nextClick] = await previewInfo(pres);
  // "Live" shows the builds the audience sees; "Preview" those of the step after it
  expect(onScreen[1]).toBe(atStart);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: k, step: 1 });
  await page.waitForTimeout(1500);
  const atStep1 = await hiddenInShow();
  expect(atStart).not.toBe(atStep1); // the slide has builds, so step 0 and step 1 differ
  expect(nextClick[1]).toBe(atStep1);
});

test('reload: the start screen chooses the slide chosen or shown last', async ({ page }) => {
  // for the failure messages: what the session keeps for the start slide, read in the page
  const kept = () => page.evaluate(() => Object.keys(sessionStorage).filter((k) => k.startsWith('df-last')).map((k) => sessionStorage.getItem(k)).join(' ') || 'nothing');
  const reload = async (what) => {
    const before = await kept();
    await page.reload();
    await page.waitForSelector('.df-start [data-df-start]');
    return `${what}: stored before the reload ${before}, after it ${await kept()}`;
  };
  await startScreen(page, 'pptx-basic.html');
  await page.click('.df-strip .df-tile[data-i="2"]');
  let said = await reload('chosen');
  await expect(page.locator('.df-strip .df-tile.df-cur'), said).toHaveAttribute('data-i', '2');
  // shown last: the show reached slide 2 before the reload
  await page.click('.df-strip .df-tile[data-i="0"]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  // the start slide in the address is gone before the player runs
  expect(await page.evaluate(() => location.hash)).toBe('');
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  said = await reload('shown');
  await expect(page.locator('.df-strip .df-tile.df-cur'), said).toHaveAttribute('data-i', '1');
  // chosen after that show: the choice wins
  await page.click('.df-strip .df-tile[data-i="2"]');
  said = await reload('chosen after the show');
  await expect(page.locator('.df-strip .df-tile.df-cur'), said).toHaveAttribute('data-i', '2');
  // the end of the show (Escape) reloads this window itself: the start screen chooses the slide
  // shown last, without a pause between the move and the end
  await page.click('.df-strip .df-tile[data-i="0"]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile.df-cur'), `ended: stored ${await kept()}`).toHaveAttribute('data-i', '1');
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
  await expect(btn).toHaveAttribute('aria-label', await label('fullscreen'));
  await btn.click();
  await expect(btn).toHaveAttribute('aria-label', await label('window'));
  await btn.click();
  await expect(btn).toHaveAttribute('aria-label', await label('fullscreen'));
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

// Keyboard Lock keeps Escape from leaving full screen, so that one Escape ends the presentation.
// Before the start and after the end there is no presentation to end: there Escape leaves full
// screen in the presenter view.
test('presenter view in full screen before the start and after the end: Escape leaves full screen', async ({ page, context }) => {
  await context.addInitScript(fakeFullscreen);
  await startScreen(page, 'pptx-basic.html');
  await page.uncheck('[data-df-fs]');
  const pres = await openPresenter(page, context);
  const full = () => pres.evaluate(() => !!document.fullscreenElement);
  await pres.click('.df-presenter [data-df-fs]');
  await expect.poll(full).toBe(true);
  expect(await pres.evaluate(() => window.__locks)).toEqual([['Escape']]);
  await pres.keyboard.press('Escape');
  await expect.poll(full, { message: 'full screen after Escape before the start' }).toBe(false);
  // the start screen is still there: Escape before the start ended nothing
  await expect(page.locator('.df-start [data-df-start]')).toBeVisible();
  // after the end of a show
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  await pres.click('.df-presenter [data-df-fs]');
  await expect.poll(full).toBe(true);
  await pres.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible({ timeout: 5000 });
  expect(await full(), 'the presenter view stays in full screen when Escape ends the show').toBe(true);
  await pres.keyboard.press('Escape');
  await expect.poll(full, { message: 'full screen after Escape after the end' }).toBe(false);
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

test('time: stands still when the show ends and runs on from there with the next start', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(async () => secs(await timeOf(pres)), { timeout: 6000 }).toBeGreaterThanOrEqual(3);
  await pres.click('.df-presenter [data-df-end]');
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible({ timeout: 5000 });
  const held = await timeOf(pres);
  await pres.waitForTimeout(2500);
  expect(await timeOf(pres), 'the time after the end of the show').toBe(held);
  // the next start goes on from the time the last show ended with, not from zero
  await page.evaluate(() => { window.__startAt = Date.now(); });
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  // what both windows show, with the time of each window (the same clock), so that a failure in CI
  // tells where the show was: the start screen removes itself once the show runs, and the
  // presenter view leaves the state before the start when the show window says so
  const states = async () => ({
    show: await page.evaluate(() => ({ now: Date.now(), startedAt: window.__startAt, audience: !!(window.__deckfold && window.__deckfold.audience), startScreen: !!document.querySelector('.df-start') })),
    presenter: await pres.evaluate(() => {
      const ui = document.querySelector('.df-presenter');
      return { now: Date.now(), before: ui.classList.contains('df-before'), at: ui.getAttribute('data-df-at'), time: document.querySelector('.df-elapsed').textContent, banner: document.querySelector('[data-df-banner]').getAttribute('data-df-banner') };
    }),
  });
  let last;
  for (const until = Date.now() + 5000; ;) {
    last = await states();
    if (!last.presenter.before || Date.now() > until) break;
    await pres.waitForTimeout(100);
  }
  expect(last.presenter.before, 'the presenter view still shows the state before the start: ' + JSON.stringify(last)).toBe(false);
  expect(secs(await timeOf(pres)), 'the time right after the next start').toBeGreaterThanOrEqual(secs(held));
  await expect.poll(async () => secs(await timeOf(pres)), { timeout: 4000 }).toBeGreaterThan(secs(held));
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
  // presenter view: "Live" and "Preview" while the show moves through the slides with builds
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  // the states the audience sees (forward, and back within slides), as the presentation shows them;
  // before the start "Live" showed the start slide, which the show starts on
  const seen = new Set();
  const note = async () => seen.add((await page.evaluate(() => {
    const all = [...document.querySelectorAll('.deck-slide')], i = all.findIndex((x) => !x.hasAttribute('data-off'));
    return [i, all[i].querySelectorAll('[data-build-hidden]').length];
  })).join());
  await page.waitForTimeout(700);
  await note();
  for (const key of ['ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowLeft', 'ArrowLeft', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'ArrowRight', 'ArrowRight']) {
    await pres.keyboard.press(key);
    await pres.waitForTimeout(700);
    await note();
  }
  const [audience, next] = await paintedFrames(pres, 'iframe[data-df-preview]');
  for (const f of [audience, next]) {
    const first = f.find(([, , vis]) => vis);
    expect(first && first[0]).toBeGreaterThanOrEqual(0);
  }
  // "Live" paints only states the audience saw, nothing in between
  const painted = audience.filter(([s, , vis]) => vis && s >= 0).map(([s, h]) => [s, h].join());
  expect(painted.length).toBeGreaterThan(0);
  expect(painted.filter((p) => !seen.has(p))).toEqual([]);
  // "Preview" paints only states the audience saw after the next click, and the state after
  // the last one, which the show did not reach
  const ahead = next.filter(([s, , vis]) => vis && s >= 0).map(([s, h]) => [s, h].join());
  expect(ahead.length).toBeGreaterThan(0);
  expect(ahead.slice(0, -1).filter((p) => !seen.has(p))).toEqual([]);
});

// "Live" (the audience pane, E20): what the audience sees; before the start the start slide.
test('presenter view: "Live" shows the start slide, the step, black, white and the end, and is never empty', async ({ page, context }) => {
  await startScreen(page, 'pptx-animations.html');
  const pres = await openPresenter(page, context);
  const aud = pres.locator('.df-audience');
  const view = () => aud.getAttribute('data-df-audience');
  const audFrame = () => pres.evaluate(() => { const f = document.querySelector('.df-audience iframe'); return f.contentWindow.__dfPptx ? f.contentWindow.__dfPptx.state() : null; });
  // before the start: the slide the show will start on, at its first step
  await expect.poll(view).toBe('start');
  await expect.poll(audFrame, { timeout: 10000 }).toEqual({ slide: 0, step: 0 });
  await expect(aud.locator('iframe')).toBeVisible();
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
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
  // the last step
  for (let i = 1; i < steps; i++) await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 0, step: steps - 1 });
  await expect.poll(view).toBe('slide');
  await expect.poll(audFrame).toEqual({ slide: 0, step: steps - 1 });
  await expect(aud.locator('iframe')).toBeVisible();
  await pres.keyboard.press('b');
  await expect.poll(view).toBe('black');
  await pres.keyboard.press('b');
  await expect.poll(view).toBe('slide');
  await pres.keyboard.press('w');
  await expect.poll(view).toBe('white');
  await pres.keyboard.press('w');
  await pres.keyboard.press('End');
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => page.evaluate(() => window.__deckfold.audience.info().ended)).toBe(true);
  await expect.poll(view).toBe('end');
  // the switch hides and shows it; the divider beside it is dragged and the share is kept
  await pres.click('.df-status [data-df-el="audience"]');
  await expect(aud).toBeHidden();
  await pres.click('.df-status [data-df-el="audience"]');
  await expect(aud).toBeVisible();
  const box = await pres.locator('[data-df-split="main"]').boundingBox();
  await pres.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await pres.mouse.down();
  await pres.mouse.move(box.x - 80, box.y + box.height / 2, { steps: 5 });
  await pres.mouse.up();
  const share = await pres.evaluate(() => window.__deckfold.settings().presenter.layout.main);
  expect(share).toBeLessThan(0.6);
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  expect(await pres.evaluate(() => window.__deckfold.settings().presenter.layout.main)).toBeCloseTo(share, 5);
});

// E20: a dot for each state the slide rests in, filled up to the one shown, and an arrow for the
// change to the next slide, marked when the next click makes it. The panes are "Live" and
// "Preview", the same on their headings and their switches, whatever the next click does; the
// right pane is never marked.
test('presenter view: the dots of "Live" count the clicks to the next slide; "Live" and "Preview" keep their names', async ({ page, context }) => {
  await startScreen(page, 'pptx-animations.html');
  const pres = await openPresenter(page, context);
  const steps = await page.evaluate(() => window.__deckfold.slides()[0].steps);
  expect(steps).toBeGreaterThan(1);
  expect(steps).toBeLessThan(13);
  const read = () => pres.evaluate(() => {
    const s = document.querySelector('.df-audience [data-df-steps]'), a = s && s.querySelector('.df-arrow');
    return {
      steps: s ? s.getAttribute('data-df-steps') : 'missing',
      dots: s ? s.querySelectorAll('.df-dot').length : -1,
      done: s ? s.querySelectorAll('.df-dot.df-done').length : -1,
      arrow: a ? a.classList.contains('df-next-is-slide') : null,
      change: document.querySelector('.df-next').classList.contains('df-change'),
      names: ['.df-audience .df-pane-title', '[data-df-el="audience"]', '.df-next .df-pane-title', '[data-df-el="next"]'].map((q) => document.querySelector(q).textContent)
    };
  });
  const [live, upNext] = await pres.evaluate(() => [window.__deckfold.t('live'), window.__deckfold.t('upNext')]);
  const names = [live, live, upNext, upNext];
  expect(live).not.toBe('live');
  expect(upNext).not.toBe('upNext');
  const at = (i) => ({ steps: `${i}/${steps}`, dots: steps, done: i, arrow: i === steps, change: false, names });
  // before the start: the start slide at its first state
  await expect.poll(read).toEqual(at(1));
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  for (let i = 1; i <= steps; i++) {
    await expect.poll(() => audState(page)).toEqual({ slide: 0, step: i - 1 });
    await expect.poll(read).toEqual(at(i));
    if (i < steps) await pres.keyboard.press('ArrowRight');
  }
  // the next click goes to the next slide; its dots start again
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
  const n1 = await page.evaluate(() => window.__deckfold.slides()[1].steps);
  await expect.poll(read).toEqual({ steps: `1/${n1}`, dots: n1, done: 1, arrow: n1 === 1, change: false, names });
  // at the end of the show: no dots, nothing marked
  await pres.keyboard.press('End');
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => page.evaluate(() => window.__deckfold.audience.info().ended)).toBe(true);
  await expect.poll(read).toEqual({ steps: '', dots: 0, done: 0, arrow: null, change: false, names });
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
      }).map((b) => b.textContent || b.getAttribute('aria-label') || b.getAttribute('data-df-el'));
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

test('presenter view: started on a slide without steps, the audience pane shows that slide at once', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  expect(await page.evaluate(() => window.__deckfold.slides()[0].steps)).toBe(1);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => pres.getAttribute('.df-audience', 'data-df-audience')).toBe('slide');
  await expect(pres.locator('.df-audience iframe')).toBeVisible();
  await expect.poll(() => pres.evaluate(() => { const w = document.querySelector('.df-audience iframe').contentWindow; return w.__dfPptx ? w.__dfPptx.state() : null; }), { timeout: 10000 }).toEqual({ slide: 0, step: 0 });
});

// Commands right after "Start": the presenter view already reports the show while the window of
// the presentation still loads its first slide; nothing sent or pressed then may get lost.
test('commands right after Start are carried out, from the presenter view and by key', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  await pres.click('.df-overview .df-tile[data-i="3"]');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
  // by key in the presentation window, at once after the click on Start
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await page.click('.df-strip .df-tile[data-i="0"]');
  await page.click('[data-df-start]');
  await page.keyboard.press('ArrowRight');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => audState(page)).toEqual({ slide: 1, step: 0 });
});

// A control clicked with the mouse gives the focus back, so that the keys that move through the
// slides never show a focus ring on it; a tile reached with Tab keeps the focus on the marked tile.
test('focus: after a click and a key, one ring only, on the marked tile', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const rings = (p, root) => p.evaluate((root) => [...document.querySelectorAll(root + ' .df-tile')].map((t) =>
    [+t.getAttribute('data-i'), t.classList.contains('df-cur'), t.matches(':focus-visible')]), root);
  const stray = (r) => r.filter(([, cur, fv]) => fv && !cur);
  await page.click('.df-start .df-strip .df-tile[data-i="1"]');
  await page.keyboard.press('ArrowRight');
  expect(stray(await rings(page, '.df-start'))).toEqual([]);
  // with Tab on a tile, the focus moves with the marked tile
  await page.focus('.df-start .df-strip .df-tile[data-i="2"]');
  await page.keyboard.press('ArrowLeft');
  const r = await rings(page, '.df-start');
  expect(stray(r)).toEqual([]);
  expect(r.find(([, cur]) => cur)[0]).toBe(1);
  // presenter view: a clicked tile and a clicked switch keep no focus ring after keys
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  await pres.click('.df-overview .df-tile[data-i="1"]');
  await pres.click('.df-status [data-df-el="notes"]');
  await pres.click('.df-status [data-df-el="notes"]');
  await pres.keyboard.press('ArrowRight');
  await pres.waitForTimeout(300);
  expect(stray(await rings(pres, '.df-overview'))).toEqual([]);
  expect(await pres.evaluate(() => [...document.querySelectorAll('button')].filter((b) => b.matches(':focus-visible') && !b.classList.contains('df-cur')).map((b) => b.getAttribute('data-df-el') || b.textContent || b.getAttribute('aria-label')))).toEqual([]);
});

// Screen wake lock (decision E19, docs/spikes/S10-wake-lock.md). A stand-in for navigator.wakeLock
// counts the requests of every window and frame and grants, refuses or leaves them open, so that
// the tests depend neither on the headless browser nor on the system.
const fakeWakeLock = () => {
  const ctl = window.__fakeWake = { mode: 'grant', calls: 0, held: [], during: [], order: [] };
  // requestFullscreen uses the click up, after which Safari refuses the wake lock
  const fs = Element.prototype.requestFullscreen;
  if (fs) Element.prototype.requestFullscreen = function (...a) { ctl.order.push('fullscreen'); return fs.apply(this, a); };
  class Sentinel extends EventTarget {
    constructor() { super(); this.type = 'screen'; this.released = false; }
    release() {
      if (!this.released) { this.released = true; ctl.held = ctl.held.filter((s) => s !== this); this.dispatchEvent(new Event('release')); }
      return Promise.resolve();
    }
  }
  const api = {
    request() {
      ctl.calls++;
      ctl.during.push(window.event ? window.event.type : null); // the event the request is made in, if any
      ctl.order.push('wake');
      if (ctl.mode === 'refuse') return Promise.reject(new DOMException('refused', 'NotAllowedError'));
      if (ctl.mode === 'hang') return new Promise(() => {});
      const s = new Sentinel();
      ctl.held.push(s);
      return Promise.resolve(s);
    },
  };
  Object.defineProperty(Navigator.prototype, 'wakeLock', { configurable: true, get: () => api });
};
const wakeCalls = (p) => p.evaluate(() => window.__fakeWake.calls);
const frameWakeCalls = async (p) => {
  let n = 0;
  for (const f of p.frames()) if (f !== p.mainFrame()) n += await f.evaluate(() => (window.__fakeWake ? window.__fakeWake.calls : 0)).catch(() => 0);
  return n;
};
const wakeInfo = (p) => p.evaluate(() => window.__deckfold.audience.info().wake);
const banner = (pres) => pres.locator('.df-banner').getAttribute('data-df-banner');
// starts with the presenter view after setting the mode of the stand-in in the presentation window
async function presentWith(page, context, mode) {
  await startScreen(page, 'pptx-basic.html');
  await page.evaluate((m) => { window.__fakeWake.mode = m; }, mode);
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  return pres;
}

test('wake lock: nothing before Start; after it one request from the presentation, none from the previews', async ({ page, context }) => {
  await context.addInitScript(fakeWakeLock);
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.waitForTimeout(1500);
  expect([await wakeCalls(page), await frameWakeCalls(page), await wakeCalls(pres), await frameWakeCalls(pres)]).toEqual([0, 0, 0, 0]);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => wakeInfo(page)).toBe('held');
  // Safari grants the lock only shortly after a click or key and not after requestFullscreen in
  // the same click, so it is asked for in the click on Start, before full screen
  expect(await page.evaluate(() => window.__fakeWake.during)).toEqual(['click']);
  expect(await page.evaluate(() => window.__fakeWake.order.slice(0, 2))).toEqual(['wake', 'fullscreen']);
  await page.waitForTimeout(2000);
  expect(await wakeCalls(page)).toBe(1);
  expect(await frameWakeCalls(page)).toBe(0);
  expect(await frameWakeCalls(pres)).toBe(0);
  await expect.poll(() => banner(pres)).toBe('');
});

test('wake lock: a lock the browser releases is requested again', async ({ page, context }) => {
  await context.addInitScript(fakeWakeLock);
  await solo(page, 'pptx-basic.html');
  await expect.poll(() => page.evaluate(() => window.__fakeWake.held.length)).toBe(1);
  await page.evaluate(() => window.__fakeWake.held[0].release());
  await expect.poll(() => wakeCalls(page), { timeout: 2000 }).toBe(2);
  await expect.poll(() => wakeInfo(page)).toBe('held');
});

test('wake lock: a refused request shows the hint in the presenter view until a later request is granted', async ({ page, context }) => {
  await context.addInitScript(fakeWakeLock);
  const pres = await presentWith(page, context, 'refuse');
  await expect.poll(() => banner(pres)).toBe('noWake');
  expect(await wakeInfo(page)).toBe('off');
  await page.evaluate(() => { window.__fakeWake.mode = 'grant'; });
  await expect.poll(() => banner(pres), { timeout: 15000 }).toBe('');
  expect(await wakeInfo(page)).toBe('held');
  expect(await wakeCalls(page)).toBeGreaterThanOrEqual(2);
});

test('wake lock: a request left open shows no hint for 3 s, then the hint, and is not repeated', async ({ page, context }) => {
  await context.addInitScript(fakeWakeLock);
  await startScreen(page, 'pptx-basic.html');
  await page.evaluate(() => { window.__fakeWake.mode = 'hang'; });
  const pres = await openPresenter(page, context);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  expect(await wakeInfo(page)).toBe('pending');
  await pres.waitForFunction(() => !document.querySelector('.df-presenter.df-before'));
  expect(await banner(pres)).not.toBe('noWake');
  await expect.poll(() => banner(pres), { timeout: 6000 }).toBe('noWake');
  expect(await wakeInfo(page)).toBe('off');
  await page.waitForTimeout(1000);
  expect(await wakeCalls(page)).toBe(1);
});

test('wake lock: without the API the presenter view shows the hint', async ({ page, context }) => {
  await context.addInitScript(() => { Object.defineProperty(Navigator.prototype, 'wakeLock', { configurable: true, get: () => undefined }); });
  const pres = await present(page, context, 'pptx-basic.html');
  await expect.poll(() => wakeInfo(page)).toBe('none');
  await expect.poll(() => banner(pres)).toBe('noWake');
});

test('wake lock: after a refusal, a key in the presentation window asks again at once', async ({ page, context }) => {
  await context.addInitScript(fakeWakeLock);
  await startScreen(page, 'pptx-basic.html');
  await page.evaluate(() => { window.__fakeWake.mode = 'refuse'; });
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await expect.poll(() => wakeInfo(page)).toBe('off');
  await page.evaluate(() => { window.__fakeWake.mode = 'grant'; });
  await page.keyboard.press('ArrowRight');
  // well before the next request of the 10 s rhythm
  await expect.poll(() => wakeInfo(page), { timeout: 2000 }).toBe('held');
  expect((await page.evaluate(() => window.__fakeWake.during)).at(-1)).toBe('keydown');
});

// The show window reloads at the end of a show. The switch of the presenter view stands as it
// stood before, from the first frame of the start screen on, and does not wait for the next
// heartbeat of the presenter view (which comes once a second).
const switchAtStart = (page) => page.waitForFunction(() => {
  const s = document.querySelector('.df-start [data-df-start]'), b = document.querySelector('[data-df-presenter]');
  return s && b ? { checked: b.checked } : null;
}, null, { polling: 'raf' }).then((h) => h.jsonValue());

test('end of the show with the presenter view open: the switch on the start screen is on at once', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  await page.keyboard.press('Escape');
  expect(await switchAtStart(page)).toEqual({ checked: true });
  // the presenter view is the same window and goes back to the start screen
  await expect(pres.locator('.df-presenter.df-before')).toBeVisible({ timeout: 5000 });
  expect(pres.isClosed()).toBe(false);
});

// Opened with ";p" in the address, the start screen expects the presenter view of the time before
// the reload. When none reports within 3 s (it would send once a second), the switch goes off.
test('the switch of the presenter view goes off when no presenter view reports after a reload', async ({ page }) => {
  await page.goto(fileURL('pptx-basic.html') + '#df-last=0;p');
  expect(await switchAtStart(page)).toEqual({ checked: true });
  await expect(page.locator('[data-df-presenter]')).not.toBeChecked({ timeout: 3500 });
  const key = await page.evaluate(() => window.__deckfold.deckKey('df-presenter'));
  expect(await page.evaluate((k) => sessionStorage.getItem(k), key)).toBeNull();
  expect(page.url()).toMatch(/#df-last=0$/);
});

// Switched off right after the reload, before the presenter view has reported: the presenter view
// is closed when it reports, and the switch stays off.
test('the switch of the presenter view turned off right after the end closes the presenter view', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  const closed = pres.waitForEvent('close', { timeout: 4000 });
  await page.keyboard.press('Escape');
  expect(await switchAtStart(page)).toEqual({ checked: true });
  await page.uncheck('[data-df-presenter]');
  await closed;
  await page.waitForTimeout(1500);
  await expect(page.locator('[data-df-presenter]')).not.toBeChecked();
});

// A- and A+ change the size of the notes; while the notes are hidden they are off
test('presenter view: A- and A+ are off while the notes are hidden', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  expect(await page.evaluate(() => window.__deckfold.slides().some((s) => s.notes))).toBe(true);
  const font = () => pres.evaluate(() => window.__deckfold.settings().presenter.notesFontSize);
  const minus = pres.locator('[data-df-font="-"]'), plus = pres.locator('[data-df-font="+"]');
  await expect(minus).toBeEnabled();
  await expect(plus).toBeEnabled();
  await pres.click('.df-status [data-df-el="notes"]');
  await expect(pres.locator('.df-notes')).toBeHidden();
  await expect(minus).toBeDisabled();
  await expect(plus).toBeDisabled();
  const before = await font();
  await plus.click({ force: true });
  expect(await font()).toBe(before);
  await pres.click('.df-status [data-df-el="notes"]');
  await expect(plus).toBeEnabled();
  await plus.click();
  expect(await font()).toBe(before + 2);
});

// Search, full screen and end show are buttons with an icon only; the name is their tooltip
test('presenter view: search, full screen and end show are icons with a name', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  const m = await pres.evaluate(() => ['[data-df-find]', '[data-df-fs]', '[data-df-end]'].map((q) => {
    const b = document.querySelector('.df-presenter ' + q);
    return b && { svg: b.querySelectorAll('svg').length, text: b.textContent.trim(), label: b.getAttribute('aria-label'), title: b.getAttribute('title') };
  }));
  const t = await pres.evaluate(() => ['search', 'fullscreen', 'endShow'].map((k) => window.__deckfold.t(k)));
  expect(m).toEqual(t.map((label) => ({ svg: 1, text: '', label, title: label })));
});

// No counter in the top bar; the number of slides at the left of the status bar
test('presenter view: no counter in the top bar, the number of slides at the left of the status bar', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  const m = await pres.evaluate(() => {
    const bar = document.querySelector('.df-bar'), status = document.querySelector('.df-status');
    const count = status.querySelector('[data-df-count]');
    const n = window.__deckfold.slides().length, shown = n - window.__deckfold.hiddenSlides().length;
    const title = bar.querySelector('.df-title').textContent;
    return {
      counter: document.querySelectorAll('.df-counter').length,
      slash: bar.textContent.replace(title, '').includes('/'),
      first: status.firstElementChild === count,
      text: count && count.textContent, want: window.__deckfold.slideCount(n, shown), all: window.__deckfold.slideCount(n), n, shown,
      of: window.__deckfold.t('of'),
      one: window.__deckfold.slideCount(1), nine: window.__deckfold.slideCount(9),
      oneWord: window.__deckfold.t('oneSlide'), many: window.__deckfold.t('slides')
    };
  });
  expect(m.counter).toBe(0);
  expect(m.slash).toBe(false);
  expect(m.first).toBe(true);
  expect(m.text).toBe(m.want);
  // the third slide of the basic deck is hidden from the show (E21): "3 of 4 slides"
  expect(m.shown).toBe(m.n - 1);
  expect(m.want).toBe(`${m.shown} ${m.of} ${m.n} ${m.many}`);
  expect(m.all).toBe(`${m.n} ${m.many}`);
  // one slide is "1 slide", not "1 slides"
  expect(m.one).toBe(`1 ${m.oneWord}`);
  expect(m.nine).toBe(`9 ${m.many}`);
  expect(m.oneWord).not.toBe(m.many);
});

// The divider above the overview: dragged up, the tiles grow; the width is stored and comes back;
// the overview never takes more than 45 % of the window height
test('presenter view: the overview can be dragged higher, with larger tiles', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  const width = () => pres.evaluate(() => document.querySelector('.df-overview .df-tile').getBoundingClientRect().width);
  const w0 = await width();
  const box = await pres.locator('[data-df-split="strip"]').boundingBox();
  expect(box, 'divider above the overview').not.toBeNull();
  const x = box.x + box.width / 2, y = box.y + box.height / 2;
  await pres.mouse.move(x, y);
  await pres.mouse.down();
  await pres.mouse.move(x, y - 60, { steps: 4 });
  await pres.mouse.move(x, y - 120, { steps: 4 });
  await pres.mouse.up();
  const w1 = await width();
  expect(w1).toBeGreaterThan(w0 + 60);
  const stored = await pres.evaluate(() => window.__deckfold.settings().presenter.layout.strip);
  expect(Math.abs(stored - w1)).toBeLessThanOrEqual(1);
  await pres.reload();
  await pres.waitForSelector('.df-presenter');
  await expect.poll(width).toBeCloseTo(w1, 0);
  // dragged to the top, the overview stops at 45 % of the window height
  const b2 = await pres.locator('[data-df-split="strip"]').boundingBox();
  await pres.mouse.move(b2.x + b2.width / 2, b2.y + b2.height / 2);
  await pres.mouse.down();
  await pres.mouse.move(b2.x + b2.width / 2, 5, { steps: 6 });
  await pres.mouse.up();
  const share = await pres.evaluate(() => document.querySelector('.df-overview').offsetHeight / document.querySelector('.df-presenter').clientHeight);
  expect(share).toBeLessThanOrEqual(0.46);
  expect(await width()).toBeGreaterThan(w1);
});

// ---------- slides hidden from the show (E21) ----------

// The presenter view's strip leaves out the slides hidden from the show and follows every change on
// the start screen at once: the show window sends the list right after the click, not with its
// next heartbeat a second later.
test('hidden slides: the strip of the presenter view follows the eyes of the start screen at once', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  const n = await page.evaluate(() => window.__deckfold.slides().length);
  const hiddenTiles = () => pres.evaluate(() => [...document.querySelectorAll('.df-overview .df-tile')].filter((t) => getComputedStyle(t).display === 'none').map((t) => +t.getAttribute('data-i')));
  await expect.poll(hiddenTiles).toEqual([2]);
  const count = (shown) => page.evaluate(([n, shown]) => window.__deckfold.slideCount(n, shown), [n, shown]);
  await expect(pres.locator('.df-status [data-df-count]')).toHaveText(await count(n - 1));
  // each change shows in the presenter view within 300 ms, three times in a row; the heartbeat
  // comes once a second, so a view that waited for it would miss this most of the time. The time
  // runs from the click in the show window to the change of the strip in the presenter view, both
  // read in the windows by the same clock (Date.now), so that the way of the click through
  // Playwright does not count (it took over 300 ms on the Windows runner by itself)
  await page.evaluate(() => document.addEventListener('click', (e) => { if (e.target.closest('[data-df-eye]')) window.__eyeAt = Date.now(); }, true));
  await pres.evaluate(() => {
    window.__strip = [];
    const band = document.querySelector('.df-overview');
    new MutationObserver(() => window.__strip.push([Date.now(), [...band.querySelectorAll('.df-tile')].filter((t) => t.classList.contains('df-hidden')).map((t) => +t.getAttribute('data-i'))]))
      .observe(band, { subtree: true, attributes: true, attributeFilter: ['class'] });
  });
  const times = [];
  for (const [i, want] of [[1, [1, 2]], [1, [2]], [2, []]]) {
    await pres.evaluate(() => { window.__strip = []; });
    await page.click(`.df-strip [data-df-eye="${i}"]`);
    const at = await page.evaluate(() => window.__eyeAt);
    await expect.poll(() => hiddenTiles()).toEqual(want);
    const seen = await pres.evaluate((want) => window.__strip.find(([, h]) => JSON.stringify(h) === JSON.stringify(want)), want);
    times.push(seen[0] - at);
  }
  expect(Math.max(...times), `ms from the click in the show window to the strip of the presenter view: ${times}`).toBeLessThan(300);
  await expect(pres.locator('.df-status [data-df-count]')).toHaveText(await count(n));
});

// "next", "back", End and the preview skip the hidden slide; a jump by its number shows it.
test('hidden slides: next, back, End and the preview skip a hidden slide, its number shows it', async ({ page, context }) => {
  const pres = await present(page, context, 'pptx-basic.html');
  expect(await page.evaluate(() => window.__deckfold.hiddenSlides())).toEqual([2]);
  const playerSlide = () => page.evaluate(() => window.__dfPptx.state().slide);
  const nextPreview = () => pres.evaluate(() => {
    const f = document.querySelector('.df-next iframe[data-df-preview]');
    return f && f.contentDocument ? [...f.contentDocument.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on')) : -1;
  });
  // the last step of the second slide: "Preview" shows the fourth slide, the arrow is marked
  const end1 = await page.evaluate(() => window.__deckfold.model.end(1));
  await page.evaluate((s) => window.__deckfold.audience.act('state', s), end1);
  await expect.poll(() => counter(pres)).toContain('2 / 4');
  await expect.poll(nextPreview, { timeout: 10000 }).toBe(3);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
  await expect.poll(playerSlide).toBe(3);
  await pres.keyboard.press('ArrowLeft');
  await expect.poll(() => audState(page)).toEqual(end1);
  await pres.keyboard.press('Home');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['first']));
  await pres.keyboard.press('End');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
  // the number of the hidden slide and Enter show it; "next" goes on to the next shown one
  await pres.keyboard.press('3');
  await pres.keyboard.press('Enter');
  await expect.poll(() => audState(page)).toEqual({ slide: 2, step: 0 });
  await expect.poll(playerSlide).toBe(2);
  await pres.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual({ slide: 3, step: 0 });
});

// A chosen start slide that is hidden: "Live" shows the slide the show will begin on, and it
// begins there; the presenter view's arrows before the start skip hidden slides.
test('hidden slides: a hidden start slide, "Live" and the start', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.click('.df-strip .df-tile[data-i="1"]');
  await expect.poll(() => counter(pres)).toContain('2 / 4');
  // hiding the chosen slide: "Live" moves to the next shown one (the third is hidden too)
  await page.click('.df-strip [data-df-eye="1"]');
  await expect.poll(() => counter(pres)).toContain('4 / 4');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  // the presenter view's back arrow goes to the shown slide before the one it shows
  await pres.keyboard.press('ArrowLeft');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '0');
  await pres.keyboard.press('ArrowRight');
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '3');
  // the last slide chosen and hidden: the show begins on the shown slide before it
  await page.click('.df-strip [data-df-eye="3"]');
  await expect.poll(() => counter(pres)).toContain('1 / 4');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await expect.poll(() => audState(page)).toEqual({ slide: 0, step: 0 });
});

// Without a chosen slide (a new tab), the start slide is the first shown one, not slide 1.
test('hidden slides: with the first slide hidden, the start slide is the first shown one', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await page.click('.df-strip [data-df-eye="0"]');
  await page.evaluate(() => sessionStorage.clear());
  await page.goto('about:blank');
  await startScreen(page, 'pptx-basic.html');
  expect(await page.evaluate(() => window.__deckfold.hiddenSlides())).toEqual([0, 2]);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '1');
  await expect(page.locator('[data-df-hint]')).toHaveText('');
});

// Auto mode skips hidden slides too, also when it loops back to the first slide and that one is
// hidden.
test('hidden slides: auto mode with loop skips them, also on the way back to the first slide', async ({ page }) => {
  test.setTimeout(60_000);
  await startScreen(page, 'pptx-basic.html');
  await page.click('.df-strip [data-df-eye="0"]');
  expect(await page.evaluate(() => window.__deckfold.hiddenSlides())).toEqual([0, 2]);
  await page.check('[data-df-auto]');
  await page.locator('[data-df-interval]').fill('1');
  await page.locator('[data-df-interval]').dispatchEvent('change');
  await page.check('[data-df-loop]');
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience && window.__dfPptx && window.__dfPptx.state().slide === 1);
  await page.evaluate(() => {
    window.__seen = [];
    setInterval(() => window.__seen.push([window.__deckfold.audience.state().slide, window.__dfPptx.state().slide]), 50);
  });
  // through the fourth slide and back to the second
  await expect.poll(() => page.evaluate(() => {
    const s = window.__seen.map((x) => x[0]), i = s.indexOf(3);
    return i >= 0 && s.slice(i).includes(1);
  }), { timeout: 40_000 }).toBe(true);
  const seen = await page.evaluate(() => window.__seen);
  expect(seen.filter(([a, p]) => a === 0 || a === 2 || p === 0 || p === 2), 'states (show, player) on a hidden slide').toEqual([]);
});
