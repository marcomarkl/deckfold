// The start screen: its layout (title, preview, footer), the key help behind its button and the
// keys of the overlays. These tests run in every browser project, because the layout rests on CSS
// that the browsers could size differently (container units, a shrinking flex item with an aspect
// ratio).
import { test, expect } from '@playwright/test';
import { startScreen } from './helpers.mjs';

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
  // the fixed keys: Enter starts, digits + Enter choose a slide
  const acts = help.rows.map(([a]) => a);
  for (const a of ['start', 'goto']) expect(acts).toContain(a);
  // a click moves on: it stands in the row of "next", after its keys, and has no row of its own
  expect(acts).not.toContain('click');
  const next = await page.evaluate(() => {
    const D = window.__deckfold;
    return { caps: [...document.querySelectorAll('[data-df-keyhelp] [data-df-action="next"] kbd')].map((k) => k.textContent),
      want: D.keyCaps(D.settings().keys.next).concat([D.t('click')]) };
  });
  expect(next.caps).toEqual(next.want);
  expect(next.caps.length).toBeGreaterThan(1);
  // no group names an action twice
  const twice = await page.evaluate(() => [...document.querySelectorAll('[data-df-keyhelp] [data-df-keygroup]')].flatMap((g) => {
    const labels = [...g.querySelectorAll('.df-keylabel')].map((l) => l.textContent);
    return labels.filter((l, i) => labels.indexOf(l) !== i).map((l) => g.getAttribute('data-df-keygroup') + ': ' + l);
  }));
  expect(twice).toEqual([]);
});

// ---------- start screen: the footer and the key help behind its button ----------
const helpOpen = (page) => page.evaluate(() => { const o = document.querySelector('[data-df-help-overlay]'); return !!o && o.classList.contains('df-on'); });
const chosenTile = (page) => page.evaluate(() => { const t = document.querySelector('.df-start .df-strip .df-tile.df-cur'); return t ? +t.getAttribute('data-i') : -1; });

test('start screen: the title in the middle; the footer below the settings starts at the left with the key help button, the search and the number of slides', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  const m = await page.evaluate(() => {
    const box = (e) => { const r = e.getBoundingClientRect(); return { l: r.left, r: r.right, t: r.top, b: r.bottom, cx: (r.left + r.right) / 2, cy: (r.top + r.bottom) / 2 }; };
    const foot = document.querySelector('.df-start-foot');
    const q = (s) => foot && foot.querySelector(s);
    const find = q('[data-df-find]'), count = q('[data-df-count]'), help = q('[data-df-help]');
    if (!foot || !find || !count || !help) return { missing: { foot: !foot, find: !find, count: !count, help: !help } };
    return { find: box(find), count: box(count), help: box(help), foot: box(foot), col: box(document.querySelector('.df-start-box')), opts: box(document.querySelector('.df-opts')),
      start: box(document.querySelector('[data-df-start]')), n: +count.getAttribute('data-df-count'), slides: window.__deckfold.slides().length,
      text: count.textContent, sub: document.querySelectorAll('.df-start .df-sub').length, last: foot === foot.parentNode.lastElementChild,
      label: help.getAttribute('aria-label') || '', title: help.getAttribute('title') || '', icons: help.querySelectorAll('svg').length, helpText: help.textContent.trim(),
      h1: (() => { const r = document.createRange(); r.selectNodeContents(document.querySelector('.df-start h1')); return box(r); })(),
      order: [...foot.children].map((c) => ['data-df-help', 'data-df-find', 'data-df-count'].find((a) => c.hasAttribute(a)) || c.tagName) };
  });
  expect(m.missing, 'parts of the footer that are missing').toBeUndefined();
  // the text of the title stands in the middle of the column
  expect(Math.abs(m.h1.cx - m.col.cx)).toBeLessThanOrEqual(2);
  expect(m.h1.l - m.col.l).toBeGreaterThan(20);
  // one row, below the row with "Start", the last row of the column
  expect(m.foot.t).toBeGreaterThanOrEqual(m.start.b);
  expect(m.last).toBe(true);
  expect(Math.abs(m.find.cy - m.count.cy)).toBeLessThanOrEqual(2);
  expect(Math.abs(m.help.cy - m.count.cy)).toBeLessThanOrEqual(2);
  // the footer is as wide as the settings above it, and nothing stands below "Start"
  expect(Math.abs(m.foot.l - m.opts.l)).toBeLessThanOrEqual(1);
  expect(Math.abs(m.foot.r - m.opts.r)).toBeLessThanOrEqual(1);
  expect(m.foot.r).toBeLessThan(m.start.l);
  expect(Math.abs(m.opts.l - m.col.l)).toBeLessThanOrEqual(1);
  // everything at the left, in this order: the key help button at the left edge of the settings,
  // then the search, then the number of slides, close to each other
  expect(m.order).toEqual(['data-df-help', 'data-df-find', 'data-df-count']);
  expect(Math.abs(m.help.l - m.opts.l)).toBeLessThanOrEqual(1);
  expect(m.find.l - m.help.r).toBeLessThanOrEqual(16);
  expect(m.count.l - m.find.r).toBeLessThanOrEqual(16);
  expect(m.count.r).toBeLessThan(m.opts.cx);
  expect(m.help.r).toBeLessThan(m.find.l);
  expect(m.find.r).toBeLessThan(m.count.l);
  expect(m.n).toBe(m.slides);
  expect(m.text).toContain(String(m.slides));
  // the button shows an icon and no text; its name is in the title and for screen readers
  expect(m.icons).toBe(1);
  expect(m.helpText).toBe('');
  expect(m.label).not.toBe('');
  expect(m.title).toBe(m.label);
  // the line below the title is gone, and the key help is not on the start screen itself
  expect(m.sub).toBe(0);
  await expect(page.locator('[data-df-keyhelp]')).toBeHidden();
  await expect(page.locator('.df-start .df-ver')).toHaveCount(0);
});

