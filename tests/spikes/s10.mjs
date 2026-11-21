// Spike S10: screen wake lock of the presentation window (docs/spikes/S10-wake-lock.md). macOS only:
// it reads the power assertions with pmset and the idle time with ioreg.
//
//   export GOFLAGS=-tags=nodynamic
//   DECKFOLD_NO_FFMPEG=1 DECKFOLD_BUNDLED_FONTS=1 go run ./cmd/deckfold fixtures/pptx/basic.pptx \
//     -o tests/.out/wakelock/basic.html --force --no-cache
//   node tests/spikes/s10.mjs grant   # Chrome and Playwright WebKit: is the lock granted, which assertion
//   node tests/spikes/s10.mjs idle    # Chrome: does the screen saver start while the lock is held
//
// "idle" needs a machine nobody touches for up to 25 minutes. Phase A holds the lock until the HID idle
// time is 90 s past the screen saver delay; phase B closes the browser and waits for the screen saver,
// which shows that it starts on this machine at all when nothing holds it off.
import { chromium, webkit } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import path from 'node:path';

const file = pathToFileURL(path.resolve('tests/.out/wakelock/basic.html')).href;
const sh = (cmd, args) => { try { return execFileSync(cmd, args).toString(); } catch { return ''; } };
const displayAsserts = () => sh('pmset', ['-g', 'assertions']).split('\n')
  .filter((l) => /NoDisplaySleep|PreventUserIdleDisplaySleep/.test(l)).map((l) => l.trim());
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// records what navigator.wakeLock.request gives the page, and every change of visibility
const hook = () => {
  window.__wl = [];
  if (!navigator.wakeLock) { window.__wl.push('no navigator.wakeLock, isSecureContext=' + isSecureContext); return; }
  const orig = navigator.wakeLock.request.bind(navigator.wakeLock);
  navigator.wakeLock.request = (t) => {
    const p = orig(t);
    p.then((s) => { window.__wl.push('granted ' + t); s.addEventListener('release', () => window.__wl.push('released')); },
      (e) => window.__wl.push('rejected ' + e.name + ': ' + e.message));
    return p;
  };
  document.addEventListener('visibilitychange', () => window.__wl.push('vis:' + document.visibilityState));
};

async function startShow(type, opts) {
  const browser = await type.launch({ headless: false, ...opts });
  const ctx = await browser.newContext();
  await ctx.addInitScript(hook);
  const page = await ctx.newPage();
  await page.goto(file);
  await page.waitForSelector('.df-start [data-df-start]');
  const before = await page.evaluate(() => ({ secure: isSecureContext, api: !!navigator.wakeLock, log: window.__wl }));
  await page.click('[data-df-start]');
  await page.waitForFunction(() => window.__deckfold && window.__deckfold.audience);
  return { browser, ctx, page, before };
}
const pageLog = (page) => page.evaluate(() => `${document.visibilityState} ${JSON.stringify(window.__wl)}`)
  .catch((e) => 'eval failed: ' + e.message);

async function grant(name, type, opts) {
  console.log(`\n=== ${name} ===`);
  console.log('display assertions before:', displayAsserts());
  const { browser, page, before } = await startShow(type, opts);
  console.log('start screen:', before);
  await sleep(1500);
  console.log('after start:', await pageLog(page));
  console.log('display assertions after start:', displayAsserts());
  await browser.close();
  console.log('display assertions after close:', displayAsserts());
}

async function idleRun() {
  const saverDelay = Number(sh('defaults', ['-currentHost', 'read', 'com.apple.screensaver', 'idleTime']).trim());
  const idle = () => Number((sh('ioreg', ['-c', 'IOHIDSystem']).match(/"HIDIdleTime" = (\d+)/) || [])[1] || 0) / 1e9;
  const saver = () => sh('pgrep', ['-l', '-f', 'ScreenSaverEngine|legacyScreenSaver']).trim().split('\n')
    .filter((l) => l && !l.includes('pgrep'));
  const locked = () => /CGSSessionScreenIsLocked"?\s*=\s*Yes/.test(sh('ioreg', ['-n', 'Root', '-d1']));
  const t0 = Date.now();
  const log = (phase, extra = '') => {
    const s = saver();
    console.log(`${new Date().toISOString()} +${Math.round((Date.now() - t0) / 1000)}s ${phase} idle=${idle().toFixed(0)}s ` +
      `saver=${s.length ? s.join(' | ') : 'none'} locked=${locked()} display=${JSON.stringify(displayAsserts())} ${extra}`);
    return s.length > 0;
  };
  console.log(`screen saver delay ${saverDelay}s`);
  log('before');

  // phase A: the presentation in full screen with the wake lock
  const { browser, ctx, page } = await startShow(chromium, { channel: 'chrome' });
  const cdp = await ctx.newCDPSession(page);
  const { windowId } = await cdp.send('Browser.getWindowForTarget');
  await cdp.send('Browser.setWindowBounds', { windowId, bounds: { windowState: 'fullscreen' } });
  await sleep(2000);
  let sawSaverA = false;
  const endA = Date.now() + (saverDelay + 900) * 1000;
  while (Date.now() < endA) {
    if (log('A wakelock', await pageLog(page))) sawSaverA = true;
    if (idle() >= saverDelay + 90) break;
    await sleep(15000);
  }
  log('A end');
  await browser.close();
  const idleAtRelease = idle();

  // phase B: nothing of ours holds the display
  let sawSaverB = false;
  const endB = Date.now() + (saverDelay + 120) * 1000;
  while (Date.now() < endB) {
    if (log('B control')) { sawSaverB = true; break; }
    await sleep(10000);
  }
  log('B end');
  console.log(`RESULT sawSaverWithWakeLock=${sawSaverA} idleAtRelease=${idleAtRelease.toFixed(0)}s sawSaverWithout=${sawSaverB}`);
}

const mode = process.argv[2];
if (mode === 'grant') {
  await grant('Chrome (channel chrome)', chromium, { channel: 'chrome' });
  await grant('Playwright WebKit', webkit, {});
} else if (mode === 'idle') {
  await idleRun();
} else {
  console.error('usage: node tests/spikes/s10.mjs grant|idle');
  process.exit(2);
}
