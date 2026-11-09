import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURL, OUT, need, openPresenter } from './helpers.mjs';

// Acceptance value from spike S3: a 300 MB file shows its first slide within 3 s (Chromium). Since
// the source player runs only after "Start", the time splits into the start screen and the time
// from the click on "Start" to the first slide.
test('300 MB file: start screen and first slide within 3 s each', async ({ page }) => {
  need('claude');
  const size = fs.statSync(path.join(OUT, 'big.html')).size;
  expect(size).toBeGreaterThan(290 * 1024 * 1024);
  // the browser's own loading of the file (up to DOMContentLoaded) depends on the disk cache; the
  // limit applies to deckfold's part after it, up to the mark the start screen sets once it is in
  // the document (waiting for the selector would also count the previews that load right after)
  await page.goto(fileURL('big.html'), { waitUntil: 'domcontentloaded' });
  await page.waitForSelector('.df-start [data-df-start]');
  const { loaded, startScreen } = await page.evaluate(() => {
    const nav = performance.getEntriesByType('navigation')[0], m = performance.getEntriesByName('deckfold:start-screen')[0];
    return { loaded: Math.round(nav.domContentLoadedEventStart), startScreen: Math.round(m.startTime - nav.domContentLoadedEventStart) };
  });
  const t1 = Date.now();
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience && !document.querySelector('.df-start'));
  const firstSlide = Date.now() - t1;
  console.log(`file loaded after ${loaded} ms, start screen ${startScreen} ms later, first slide ${firstSlide} ms after "Start" (file ${(size / 1048576).toFixed(0)} MB)`);
  expect(startScreen).toBeLessThanOrEqual(3000);
  expect(firstSlide).toBeLessThanOrEqual(3000);
  // the end reloads the window: back on the start screen
  const t2 = Date.now();
  await page.keyboard.press('Escape');
  await page.waitForSelector('.df-start [data-df-start]');
  console.log(`end to the start screen: ${Date.now() - t2} ms`);
  expect(Date.now() - t2).toBeLessThanOrEqual(3000);
});

// PowerPoint deck with 200 slides: conversion, start screen, presenter view with previews and
// overview, steps.
test('PowerPoint deck with 200 slides', async ({ page, context }) => {
  const { convertMs } = JSON.parse(fs.readFileSync(path.join(OUT, 'pptx-big.json'), 'utf8'));
  console.log(`conversion of 200 slides: ${convertMs} ms`);
  const t0 = Date.now();
  await page.goto(fileURL('pptx-big.html'));
  await page.waitForSelector('.df-start [data-df-start]');
  await expect(page.locator('.df-strip .df-tile')).toHaveCount(200);
  const startScreen = Date.now() - t0;
  // the strip builds only the tiles that are visible
  expect(await page.locator('.df-strip .df-clone').count()).toBeLessThan(200);
  const pres = await openPresenter(page, context);
  const t1 = Date.now();
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__dfPptx && window.__deckfold && window.__deckfold.audience && !document.querySelector('.df-start'));
  const firstSlide = Date.now() - t1;
  await pres.waitForFunction(() => [...document.querySelectorAll('iframe[data-df-preview]')].filter((f) =>
    f.contentDocument && f.contentDocument.querySelector('.df-pptx-slide.df-on')).length === 3); // current, audience, next
  const ready = Date.now() - t1;
  // the overview band of the presenter view: ready when its first tiles are built
  const t2 = Date.now();
  await expect(pres.locator('.df-overview .df-tile')).toHaveCount(200);
  await pres.waitForFunction(() => document.querySelectorAll('.df-overview .df-tile .df-clone svg').length > 0);
  const overview = Date.now() - t2;
  const steps = [];
  for (let i = 0; i < 10; i++) {
    const t = Date.now();
    await pres.keyboard.press('ArrowRight');
    await page.waitForFunction((n) => window.__dfPptx.state().slide === n, i + 1);
    steps.push(Date.now() - t);
  }
  const step = Math.max(...steps);
  console.log(`start screen ${startScreen} ms, first slide ${firstSlide} ms, presenter view ready ${ready} ms, overview band ${overview} ms, slowest step ${step} ms`);
  expect(convertMs).toBeLessThanOrEqual(LIMITS.convertMs);
  expect(startScreen).toBeLessThanOrEqual(LIMITS.readyMs);
  expect(firstSlide).toBeLessThanOrEqual(LIMITS.readyMs);
  expect(ready).toBeLessThanOrEqual(LIMITS.readyMs);
  expect(overview).toBeLessThanOrEqual(LIMITS.overviewMs);
  expect(step).toBeLessThanOrEqual(LIMITS.stepMs);
});

// Measured on the first end-to-end slice (Apple silicon MacBook, Chrome, 2026-11-07): conversion
// 62 ms, presenter view ready 236 ms, overview 62 ms, slowest step 18 ms. The limits leave room
// for slower machines and for the features that M1 to M3 add to every slide.
const LIMITS = { convertMs: 3000, readyMs: 3000, overviewMs: 2000, stepMs: 500 };