test('start screen: the key help opens as an overlay, keeps the keys to itself and closes by Escape, its button and a click beside it', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  expect(await helpOpen(page)).toBe(false);
  await page.click('[data-df-help]');
  expect(await helpOpen(page)).toBe(true);
  await expect(page.locator('[data-df-help-overlay] [data-df-keyhelp] [data-df-keygroup]').first()).toBeVisible();
  await expect(page.locator('[data-df-help-overlay] [data-df-keyhelp] kbd').first()).toBeVisible();
  await expect(page.locator('[data-df-help-overlay] .df-ver')).toContainText('deckfold');
  // while it is open no key reaches the start screen: the arrows do not choose, Enter and Space do
  // not start, the search key does not open the search
  for (const k of ['ArrowRight', 'ArrowDown', 'Enter', ' ', '/', 'Control+f']) await page.keyboard.press(k);
  await page.waitForTimeout(400);
  expect(await chosenTile(page)).toBe(0);
  expect(await page.evaluate(() => !!window.__deckfold.audience)).toBe(false);
  await expect(page.locator('.df-start [data-df-start]')).toBeEnabled();
  expect(await page.evaluate(() => { const f = document.querySelector('[data-df-finder]'); return !!f && f.classList.contains('df-on'); })).toBe(false);
  expect(await helpOpen(page)).toBe(true);
  // Escape closes it and nothing else
  await page.keyboard.press('Escape');
  expect(await helpOpen(page)).toBe(false);
  await expect(page.locator('.df-start [data-df-start]')).toBeVisible();
  // its close button
  await page.click('[data-df-help]');
  expect(await helpOpen(page)).toBe(true);
  await page.click('[data-df-help-overlay] [data-df-close]');
  expect(await helpOpen(page)).toBe(false);
  // a click beside the box
  await page.click('[data-df-help]');
  expect(await helpOpen(page)).toBe(true);
  await page.mouse.click(5, 5);
  expect(await helpOpen(page)).toBe(false);
  // afterwards the keys choose again, and neither button keeps the focus
  expect(await page.evaluate(() => { const a = document.activeElement; return !!a && (a.hasAttribute('data-df-help') || a.hasAttribute('data-df-close')); })).toBe(false);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => chosenTile(page)).toBe(1);
});

test('start screen: the key help is gone once the presentation runs', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await page.click('[data-df-help]');
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-df-help-overlay]')).toHaveCount(1);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience && !document.querySelector('.df-start'));
  await expect(page.locator('[data-df-help-overlay]')).toHaveCount(0);
  await expect(page.locator('[data-df-keyhelp]')).toHaveCount(0);
});

