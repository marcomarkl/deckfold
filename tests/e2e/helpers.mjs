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

// Starts the solo audience view (second button on the start screen).
export async function solo(page, name) {
  await page.goto(fileURL(name));
  await page.waitForSelector('.df-start');
  await page.click('.df-start-buttons button:nth-child(2)');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
}

// Starts the presenter view with the audience window; returns the popup.
export async function present(page, context, name) {
  await page.goto(fileURL(name));
  await page.waitForSelector('.df-start');
  const popupP = context.waitForEvent('page');
  await page.click('.df-start-buttons button:nth-child(1)');
  const dlg = page.locator('.df-dialog .df-primary');
  await dlg.waitFor({ timeout: 3000 }).then(() => dlg.click()).catch(() => {});
  const aud = await popupP;
  await aud.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  return aud;
}

// Whether the page's browser can create a WebGL context (the Firefox of the Linux CI cannot).
export const hasWebGL = (page) => page.evaluate(() => {
  const c = document.createElement('canvas');
  return !!(c.getContext('webgl2') || c.getContext('webgl'));
});
export const NO_WEBGL = 'no WebGL in this browser; the fallback is checked by the transitions test';

export const audState = (aud) => aud.evaluate(() => window.__deckfold.audience.state());
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
