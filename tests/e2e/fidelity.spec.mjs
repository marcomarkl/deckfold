// Fidelity to the source: pixel comparison of every slide and an objective comparison of the
// running animations between the original (served by a local server) and the deckfold output (file://).
import { test, expect } from '@playwright/test';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { PNG } from 'pngjs';
import pixelmatch from 'pixelmatch';
import { ROOT, OUT, fileURL, corpus, need } from './helpers.mjs';

let server, base;
test.beforeAll(async () => {
  const root = ROOT; // corpus paths are relative to the repository
  server = http.createServer((req, res) => {
    const p = path.join(root, decodeURIComponent(new URL(req.url, 'http://x').pathname));
    if (!p.startsWith(root) || !fs.existsSync(p) || fs.statSync(p).isDirectory()) { res.writeHead(404); return res.end(); }
    const ext = path.extname(p);
    res.writeHead(200, { 'Content-Type': { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.json': 'application/json', '.pdf': 'application/pdf', '.png': 'image/png', '.jpeg': 'image/jpeg' }[ext] || 'application/octet-stream' });
    fs.createReadStream(p).pipe(res);
  });
  await new Promise((r) => server.listen(0, '127.0.0.1', r));
  base = 'http://127.0.0.1:' + server.address().port + '/';
});
test.afterAll(() => server && server.close());

const toURL = (rel) => base + rel.split(path.sep).map(encodeURIComponent).join('/');

function diffRatio(a, b, name) {
  const A = PNG.sync.read(a), B = PNG.sync.read(b);
  const out = new PNG({ width: A.width, height: A.height });
  const n = pixelmatch(A.data, B.data, out.data, A.width, A.height, { threshold: 0.1 });
  if (n > 0) fs.writeFileSync(path.join(OUT, 'diff-' + name + '.png'), PNG.sync.write(out));
  return n / (A.width * A.height);
}

// Records the animations shortly after a key press.
async function animationsAfterKey(page, key) {
  await page.evaluate(() => {
    window.__anim = null;
    const grab = () => {
      window.__anim = document.getAnimations().map((a) => {
        const e = a.effect, t = e && e.target;
        const tgt = t ? (t.tagName + '.' + (typeof t.className === 'string' ? t.className.split(' ').filter((c) => !c.startsWith('df-')).sort().join('.') : '') + '[' + (t.getAttribute('data-frame-id') || '') + ']') : '';
        const tm = e ? e.getTiming() : {};
        const kf = e && e.getKeyframes ? e.getKeyframes().map((k) => { const o = { ...k }; delete o.computedOffset; return o; }) : [];
        return JSON.stringify({ type: a.constructor.name, name: a.animationName || a.transitionProperty || '', tgt, d: tm.duration, ease: tm.easing, fill: tm.fill, kf });
      }).sort();
    };
    window.addEventListener('keydown', () => setTimeout(grab, 40), { once: true });
  });
  await page.keyboard.press(key);
  await page.waitForFunction(() => window.__anim !== null);
  return page.evaluate(() => window.__anim);
}

test('claude.ai: slides are pixel-identical and animations match', async ({ browser }) => {
  need('claude');
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const orig = await ctx.newPage();
  await orig.goto(toURL(corpus.claude));
  await orig.evaluate(() => document.fonts.ready);
  const out = await ctx.newPage();
  await out.goto(fileURL('claude.html'));
  await out.click('.df-start-buttons button:nth-child(2)');
  await out.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await out.evaluate(() => document.fonts.ready);
  await out.waitForTimeout(2000); await orig.waitForTimeout(2000);
  const states = await out.evaluate(() => { let s = window.__deckfold.model.first(), n = 0; while (s) { n++; s = window.__deckfold.model.next(s); } return n; });
  let worst = 0, animCompared = 0;
  for (let i = 0; i < states; i++) {
    const r = diffRatio(await orig.screenshot(), await out.screenshot(), 'claude-' + (i + 1));
    worst = Math.max(worst, r);
    expect(r, `state ${i + 1}: share of differing pixels`).toBeLessThan(0.002);
    if (i < states - 1) {
      const [ao, bo] = [await animationsAfterKey(orig, 'ArrowRight'), await animationsAfterKey(out, 'ArrowRight')];
      expect(bo, `animations on the way to state ${i + 2}`).toEqual(ao);
      animCompared += ao.length;
      await orig.waitForTimeout(2200); await out.waitForTimeout(2200);
    }
  }
  console.log(`claude.ai: largest difference ${(worst * 100).toFixed(3)} %, ${animCompared} animations compared over ${states} states`);
  expect(animCompared).toBeGreaterThan(0);
  await ctx.close();
});

test('Keynote: slides are pixel-identical with the original export', async ({ browser }) => {
  need('keynote');
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const orig = await ctx.newPage();
  await orig.goto(toURL(path.join(corpus.keynote, 'index.html')));
  const out = await ctx.newPage();
  await out.goto(fileURL('keynote.html'));
  await out.click('.df-start-buttons button:nth-child(2)');
  await out.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await orig.waitForTimeout(3000); await out.waitForTimeout(3000);
  const n = Math.min(8, await out.evaluate(() => window.__deckfold.slides().length));
  let worst = 0;
  for (let i = 0; i < n; i++) {
    const r = diffRatio(await orig.screenshot(), await out.screenshot(), 'keynote-' + (i + 1));
    worst = Math.max(worst, r);
    expect(r, `step ${i + 1}`).toBeLessThan(0.002);
    await orig.keyboard.press('ArrowRight'); await out.keyboard.press('ArrowRight');
    await orig.waitForTimeout(2500); await out.waitForTimeout(2500);
  }
  console.log(`Keynote: largest difference ${(worst * 100).toFixed(3)} % over ${n} steps`);
  await ctx.close();
});
