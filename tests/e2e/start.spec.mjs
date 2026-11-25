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
      findLabel: find.getAttribute('aria-label') || '', findTitle: find.getAttribute('title') || '', findIcons: find.querySelectorAll('svg').length, findText: find.textContent.trim(),
      search: window.__deckfold.t('search'), countText: window.__deckfold.slideCount(window.__deckfold.slides().length, window.__deckfold.slides().length - window.__deckfold.hiddenSlides().length),
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
  expect(m.text).toBe(m.countText);
  // the search is an icon, like the key help, with its name as tooltip
  expect(m.findIcons).toBe(1);
  expect(m.findText).toBe('');
  expect(m.findLabel).toBe(m.search);
  expect(m.findTitle).toBe(m.search);
  expect(Math.abs((m.find.r - m.find.l) - (m.help.r - m.help.l))).toBeLessThanOrEqual(1);
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

// A browser without container units ignores the @supports block of the start screen and applies
// the rules before it. No test browser is that old, and none lets the units be switched off, so
// the test takes the block out of the loaded style sheet: what is left is what such a browser
// applies. It does not show what else such a browser lacks.
test('start screen without container units: the preview keeps its share of the window height and stays in the column', async ({ page }) => {
  for (const [w, h] of [[1280, 800], [1920, 1080], [800, 500], [360, 700]]) {
    await page.setViewportSize({ width: w, height: h });
    await startScreen(page, 'pptx-basic.html');
    await page.waitForSelector('.df-start-stage iframe[data-df-preview]');
    const removed = await page.evaluate(() => {
      const sheet = document.getElementById('df-style').sheet;
      let n = 0;
      for (let i = sheet.cssRules.length - 1; i >= 0; i--) {
        const r = sheet.cssRules[i];
        if (r instanceof CSSSupportsRule && /cqh/.test(r.conditionText)) { sheet.deleteRule(i); n++; }
      }
      return n;
    });
    expect(removed, 'the @supports block for container units').toBe(1);
    await page.waitForTimeout(300);
    const m = await page.evaluate(() => {
      const ui = document.querySelector('.df-start');
      const box = (e) => { const r = e.getBoundingClientRect(); return { l: r.left, r: r.right, t: r.top, b: r.bottom, w: r.width, h: r.height }; };
      const f = document.querySelector('.df-start-stage iframe');
      const top = document.querySelector('.df-start h1').getBoundingClientRect().top;
      const out = { stage: box(document.querySelector('.df-start-stage')), col: box(document.querySelector('.df-start-box')), strip: box(document.querySelector('.df-strip-wrap')),
        over: ui.scrollHeight - ui.clientHeight, wide: ui.scrollWidth - ui.clientWidth, top, ratio: parseFloat(f.style.width) / parseFloat(f.style.height) };
      ui.scrollTop = ui.scrollHeight;
      out.footBottom = document.querySelector('.df-start-foot').getBoundingClientRect().bottom;
      ui.scrollTop = 0;
      return out;
    });
    const at = `${w}x${h}: ${JSON.stringify(m)}`;
    // the fixed share of the window height, inside the column and above the strip of slides
    expect(Math.abs(m.stage.h - Math.min(0.42 * h, 520)), at).toBeLessThanOrEqual(1);
    expect(m.stage.l, at).toBeGreaterThanOrEqual(m.col.l - 0.5);
    expect(m.stage.r, at).toBeLessThanOrEqual(m.col.r + 0.5);
    expect(m.stage.b, at).toBeLessThanOrEqual(m.strip.t + 0.5);
    expect(m.wide, at).toBeLessThanOrEqual(0);
    expect(m.top, at).toBeGreaterThanOrEqual(0);
    expect(m.footBottom, at).toBeLessThanOrEqual(h);
    // where the column is wide enough the preview has the shape of the slide; in a narrow window it
    // is as wide as the column, and the scaled player stands in its middle
    if (m.stage.h * m.ratio <= m.col.w) expect(Math.abs(m.stage.w - m.stage.h * m.ratio), at).toBeLessThanOrEqual(1.5);
    else expect(Math.abs(m.stage.w - m.col.w), at).toBeLessThanOrEqual(1);
    if (w >= 1280) expect(m.over, at).toBeLessThanOrEqual(0);
  }
});