test('start screen: it fits the window; the preview takes the height the other rows leave, up to 42 % of the window', async ({ page }) => {
  for (const [w, h] of [[1280, 800], [1920, 1080], [800, 500]]) {
    await page.setViewportSize({ width: w, height: h });
    await startScreen(page, 'pptx-basic.html');
    await page.waitForSelector('.df-start-stage iframe[data-df-preview]');
    await page.waitForTimeout(300);
    const m = await page.evaluate(() => {
      const ui = document.querySelector('.df-start');
      const f = document.querySelector('.df-start-stage iframe');
      const st = document.querySelector('.df-start-stage').getBoundingClientRect();
      const foot = document.querySelector('.df-start-foot');
      const top = document.querySelector('.df-start h1').getBoundingClientRect().top;
      ui.scrollTop = ui.scrollHeight;
      const footBottom = foot ? foot.getBoundingClientRect().bottom : -1;
      ui.scrollTop = 0;
      return { over: ui.scrollHeight - ui.clientHeight, wide: ui.scrollWidth - ui.clientWidth, top, footBottom,
        stage: [st.width, st.height], ratio: parseFloat(f.style.width) / parseFloat(f.style.height) };
    });
    const at = `${w}x${h}: ${JSON.stringify(m)}`;
    // the preview has the shape of the slide, so the scaled player leaves no bars
    expect(Math.abs(m.stage[0] - m.stage[1] * m.ratio), at).toBeLessThanOrEqual(1.5);
    expect(m.wide, at).toBeLessThanOrEqual(0);
    expect(m.top, at).toBeGreaterThanOrEqual(0);
    // the footer can be reached: scrolled to the end, it is in the window
    expect(m.footBottom, at).toBeGreaterThan(0);
    expect(m.footBottom, at).toBeLessThanOrEqual(h);
    // the same space below the footer as above the title, also at the end of a screen that scrolls
    expect(Math.abs((h - m.footBottom) - m.top), at).toBeLessThanOrEqual(1);
    if (h >= 800) {
      // nothing to scroll, and the preview uses its share of the window height
      expect(m.over, at).toBeLessThanOrEqual(0);
      expect(m.stage[1], at).toBeGreaterThan(0.4 * h);
      expect(m.stage[1], at).toBeLessThanOrEqual(0.42 * h + 1);
    } else {
      // a low window: the preview keeps a height that still shows the slide, and the screen scrolls
      expect(m.stage[1], at).toBeGreaterThanOrEqual(170);
    }
  }
});

test('start screen of an unknown format: the notice below the title, the footer with the key help only', async ({ page }) => {
  await startScreen(page, 'generic.html');
  await expect(page.locator('.df-start .df-sub')).toHaveCount(1);
  await expect(page.locator('.df-start .df-sub')).not.toBeEmpty();
  await expect(page.locator('[data-df-find]')).toHaveCount(0);
  await expect(page.locator('[data-df-count]')).toHaveCount(0);
  await expect(page.locator('.df-start-foot [data-df-help]')).toBeVisible();
  await expect(page.locator('[data-df-keyhelp]')).toBeHidden();
  await page.click('[data-df-help]');
  expect(await helpOpen(page)).toBe(true);
  await expect(page.locator('[data-df-help-overlay] [data-df-keygroup]')).toHaveCount(3);
  await page.keyboard.press('Escape');
  expect(await helpOpen(page)).toBe(false);
});

test('start screen: with the key help open, no key scrolls the screen behind it or opens the search of the browser', async ({ page }) => {
  // a window so low that the start screen scrolls and the button is reached only by scrolling
  await page.setViewportSize({ width: 800, height: 400 });
  await startScreen(page, 'pptx-basic.html');
  const scrolled = () => page.evaluate(() => Math.round(document.querySelector('.df-start').scrollTop));
  await page.click('[data-df-help]');
  expect(await helpOpen(page)).toBe(true);
  const at = await scrolled();
  expect(at).toBeGreaterThan(0);
  for (const k of ['PageUp', 'ArrowUp', 'Home', 'PageDown', 'ArrowDown', 'End']) await page.keyboard.press(k);
  await page.waitForTimeout(400);
  expect(await scrolled()).toBe(at);
  // what a key would do by itself is cancelled: the keys that scroll, and Ctrl+F and Cmd+F, which
  // would open the search of the browser; shortcuts of the browser such as reload stay
  const prevented = await page.evaluate(() => {
    const press = (init) => { const e = new KeyboardEvent('keydown', Object.assign({ bubbles: true, cancelable: true }, init)); document.body.dispatchEvent(e); return e.defaultPrevented; };
    return { pageDown: press({ key: 'PageDown' }), arrow: press({ key: 'ArrowDown' }), letter: press({ key: 'b' }),
      ctrlF: press({ key: 'f', ctrlKey: true }), cmdF: press({ key: 'f', metaKey: true }), reload: press({ key: 'r', metaKey: true }), ctrlReload: press({ key: 'r', ctrlKey: true }) };
  });
  expect(prevented).toEqual({ pageDown: true, arrow: true, letter: true, ctrlF: true, cmdF: true, reload: false, ctrlReload: false });
  expect(await helpOpen(page)).toBe(true);
});

