// Fidelity of PowerPoint decks: every slide of a feature deck, shown in the player's static mode
// (all objects visible, like PowerPoint's editing view), is compared by SSIM with the PNG that
// PowerPoint exported (fixtures/pptx/ref/<name>/<n>.png, made by scripts/pptxref/pptxref.sh).
// Decks without reference images are skipped. See docs/spikes/S4-powerpoint-reference.md.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from '@playwright/test';
import { PNG } from 'pngjs';
import { ROOT, OUT, fileURL } from './helpers.mjs';

// Starting value from the plan; spike S4 calibrates it.
const THRESHOLD = 0.95;
// Decks with a threshold of their own, with the reason.
const DECK_THRESHOLD = {
  // Every slide of the kerning deck is full of text, down to 10 pt, and SSIM follows how a
  // browser draws letters more closely there than anywhere else. Chrome on the Linux and Windows
  // runners is up to 0.017 below Chrome on the Mac for these slides, with and without kerning
  // (slide 5 has no kerned line and loses as much as slide 4), while the other 90 slides differ
  // by 0.0006 on average. Measured on 2026-11-10: 0.946 to 0.985 on the Mac, 0.935 to 0.977 on
  // the runners. Where the words and the baselines stand is checked against PowerPoint's own
  // numbers (the kerning test in pptx.spec.mjs, internal/pptx/kerning_test.go and
  // baseline_test.go); this comparison still catches the deck drawn another way: with the
  // kerning switched off, slides 1, 2 and 4 are at 0.828, 0.853 and 0.828, and with the
  // baselines of before slide 1 was at 0.828 as well.
  kerning: 0.92,
};
// Slides allowed below the threshold, with the reason: { 'deck/3': 'reason' }.
const EXCEPTIONS = {
  // Values, axis scale, legend and colors match PowerPoint, but PowerPoint lays out the plot area
  // of an automatic layout a few pixels differently; the offset of the many one-pixel gridlines
  // and tick marks dominates SSIM (measured 0.68 on 2026-11-07).
  'charts/1': 'automatic plot area layout',
  // The same for the gridlines of the area chart (0.92); the pie and the doughnut match.
  'charts/2': 'automatic plot area layout',
  // The embedded font is used and its glyphs match, but PowerPoint places the baselines of this
  // embedded font 1 pt higher than the metrics of the font give, which it does not do for an
  // installed Liberation Serif with the same metrics (measured 0.924 on 2026-11-10).
  'fontembed/1': 'baseline of the embedded font',
};
// The charts of charts2 (scatter, bubble, radar, stock, pie and bar of pie, surface, and the 3D
// charts) show the right data, axes and colors, but PowerPoint sets chart text without c:txPr in
// another font and lays out their plot areas automatically; the 3D plots are approximations
// (measured 0.74 to 0.90 on 2026-11-07).
for (let i = 1; i <= 12; i++) EXCEPTIONS[`charts2/${i}`] = 'chart text font and automatic layout; 3D approximation';

const fixtures = path.join(ROOT, 'fixtures', 'pptx');
const decks = fs.readdirSync(path.join(fixtures, 'src')).filter((f) => f.endsWith('.pptx')).map((f) => f.slice(0, -5));

// SSIM of two equally sized RGBA images, on luminance, in 8 x 8 windows.
export function ssim(a, b, w, h) {
  const lum = (d, i) => 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2];
  const C1 = (0.01 * 255) ** 2, C2 = (0.03 * 255) ** 2;
  let sum = 0, n = 0;
  for (let y = 0; y + 8 <= h; y += 8) {
    for (let x = 0; x + 8 <= w; x += 8) {
      let ma = 0, mb = 0;
      for (let j = 0; j < 8; j++) for (let i = 0; i < 8; i++) { const k = ((y + j) * w + x + i) * 4; ma += lum(a, k); mb += lum(b, k); }
      ma /= 64; mb /= 64;
      let va = 0, vb = 0, cov = 0;
      for (let j = 0; j < 8; j++) for (let i = 0; i < 8; i++) {
        const k = ((y + j) * w + x + i) * 4, da = lum(a, k) - ma, db = lum(b, k) - mb;
        va += da * da; vb += db * db; cov += da * db;
      }
      va /= 63; vb /= 63; cov /= 63;
      sum += ((2 * ma * mb + C1) * (2 * cov + C2)) / ((ma * ma + mb * mb + C1) * (va + vb + C2));
      n++;
    }
  }
  return n ? sum / n : 1;
}

for (const name of decks) {
  test(`PowerPoint fidelity: ${name}`, async ({ page }) => {
    const ref = path.join(fixtures, 'ref', name);
    const saved = path.join(fixtures, name + '.pptx');
    test.skip(!fs.existsSync(path.join(ref, 'source.sha256')) || !fs.existsSync(saved), `no PowerPoint reference for ${name} yet (run scripts/pptxref/pptxref.sh)`);
    const want = fs.readFileSync(path.join(ref, 'source.sha256'), 'utf8').trim();
    const have = crypto.createHash('sha256').update(fs.readFileSync(saved)).digest('hex');
    expect(have, `${name}.pptx changed after its reference images were made; run scripts/pptxref/pptxref.sh ${name}`).toBe(want);
    const out = path.join(OUT, `pptx-ref-${name}.html`);
    expect(fs.existsSync(out), 'setup converts every deck with a reference').toBe(true);
    const images = fs.readdirSync(ref).filter((f) => f.endsWith('.png')).sort((x, y) => parseInt(x) - parseInt(y));
    const first = PNG.sync.read(fs.readFileSync(path.join(ref, images[0])));
    await page.setViewportSize({ width: first.width, height: first.height });
    await page.goto(fileURL(path.basename(out)));
    await page.click('[data-df-start]');
    await page.waitForFunction(() => window.__dfPptx);
    await page.evaluate(() => document.fonts.ready);
    const n = await page.evaluate(() => window.__dfPptx.deck.slides.length);
    expect(n, 'one reference image per visible slide').toBe(images.length);
    const threshold = DECK_THRESHOLD[name] ?? THRESHOLD;
    // every slide is measured before the test fails, so that a run in CI shows the value of
    // each slide of a deck and not only of the first one below the threshold
    const low = [];
    for (let i = 0; i < n; i++) {
      // the slide through deckfold first (it brings a player that went elsewhere back to its own
      // slide), then every object of it shown
      await page.evaluate((i) => window.__deckfold.audience.act('state', { slide: i, step: 0 }), i);
      await page.waitForFunction((i) => window.__dfPptx.state().slide === i, i);
      await page.evaluate((i) => window.__dfPptx.showAll(i), i);
      await page.waitForTimeout(100);
      const shot = PNG.sync.read(await page.screenshot());
      const refImg = PNG.sync.read(fs.readFileSync(path.join(ref, images[i])));
      expect([shot.width, shot.height]).toEqual([refImg.width, refImg.height]);
      const score = ssim(shot.data, refImg.data, shot.width, shot.height);
      const key = `${name}/${i + 1}`;
      console.log(`${key}: SSIM ${score.toFixed(3)}`);
      if (score < threshold && !EXCEPTIONS[key]) {
        fs.writeFileSync(path.join(OUT, `pptx-diff-${name}-${i + 1}.png`), PNG.sync.write(shot));
      }
      if (!EXCEPTIONS[key] && score < threshold) low.push(`${key}: SSIM ${score.toFixed(3)} (rendering saved as tests/.out/pptx-diff-${name}-${i + 1}.png)`);
    }
    expect(low, `slides below ${threshold}`).toEqual([]);
  });
}
