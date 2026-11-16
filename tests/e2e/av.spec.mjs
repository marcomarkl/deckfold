// Video and sound that deckfold converted with the ffmpeg of the machine play in the browser
// (decision E18). The deck comes from the setup: a Windows Media video and a WAV sound, which
// become an MP4 file with H.264 and a FLAC file. Without ffmpeg there is no such deck.
import { test, expect } from '@playwright/test';
import { solo, need } from './helpers.mjs';

// What a media element says about itself, for the failure message.
const state = (page, id) => page.evaluate((id) => {
  const m = document.getElementById(id);
  return {
    src: (m.currentSrc || m.src).slice(0, 5), readyState: m.readyState, error: m.error ? m.error.code : 0,
    duration: Number.isFinite(m.duration) ? Math.round(m.duration * 10) / 10 : String(m.duration),
    width: m.videoWidth || 0, height: m.videoHeight || 0,
    canPlay: { mp4: m.canPlayType('video/mp4; codecs="avc1.640015"'), flac: m.canPlayType('audio/flac') },
  };
}, id);

test('converted video and sound: the browser plays the MP4 and the FLAC file', async ({ page }) => {
  need('av');
  await solo(page, 'av.html');
  // the files are the converted ones
  const mimes = await page.evaluate(() => [...document.querySelectorAll('script[type="application/octet-stream"][data-mime]')].map((s) => s.getAttribute('data-mime')));
  expect([...new Set(mimes)].sort(), JSON.stringify(mimes)).toEqual(['audio/flac', 'video/mp4']);
  // the video has its picture and its length, and a frame can be shown
  await expect.poll(() => state(page, 'v'), { timeout: 15000 }).toMatchObject({ src: 'blob:', error: 0, width: 320, height: 240 });
  // two seconds of picture; the sound track may end a little later, and browsers report the longer one
  const length = (await state(page, 'v')).duration;
  expect(length, JSON.stringify(await state(page, 'v'))).toBeGreaterThanOrEqual(1.9);
  expect(length).toBeLessThanOrEqual(2.3);
  await expect.poll(async () => (await state(page, 'v')).readyState, { timeout: 15000 }).toBeGreaterThanOrEqual(2);
  // it plays: the position moves
  await page.evaluate(() => document.getElementById('v').play());
  await expect.poll(() => page.evaluate(() => document.getElementById('v').currentTime), { timeout: 15000 }).toBeGreaterThan(0.3);
  expect((await state(page, 'v')).error, JSON.stringify(await state(page, 'v'))).toBe(0);
  // the sound is read to its end
  await expect.poll(() => state(page, 'a'), { timeout: 15000 }).toMatchObject({ src: 'blob:', error: 0, duration: 2 }); // FLAC holds the samples of the WAV file, so the length is exact
  await expect.poll(async () => (await state(page, 'a')).readyState, { timeout: 15000 }).toBeGreaterThanOrEqual(2);
});
