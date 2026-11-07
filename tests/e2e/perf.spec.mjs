import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURL, OUT, need } from './helpers.mjs';

// Acceptance value from spike S3: a 300 MB file shows its first slide within 3 s (Chromium).
test('300 MB file: first slide within 3 s', async ({ page }) => {
  need('claude');
  const size = fs.statSync(path.join(OUT, 'big.html')).size;
  expect(size).toBeGreaterThan(290 * 1024 * 1024);
  await page.goto(fileURL('big.html'));
  await page.waitForSelector('.df-start');
  const t0 = Date.now();
  await page.click('.df-start-buttons button:nth-child(2)');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience && !document.documentElement.classList.contains('df-loading'));
  const ms = Date.now() - t0;
  console.log(`solo view after ${ms} ms (file ${(size / 1048576).toFixed(0)} MB)`);
  expect(ms).toBeLessThanOrEqual(3000);
  const t1 = Date.now();
  await page.reload();
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience && !document.documentElement.classList.contains('df-loading'));
  console.log(`reload to the first slide: ${Date.now() - t1} ms`);
  expect(Date.now() - t1).toBeLessThanOrEqual(3000);
});

// PowerPoint deck with 200 slides: conversion, presenter view with previews and overview, steps.
test('PowerPoint deck with 200 slides', async ({ page, context }) => {
  const { convertMs } = JSON.parse(fs.readFileSync(path.join(OUT, 'pptx-big.json'), 'utf8'));
  console.log(`conversion of 200 slides: ${convertMs} ms`);
  await page.goto(fileURL('pptx-big.html'));
  await page.waitForSelector('.df-start');
  const t0 = Date.now();
  const popupP = context.waitForEvent('page');
  await page.click('.df-start-buttons button:nth-child(1)');
  const dlg = page.locator('.df-dialog .df-primary');
  await dlg.waitFor({ timeout: 3000 }).then(() => dlg.click()).catch(() => {});
  const aud = await popupP;
  await aud.waitForFunction(() => window.__dfPptx && window.__deckfold && window.__deckfold.audience);
  await page.waitForFunction(() => [...document.querySelectorAll('iframe[data-df-preview]')].filter((f) =>
    f.contentDocument && f.contentDocument.querySelector('.df-pptx-slide.df-on')).length === 2);
  const ready = Date.now() - t0;
  const t1 = Date.now();
  await page.click('.df-bar button[data-k="overview"]');
  await page.waitForFunction(() => document.querySelectorAll('.df-tile .df-clone svg').length === 200);
  const overview = Date.now() - t1;
  await page.click('.df-bar button[data-k="overview"]'); // close the overview (Escape would end the show)
  const steps = [];
  for (let i = 0; i < 10; i++) {
    const t = Date.now();
    await page.keyboard.press('ArrowRight');
    await aud.waitForFunction((n) => window.__dfPptx.state().slide === n, i + 1);
    steps.push(Date.now() - t);
  }
  const step = Math.max(...steps);
  console.log(`presenter view ready ${ready} ms, overview ${overview} ms, slowest step ${step} ms`);
  expect(convertMs).toBeLessThanOrEqual(LIMITS.convertMs);
  expect(ready).toBeLessThanOrEqual(LIMITS.readyMs);
  expect(overview).toBeLessThanOrEqual(LIMITS.overviewMs);
  expect(step).toBeLessThanOrEqual(LIMITS.stepMs);
});

// Measured on the first end-to-end slice (Apple silicon MacBook, Chrome, 2026-11-07): conversion
// 62 ms, presenter view ready 236 ms, overview 62 ms, slowest step 18 ms. The limits leave room
// for slower machines and for the features that M1 to M3 add to every slide.
const LIMITS = { convertMs: 3000, readyMs: 3000, overviewMs: 2000, stepMs: 500 };
