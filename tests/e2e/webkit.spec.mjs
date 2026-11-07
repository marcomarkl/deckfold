// Fallback browser (E3): WebKit is close to Safari and has no Window Management API.
import { test, expect } from '@playwright/test';
import { solo, present, audState, counter, slideCount, expected, need } from './helpers.mjs';

test('WebKit: solo navigation in a claude.ai deck', async ({ page }) => {
  need('claude');
  await solo(page, 'claude.html');
  await page.keyboard.press('ArrowRight'); await page.keyboard.press('ArrowRight');
  await expect.poll(() => audState(page)).toEqual(await expected(page, ['next', 'next']));
});

test('WebKit: presenter view with the hint to drag the window', async ({ page, context }) => {
  need('claude');
  const aud = await present(page, context, 'claude.html');
  await expect(page.locator('.df-banner.df-on')).toBeVisible();
  await page.keyboard.press('ArrowRight');
  const s1 = await expected(page, ['next']);
  await expect.poll(() => audState(aud)).toEqual(s1);
  await expect.poll(() => counter(page)).toContain(`${s1.slide + 1} / ${await slideCount(page)}`);
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

test('WebKit: PowerPoint animations reach every step', async ({ page }) => {
  await solo(page, 'pptx-animations.html');
  const steps = await page.evaluate(() => window.__dfPptx.deck.slides[0].steps);
  for (let k = 1; k < steps; k++) {
    await page.keyboard.press('ArrowRight');
    await expect.poll(() => page.evaluate(() => window.__dfPptx.state())).toEqual({ slide: 0, step: k });
  }
  // the entrance effects ended with their objects visible
  const hidden = await page.evaluate(() => [...document.querySelectorAll('.df-pptx-slide.df-on g[data-sid]')]
    .filter((g) => getComputedStyle(g).visibility === 'hidden').length);
  expect(hidden).toBe(0);
});

test('WebKit: PowerPoint transitions (2D, 3D, tiles) end on the next slide', async ({ page }) => {
  await solo(page, 'pptx-transitions3d.html');
  const n = await page.evaluate(() => window.__dfPptx.deck.slides.length);
  for (let i = 0; i < Math.min(n - 1, 12); i++) {
    await page.keyboard.press('ArrowRight');
    await expect.poll(() => shownSlide(page), { timeout: 8000 }).toBe(i + 1);
  }
});
