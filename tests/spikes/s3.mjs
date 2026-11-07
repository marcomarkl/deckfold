// Spike S3: load time, decode time and JS heap of large Base64 files.
import { chromium, webkit } from '@playwright/test';
const dir = process.argv[2];
const files = ['one.html', 'chunks8.html', 'big375.html'];
for (const [name, type, opts] of [['chrome', chromium, { channel: 'chrome' }], ['webkit', webkit, {}]]) {
  for (const f of files) {
    const browser = await type.launch(opts);
    const page = await browser.newPage();
    const t = Date.now();
    let res;
    try {
      await page.goto('file://' + dir + '/' + f, { waitUntil: 'load', timeout: 180000 });
      const wall = Date.now() - t;
      const r = await page.evaluate(() => ({ dcl: window.TDCL, load: window.TLOAD, d0: window.decode('b0') }));
      let heap = null;
      if (name === 'chrome') {
        const cdp = await page.context().newCDPSession(page);
        await cdp.send('Performance.enable');
        const m = (await cdp.send('Performance.getMetrics')).metrics;
        heap = Math.round(m.find(x => x.name === 'JSHeapUsedSize').value / 1048576);
      }
      res = { wallMs: wall, dclMs: Math.round(r.dcl), decodeFirstBlock: { readMs: Math.round(r.d0.read), decodeMs: Math.round(r.d0.decode), MB: +(r.d0.bytes / 1048576).toFixed(1) }, heapMB: heap };
    } catch (e) { res = 'FAIL ' + e.message.split('\n')[0]; }
    console.log(name, f, JSON.stringify(res));
    await browser.close();
  }
}
