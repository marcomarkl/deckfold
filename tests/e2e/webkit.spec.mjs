// Fallback browser (E3): WebKit is close to Safari and has no Window Management API.
import { test, expect } from '@playwright/test';
import { solo, startScreen, openPresenter, audState, counter, slideCount, expected, need, assetState, middleColor } from './helpers.mjs';

test('WebKit: solo navigation in a claude.ai deck', async ({ page }) => {
  need('claude');
  await solo(page, 'claude.html');
  await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next', 'next']));
});

test('WebKit: presenter view without screen detection: no hint, full screen allowed, keys work', async ({ page, context }) => {
  need('claude');
  await startScreen(page, 'claude.html');
  const pres = await openPresenter(page, context);
  // WebKit has no Window Management API: deckfold does not check where the windows are
  await page.waitForTimeout(1500);
  await expect(page.locator('[data-df-fs]')).toBeEnabled();
  await expect(page.locator('[data-df-hint].df-on')).toHaveCount(0);
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  await pres.keyboard.press('ArrowRight');
  const s1 = await expected(pres, ['next']);
  await expect.poll(() => audState(page)).toEqual(s1);
  await expect.poll(() => counter(pres)).toContain(`${s1.slide + 1} / ${await slideCount(pres)}`);
});

test('WebKit: Keynote deck runs', async ({ page }) => {
  need('keynote');
  await solo(page, 'keynote.html');
  await expect(page.locator('#stage canvas').first()).toBeAttached({ timeout: 20000 });
  await page.waitForTimeout(1000);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next']));
});

// PowerPoint decks (public feature decks, no corpus needed)
const shownSlide = (page) => page.evaluate(() => [...document.querySelectorAll('.df-pptx-slide')].findIndex((s) => s.classList.contains('df-on')));

test('WebKit: PowerPoint solo navigation and fonts', async ({ page }) => {
  await solo(page, 'pptx-basic.html');
  await page.keyboard.press('PageDown');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next']));
  await expect.poll(() => shownSlide(page)).toBe(1);
  const fonts = await page.evaluate(async () => {
    await document.fonts.ready;
    const fams = new Set([...document.querySelectorAll('.df-pptx-slide text')].map((t) => t.getAttribute('font-family')));
    return [...fams].every((f) => document.fonts.check('16px ' + f));
  });
  expect(fonts).toBe(true);
});

test('WebKit: converted pictures and fonts of a PowerPoint deck load', async ({ page }) => {
  await solo(page, 'pptx-basic.html');
  const st = await assetState(page);
  const msg = JSON.stringify(st);
  expect(st.mimes.filter((m) => m === 'image/webp').length, msg).toBeGreaterThan(0);
  expect(st.mimes.filter((m) => m === 'font/woff2').length, msg).toBeGreaterThan(0);
  expect(st.pictures.length, msg).toBeGreaterThan(0);
  expect(st.pictures.filter((p) => !(p.width > 0) || p.error), msg).toEqual([]);
  expect(st.fonts.filter((f) => !f.endsWith(':loaded')), msg).toEqual([]);
});

test('WebKit: an animated WebP picture starts again when its slide is entered', async ({ page }) => {
  await solo(page, 'pptx-gifs.html');
  const img = page.locator('.df-pptx-slide.df-on image[data-df-pptx-gif]');
  await expect(img).toHaveCount(1);
  expect((await assetState(page)).mimes).toContain('image/webp');
  // the picture shows red, green and blue for half a second each
  await expect.poll(() => middleColor(page, img), { timeout: 3000 }).toBe('green');
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => shownSlide(page)).toBe(1);
  await page.waitForTimeout(400);
  await page.keyboard.press('ArrowLeft');
  await expect.poll(() => shownSlide(page)).toBe(0);
  expect(await middleColor(page, img)).toBe('red');
});

test('WebKit: PowerPoint animations reach every step', async ({ page }) => {
  await solo(page, 'pptx-animations.html');
  const steps = await page.evaluate(() => window.__dfPptx.deck.slides[0].steps);
  for (let k = 1; k < steps; k++) {
    await page.keyboard.press('ArrowRight');
    await expect.poll(() => page.evaluate(() => window.__dfPptx.state())).toEqual({ slide: 0, step: k });
  }
  // the entrance effects ended with their objects visible
  const hidden = () => page.evaluate(() => [...document.querySelectorAll('.df-pptx-slide.df-on g[data-sid]')]
    .filter((g) => getComputedStyle(g).visibility === 'hidden').length);
  await expect.poll(hidden, { timeout: 5000 }).toBe(0);
});

test('WebKit: PowerPoint transitions (2D, 3D, tiles) end on the next slide', async ({ page }) => {
  await solo(page, 'pptx-transitions3d.html');
  const n = await page.evaluate(() => window.__dfPptx.deck.slides.length);
  for (let i = 0; i < Math.min(n - 1, 12); i++) {
    await page.keyboard.press('ArrowRight');
    await expect.poll(() => shownSlide(page), { timeout: 8000 }).toBe(i + 1);
  }
});

test('WebKit: the presenter view closes with the window of the presentation, stays when that window reloads', async ({ page, context }) => {
  await startScreen(page, 'pptx-basic.html');
  const pres = await openPresenter(page, context);
  await page.reload();
  await page.waitForSelector('.df-start [data-df-start]');
  await page.waitForTimeout(2500);
  expect(pres.isClosed()).toBe(false);
  const closed = pres.waitForEvent('close', { timeout: 4000 });
  await page.close();
  await closed;
  expect(pres.isClosed()).toBe(true);
});

test('WebKit: search and outline: open, search, choose', async ({ page }) => {
  await startScreen(page, 'pptx-content.html');
  await page.click('[data-df-find]');
  await expect(page.locator('[data-df-finder].df-on [data-df-find-item]')).toHaveCount(await slideCount(page));
  await page.keyboard.type('nested');
  await expect(page.locator('[data-df-finder] [data-df-find-item]')).toHaveCount(1);
  await page.keyboard.press('Enter');
  await expect(page.locator('[data-df-finder].df-on')).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => +document.querySelector('.df-start .df-strip .df-tile.df-cur').getAttribute('data-i'))).toBe(1);
});

test('WebKit: the overlays close by their button; the key help opens from the footer', async ({ page }) => {
  await startScreen(page, 'pptx-content.html');
  await page.click('[data-df-find]');
  await expect(page.locator('[data-df-finder].df-on')).toHaveCount(1);
  await page.click('[data-df-finder] [data-df-close]');
  await expect(page.locator('[data-df-finder].df-on')).toHaveCount(0);
  await page.click('.df-start-foot [data-df-help]');
  await expect(page.locator('[data-df-help-overlay].df-on [data-df-keygroup]').first()).toBeVisible();
  await page.click('[data-df-help-overlay] [data-df-close]');
  await expect(page.locator('[data-df-help-overlay].df-on')).toHaveCount(0);
  // the preview keeps the shape of the slide
  await page.waitForSelector('.df-start-stage iframe[data-df-preview]');
  const m = await page.evaluate(() => {
    const f = document.querySelector('.df-start-stage iframe');
    const st = document.querySelector('.df-start-stage').getBoundingClientRect();
    return { stage: [st.width, st.height], ratio: parseFloat(f.style.width) / parseFloat(f.style.height) };
  });
  expect(Math.abs(m.stage[0] - m.stage[1] * m.ratio), JSON.stringify(m)).toBeLessThanOrEqual(1.5);
});