test('overlays: Tab reaches the close button, and Enter or Space presses it', async ({ page }) => {
  await startScreen(page, 'pptx-content.html');
  const focused = () => page.evaluate(() => { const a = document.activeElement; return a ? ['data-df-close', 'data-df-find-input'].find((n) => a.hasAttribute(n)) || a.tagName : null; });
  const finderOpen = () => page.evaluate(() => { const f = document.querySelector('[data-df-finder]'); return !!f && f.classList.contains('df-on'); });
  // the search: the text field and the close button take turns
  await page.click('[data-df-find]');
  await page.keyboard.type('entry');
  await expect(page.locator('[data-df-finder] [data-df-find-item]')).toHaveCount(1);
  expect(await focused()).toBe('data-df-find-input');
  await page.keyboard.press('Tab');
  expect(await focused()).toBe('data-df-close');
  await page.keyboard.press('Tab');
  expect(await focused()).toBe('data-df-find-input');
  await page.keyboard.press('Shift+Tab');
  expect(await focused()).toBe('data-df-close');
  // on the close button Enter closes and does not choose the entry found (slide 2)
  await page.keyboard.press('Enter');
  expect(await finderOpen()).toBe(false);
  await page.waitForTimeout(300);
  expect(await chosenTile(page)).toBe(0);
  expect(await focused()).not.toBe('data-df-close');
  // the key help: its only stop is the close button; Space and Enter close and do not start
  for (const k of [' ', 'Enter']) {
    await page.click('[data-df-help]');
    expect(await helpOpen(page)).toBe(true);
    await page.keyboard.press('Tab');
    expect(await focused()).toBe('data-df-close');
    await page.keyboard.press(k);
    expect(await helpOpen(page)).toBe(false);
    await page.waitForTimeout(300);
    expect(await page.evaluate(() => !!window.__deckfold.audience)).toBe(false);
    await expect(page.locator('.df-start [data-df-start]')).toBeEnabled();
  }
});

test('start screen: a title of two lines and decks of other shapes fit as well', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await startScreen(page, 'pptx-basic.html');
  await page.waitForSelector('.df-start-stage iframe[data-df-preview]');
  const measure = () => page.evaluate(() => {
    const ui = document.querySelector('.df-start');
    const st = document.querySelector('.df-start-stage').getBoundingClientRect();
    const view = document.querySelector('.df-start-view').getBoundingClientRect();
    const h1 = document.querySelector('.df-start h1').getBoundingClientRect();
    return { over: ui.scrollHeight - ui.clientHeight, stage: [st.width, st.height], view: [view.width, view.height], title: h1.height,
      footBottom: document.querySelector('.df-start-foot').getBoundingClientRect().bottom };
  });
  const one = (await measure()).title;
  // a long title wraps; the preview gives the height, and nothing has to be scrolled
  await page.evaluate(() => { document.querySelector('.df-start h1').textContent = 'A title that is long enough to need a second line on the start screen '.repeat(2); });
  await page.waitForTimeout(300);
  let m = await measure();
  expect(m.title, JSON.stringify(m)).toBeGreaterThan(one * 1.5);
  expect(m.over, JSON.stringify(m)).toBeLessThanOrEqual(0);
  expect(m.stage[1], JSON.stringify(m)).toBeGreaterThanOrEqual(180);
  expect(m.footBottom, JSON.stringify(m)).toBeLessThanOrEqual(800);
  // other shapes of the slide (width by height): 4:3, portrait, a wide banner
  for (const ar of [4 / 3, 9 / 16, 3]) {
    await page.evaluate((ar) => document.querySelector('.df-start-view').style.setProperty('--df-ar', String(ar)), ar);
    await page.waitForTimeout(300);
    m = await measure();
    const at = `${ar}: ${JSON.stringify(m)}`;
    expect(Math.abs(m.stage[0] - m.stage[1] * ar), at).toBeLessThanOrEqual(1.5);
    expect(m.stage[0], at).toBeLessThanOrEqual(m.view[0] + 0.5);
    expect(m.stage[1], at).toBeLessThanOrEqual(m.view[1] + 0.5);
    expect(m.stage[1], at).toBeGreaterThanOrEqual(180);
    expect(m.over, at).toBeLessThanOrEqual(0);
    expect(m.footBottom, at).toBeLessThanOrEqual(800);
  }
});
