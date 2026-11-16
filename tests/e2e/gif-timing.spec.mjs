// An animated GIF and the WebP file deckfold makes of it play at the same speed (decision E17).
// The page comes from scripts/giftiming: for each frame duration a GIF of three frames, red, green
// and blue, and its WebP file.
//
// A canvas cannot measure this: drawImage gives the first frame of an animation in every browser.
// So the test takes screenshots. One screenshot shows all pictures at the same moment, and the
// test looks at how far one picture is ahead of the other, counted in frames. Two animations of
// the same speed keep their distance, whatever it is; two of different speeds drift through all
// three distances.
import { test, expect } from '@playwright/test';
import { PNG } from 'pngjs';
import { fileURL } from './helpers.mjs';

const DELAYS = [500, 200, 100, 50, 20];

// Takes screenshots for about `ms` and returns, per screenshot, the time and the colour index
// (0 red, 1 green, 2 blue) in the middle of every picture, keyed "gif-200", "webp-200", ...
async function sample(page, ms) {
  // all animations start in one task, from the first frame
  await page.evaluate(async () => {
    const imgs = [...document.querySelectorAll('img')];
    const urls = await Promise.all(imgs.map(async (im) => URL.createObjectURL(await (await fetch(im.src)).blob())));
    imgs.forEach((im) => im.removeAttribute('src'));
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    imgs.forEach((im, i) => { im.src = urls[i]; });
    await Promise.all(imgs.map((im) => im.decode()));
  });
  const boxes = await page.evaluate(() => [...document.querySelectorAll('img')].map((im) => {
    const b = im.getBoundingClientRect();
    return { key: `${im.dataset.kind}-${im.dataset.ms}`, x: Math.round(b.x + b.width / 2), y: Math.round(b.y + b.height / 2) };
  }));
  const shots = [];
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    const before = Date.now();
    const buf = await page.screenshot();
    shots.push({ t: (before + Date.now()) / 2 - t0, buf });
  }
  return shots.map(({ t, buf }) => {
    const png = PNG.sync.read(buf);
    const scale = png.width / page.viewportSize().width;
    const at = {};
    for (const b of boxes) {
      const i = (Math.round(b.y * scale) * png.width + Math.round(b.x * scale)) * 4;
      const [r, g, bl] = [png.data[i], png.data[i + 1], png.data[i + 2]];
      at[b.key] = r > 150 && g < 100 ? 0 : g > 120 && r < 100 ? 1 : bl > 150 ? 2 : -1;
    }
    return { t, at };
  });
}

// How often picture b is 0, 1 or 2 frames ahead of picture a, as shares of the screenshots.
function distances(samples, a, b) {
  const n = [0, 0, 0];
  let all = 0;
  for (const s of samples) {
    if (s.at[a] < 0 || s.at[b] < 0) continue;
    n[(s.at[b] - s.at[a] + 3) % 3]++;
    all++;
  }
  return n.map((k) => Math.round(k / all * 100) / 100);
}

// The average time a colour stays, from the first change to the last one seen.
function meanState(samples, key) {
  const changes = [];
  let last = samples[0].at[key];
  for (const s of samples) if (s.at[key] !== last) { changes.push(s.t); last = s.at[key]; }
  return changes.length > 1 ? Math.round((changes[changes.length - 1] - changes[0]) / (changes.length - 1)) : 0;
}

test('GIF and WebP: an animation plays at the same speed in both formats', async ({ page }) => {
  await page.goto(fileURL('giftiming/index.html'));
  const samples = await sample(page, 8000);
  const seen = {};
  for (const ms of DELAYS) {
    seen[ms] = { pair: distances(samples, `gif-${ms}`, `webp-${ms}`), gif: meanState(samples, `gif-${ms}`), webp: meanState(samples, `webp-${ms}`) };
  }
  const control = { gif: distances(samples, 'gif-100', 'gif-200'), webp: distances(samples, 'webp-100', 'webp-200'), mixed: distances(samples, 'gif-50', 'webp-100') };
  const said = `${samples.length} screenshots; pairs ${JSON.stringify(seen)}; pictures of different speeds ${JSON.stringify(control)}`;
  console.log(`[${test.info().project.name}] ${said}`); // the measured values, for the log of the run
  // The method tells two speeds apart: pictures of different speeds are found at every distance.
  // Without this the comparison below would show nothing.
  for (const d of Object.values(control)) expect(Math.min(...d), said).toBeGreaterThan(0.15);
  // The slow pictures change as often as their files say. Screenshots can show that only where
  // one takes much less time than a frame lasts: with a screenshot every 150 ms, as on the Windows
  // runner, changes of a picture with frames of 200 ms are missed and it measures as slower.
  const step = 8000 / samples.length;
  for (const ms of [500, 200]) {
    expect(seen[ms].gif, said).toBeGreaterThan(0);
    expect(seen[ms].webp, said).toBeGreaterThan(0);
    if (step > ms / 3) continue;
    expect(Math.abs(seen[ms].gif - ms), said).toBeLessThan(ms * 0.1);
    expect(Math.abs(seen[ms].webp - ms), said).toBeLessThan(ms * 0.1);
  }
  // A GIF and its WebP file keep their distance: one of the three distances is not seen, or
  // hardly, at every frame duration.
  for (const ms of DELAYS) expect(Math.min(...seen[ms].pair), `frames of ${ms} ms: ${said}`).toBeLessThan(0.08);
});
