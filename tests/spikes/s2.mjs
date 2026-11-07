// Spike S2: file:// behaviour in Chromium, Chrome, Firefox and WebKit.
import { chromium, firefox, webkit } from '@playwright/test';
const url = 'file://' + process.argv[2];
const engines = [['chromium', chromium, {}], ['chrome', chromium, { channel: 'chrome' }], ['firefox', firefox, {}], ['webkit', webkit, {}]];
for (const [name, type, opts] of engines) {
  let browser;
  try { browser = await type.launch(opts); } catch (e) { console.log(name, 'LAUNCH-FAIL', e.message.split('\n')[0]); continue; }
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto(url);
  const [popup] = await Promise.all([ctx.waitForEvent('page', { timeout: 5000 }).catch(() => null), page.click('#go')]);
  await page.waitForTimeout(1500);
  const r = await page.evaluate(() => window.R);
  console.log(name, JSON.stringify(r));
  await browser.close();
}
