// One-shot blob URLs (scripts, XHR, fetch, worker code) are released after use; URLs that the DOM
// still references are not.
import { test, expect } from '@playwright/test';
import { watch, realErrors, solo, present, audState, expected, need } from './helpers.mjs';

test.beforeEach(() => need('keynote'));

// Counts created and revoked blob URLs in every frame (including the srcdoc previews).
async function countBlobs(context) {
  await context.addInitScript(() => {
    const created = [], revoked = [];
    const c = URL.createObjectURL.bind(URL), r = URL.revokeObjectURL.bind(URL);
    URL.createObjectURL = (o) => { const u = c(o); created.push(u); return u; };
    URL.revokeObjectURL = (u) => { revoked.push(u); return r(u); };
    window.__blobs = { created, revoked };
  });
}

test('Keynote solo: one-shot URLs are released, DOM-bound URLs stay', async ({ page, context }) => {
  await countBlobs(context);
  const w = watch(context);
  await solo(page, 'keynote.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 15000 });
  await page.waitForTimeout(1500);
  for (let i = 0; i < 6; i++) { await page.keyboard.press('ArrowRight'); await page.waitForTimeout(700); }
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next', 'next', 'next', 'next', 'next', 'next']));
  const r = await page.evaluate(() => {
    const { created, revoked } = window.__blobs;
    const inDom = [...document.querySelectorAll('img, link, video, audio, source')]
      .map((e) => e.getAttribute('src') || e.getAttribute('href') || '')
      .filter((u) => u.startsWith('blob:'));
    return { created: created.length, revoked: revoked.length, unknown: revoked.filter((u) => !created.includes(u)).length, domRevoked: inDom.filter((u) => revoked.includes(u)).length };
  });
  console.log(`created ${r.created}, revoked ${r.revoked}`);
  expect(r.revoked).toBeGreaterThan(0);
  expect(r.unknown).toBe(0);
  expect(r.domRevoked).toBe(0);
  expect(w.net).toEqual([]);
  expect(realErrors(w)).toEqual([]);
});

test('Keynote presenter view: previews keep rendering with released URLs', async ({ page, context }) => {
  await countBlobs(context);
  const w = watch(context);
  const pres = await present(page, context, 'keynote.html');
  const canvases = () => pres.evaluate(() => [...document.querySelectorAll('iframe[data-df-preview]')].map((f) => f.contentDocument.querySelectorAll('#stage canvas').length));
  await expect.poll(async () => (await canvases()).every((c) => c > 0), { timeout: 20000 }).toBe(true);
  for (let i = 0; i < 3; i++) { await pres.keyboard.press('ArrowRight'); await pres.waitForTimeout(800); }
  await expect.poll(() => audState(page)).toEqual(await expected(pres, ['next', 'next', 'next']));
  await pres.waitForTimeout(1500);
  expect((await canvases()).every((c) => c > 0)).toBe(true);
  expect(await pres.evaluate(() => window.__blobs.revoked.length)).toBeGreaterThan(0);
  expect(realErrors(w)).toEqual([]);
});
