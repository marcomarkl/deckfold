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
