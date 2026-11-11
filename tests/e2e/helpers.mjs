import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from '@playwright/test';

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
export const counter = (page) => page.textContent('.df-counter');
export const slideCount = (page) => page.evaluate(() => window.__deckfold.slides().length);

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