// E21: an eye at the right of the number of every tile hides the slide from the show and shows it
// again. The deck's own choice (the third slide of the basic deck is hidden in PowerPoint) comes
// first; a change is stored for the deck, the count says how many slides the show has, and the
// last shown slide cannot be hidden.
test('start screen: the eye of a tile hides the slide from the show and shows it again', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  const n = await page.evaluate(() => window.__deckfold.slides().length);
  const eyes = () => page.evaluate(() => [...document.querySelectorAll('.df-strip [data-df-eye]')].map((b) => {
    const wrap = b.closest('.df-tile-wrap'), tile = wrap.querySelector('.df-tile'), num = tile.querySelector('span');
    const r = b.getBoundingClientRect(), t = tile.getBoundingClientRect(), p = tile.querySelector('.df-thumb').getBoundingClientRect(), s = num.getBoundingClientRect();
    return { i: +b.getAttribute('data-df-eye'), svg: b.querySelectorAll('svg').length, text: b.textContent.trim(), label: b.getAttribute('aria-label'),
      title: b.getAttribute('title'), pressed: b.getAttribute('aria-pressed'), off: wrap.classList.contains('df-off'), disabled: b.disabled,
      // at the right of the slide number, below the picture, inside the tile
      place: r.top >= p.bottom - 1 && r.bottom <= t.bottom + 1 && r.right <= t.right + 1 && r.left >= s.left && t.right - r.right < 12 &&
        r.top < s.bottom && r.bottom > s.top };
  }));
  const t = (k) => page.evaluate((k) => window.__deckfold.t(k), k);
  const count = (shown) => page.evaluate(([n, shown]) => window.__deckfold.slideCount(n, shown), [n, shown]);
  const stored = () => page.evaluate(() => window.__deckfold.settings().hidden);
  let e = await eyes();
  expect(e.length).toBe(n);
  for (const x of e) {
    expect(x.svg, `eye ${x.i}`).toBe(1);
    expect(x.text, `eye ${x.i}`).toBe('');
    expect(x.place, `eye ${x.i} at the right of the slide number`).toBe(true);
    expect(x.label, `eye ${x.i}`).toBe(await t(x.i === 2 ? 'showSlide' : 'hideSlide'));
    expect(x.title, `eye ${x.i}`).toBe(x.label);
    expect([x.pressed, x.off], `eye ${x.i}`).toEqual(x.i === 2 ? ['true', true] : ['false', false]);
  }
  await expect(page.locator('[data-df-count]')).toHaveText(await count(n - 1));
  expect(await stored()).toBeUndefined();
  // hiding slide 2: the start slide stays, the choice is stored, and it survives a reload
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '0');
  await page.click('.df-strip [data-df-eye="1"]');
  e = await eyes();
  expect([e[1].pressed, e[1].off, e[1].label]).toEqual(['true', true, await t('showSlide')]);
  await expect(page.locator('.df-strip .df-tile.df-cur')).toHaveAttribute('data-i', '0');
  await expect(page.locator('[data-df-count]')).toHaveText(await count(n - 2));
  expect(await stored()).toEqual([1, 2]);
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  expect((await eyes()).map((x) => x.off)).toEqual([false, true, true, false]);
  await expect(page.locator('[data-df-count]')).toHaveText(await count(n - 2));
  // the last shown slide cannot be hidden
  await page.click('.df-strip [data-df-eye="3"]');
  e = await eyes();
  expect(e.map((x) => [x.off, x.disabled])).toEqual([[false, true], [true, false], [true, false], [true, false]]);
  await page.click('.df-strip [data-df-eye="0"]', { force: true });
  expect((await eyes())[0].off).toBe(false);
  // showing them again: back to the deck's own choice, which is not stored
  await page.click('.df-strip [data-df-eye="3"]');
  await page.click('.df-strip [data-df-eye="1"]');
  expect((await eyes()).map((x) => x.off)).toEqual([false, false, true, false]);
  expect(await stored()).toBeUndefined();
  await expect(page.locator('[data-df-count]')).toHaveText(await count(n - 1));
  // without hidden slides the count is the plain number
  await page.click('.df-strip [data-df-eye="2"]');
  await expect(page.locator('[data-df-count]')).toHaveText(await count(n));
  expect(await page.locator('[data-df-count]').textContent()).toBe(await page.evaluate((n) => window.__deckfold.slideCount(n), n));
  expect(await stored()).toEqual([]);
});

// E21: a chosen start slide that is hidden from the show is not where the show begins; the line
// beside "Start" says where it does.
test('start screen: a hidden start slide is named in the hint, with the slide the show starts on', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  const hint = page.locator('[data-df-hint]');
  await expect(hint).toHaveText('');
  // the third slide is hidden in the deck: chosen, the show starts on the fourth
  await page.click('.df-strip .df-tile[data-i="2"]');
  const want = await page.evaluate(() => window.__deckfold.t('startsAt') + ' ' + (window.__deckfold.model.startAt(2).slide + 1));
  expect(want).toMatch(/ 4$/);
  await expect(hint).toHaveText(want);
  // shown again, the hint goes
  await page.click('.df-strip [data-df-eye="2"]');
  await expect(hint).toHaveText('');
  // the fourth slide chosen and hidden: no slide after it, the show starts on the second
  await page.click('.df-strip [data-df-eye="2"]');
  await page.click('.df-strip .df-tile[data-i="3"]');
  await page.click('.df-strip [data-df-eye="3"]');
  await expect(hint).toHaveText(await page.evaluate(() => window.__deckfold.t('startsAt') + ' 2'));
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold.audience);
  await expect.poll(() => page.evaluate(() => window.__deckfold.audience.state())).toEqual({ slide: 1, step: 0 });
});

// The start screen puts the focus on no control when it loads: a focused button shows its focus
// ring from the first frame (seen in Safari, on an iPad and in Chrome), as if it had been pressed.
// Enter starts all the same, since the keys are taken on the window, whatever has the focus.
test('start screen: no control has the focus after loading, and Enter still starts', async ({ page }) => {
  await startScreen(page, 'pptx-basic.html');
  await page.waitForTimeout(400);
  const ringed = () => page.evaluate(() => [...document.querySelectorAll('[data-df-ui] button, [data-df-ui] input')]
    .filter((c) => c === document.activeElement || c.matches(':focus-visible')).map((c) => c.outerHTML.slice(0, 60)));
  expect(await ringed()).toEqual([]);
  await page.uncheck('[data-df-fs]');
  await page.keyboard.press('Enter');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  // back on the start screen after the end: again nothing has the focus
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  await page.waitForTimeout(400);
  expect(await ringed()).toEqual([]);
});
