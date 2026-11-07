// Builds deckfold and writes the test outputs to tests/.out/.
//
// The local corpus in testdata/ is not part of the repository (it holds private decks).
// Expected layout: testdata/claude/*.html and testdata/keynote/<export>/index.html.
// Tests that need a corpus entry skip themselves when it is missing (see corpus.json).
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const ROOT = path.resolve(here, '..', '..');
export const OUT = path.join(ROOT, 'tests', '.out');
const BIN = path.join(OUT, process.platform === 'win32' ? 'deckfold.exe' : 'deckfold');

function deckfold(args, env) {
  return execFileSync(BIN, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, ...(env || {}) } });
}

// Public PowerPoint feature decks: the PowerPoint-saved copy in fixtures/pptx when it exists,
// otherwise the generator output in fixtures/pptx/src. Converted with the bundled fonts only,
// so the output does not depend on the fonts installed on the machine.
export function pptxFixture(name) {
  const saved = path.join(ROOT, 'fixtures', 'pptx', name + '.pptx');
  return fs.existsSync(saved) ? saved : path.join(ROOT, 'fixtures', 'pptx', 'src', name + '.pptx');
}
const BUNDLED = { DECKFOLD_BUNDLED_FONTS: '1' };

// WAV file with a sine tone (duration in seconds), for the "auto mode waits for media" test.
function wav(file, seconds) {
  const rate = 8000, n = rate * seconds, buf = Buffer.alloc(44 + n * 2);
  buf.write('RIFF', 0); buf.writeUInt32LE(36 + n * 2, 4); buf.write('WAVE', 8); buf.write('fmt ', 12);
  buf.writeUInt32LE(16, 16); buf.writeUInt16LE(1, 20); buf.writeUInt16LE(1, 22); buf.writeUInt32LE(rate, 24);
  buf.writeUInt32LE(rate * 2, 28); buf.writeUInt16LE(2, 32); buf.writeUInt16LE(16, 34); buf.write('data', 36); buf.writeUInt32LE(n * 2, 40);
  for (let i = 0; i < n; i++) buf.writeInt16LE(Math.round(Math.sin(i / rate * 2 * Math.PI * 440) * 3000), 44 + i * 2);
  fs.writeFileSync(file, buf);
}

function findCorpus() {
  const td = path.join(ROOT, 'testdata');
  const list = (d) => (fs.existsSync(d) ? fs.readdirSync(d).sort() : []);
  const claude = list(path.join(td, 'claude')).filter((f) => f.endsWith('.html')).map((f) => path.join('claude', f))[0] || null;
  const keynote = list(path.join(td, 'keynote')).map((d) => path.join('keynote', d))
    .find((d) => fs.existsSync(path.join(td, d, 'assets', 'player', 'main.js'))) || null;
  return { claude, keynote };
}

// Inserts html after the n-th slide canvas wrapper of a claude.ai deck.
function injectIntoSlide(html, n, snippet) {
  let count = 0;
  return html.replace(/<div class="deck-scaler">/g, (m) => (++count === n ? m + snippet : m));
}

export default async function setup() {
  fs.mkdirSync(OUT, { recursive: true });
  execFileSync('go', ['build', '-o', BIN, './cmd/deckfold'], { cwd: ROOT, stdio: 'inherit' });
  const corpus = findCorpus();
  fs.writeFileSync(path.join(OUT, 'corpus.json'), JSON.stringify(corpus));

  // Generic deck with its own key handler and a live embed (no corpus needed)
  const gen = path.join(OUT, 'generic');
  fs.mkdirSync(gen, { recursive: true });
  fs.writeFileSync(path.join(gen, 'deck.html'), `<!doctype html><html><head><meta charset="utf-8"><title>Generic</title></head><body>
<h1 id="k">none</h1><div style="position:relative;width:400px;height:300px"><iframe id="live" src="https://example.com/" style="width:400px;height:300px"></iframe></div>
<script>window.keys=[];document.addEventListener('keydown',function(e){window.keys.push(e.key);document.getElementById('k').textContent=e.key;});</script>
</body></html>`);
  deckfold([path.join(gen, 'deck.html'), '-o', path.join(OUT, 'generic.html'), '--force', '--generic']);

  // PowerPoint feature decks (public, no corpus needed)
  for (const name of ['basic', 'media', 'animations']) {
    deckfold([pptxFixture(name), '-o', path.join(OUT, `pptx-${name}.html`), '--force'], BUNDLED);
  }
  // every feature deck with PowerPoint reference images (pptx-fidelity.spec.mjs)
  const ref = path.join(ROOT, 'fixtures', 'pptx', 'ref');
  for (const name of fs.existsSync(ref) ? fs.readdirSync(ref) : []) {
    if (fs.existsSync(path.join(ref, name, 'source.sha256'))) {
      deckfold([pptxFixture(name), '-o', path.join(OUT, `pptx-ref-${name}.html`), '--force'], BUNDLED);
    }
  }
  // auto mode: 1 s per step without source timing, the source timing (advTm) on slide 3
  deckfold([pptxFixture('animations'), '-o', path.join(OUT, 'pptx-animations-auto.html'), '--force', '--auto', '--auto-interval', '1'], BUNDLED);
  // the same deck with an old player in the output, for the --refresh test
  const old = fs.readFileSync(path.join(OUT, 'pptx-basic.html'), 'utf8')
    .replace(/(<!--deckfold:pptx-player:begin--><script[^>]*>)[\s\S]*?(<\/script><!--deckfold:pptx-player:end-->)/, '$1window.__dfOldPlayer=true;$2');
  fs.writeFileSync(path.join(OUT, 'pptx-old.html'), old);
  deckfold(['--refresh', path.join(OUT, 'pptx-old.html'), '-o', path.join(OUT, 'pptx-refreshed.html'), '--force']);
  // 200 slides for the performance test (generated, not committed); the conversion is timed
  const pgen = path.join(OUT, 'pptxgen');
  execFileSync('go', ['run', './scripts/pptxgen', '-big', '-out', pgen], { cwd: ROOT, stdio: 'ignore' });
  const t0 = Date.now();
  deckfold([path.join(pgen, 'big.pptx'), '-o', path.join(OUT, 'pptx-big.html'), '--force'], BUNDLED);
  fs.writeFileSync(path.join(OUT, 'pptx-big.json'), JSON.stringify({ convertMs: Date.now() - t0 }));

  if (corpus.keynote) {
    deckfold([path.join(ROOT, 'testdata', corpus.keynote), '-o', path.join(OUT, 'keynote.html'), '--force']);
  }
  if (!corpus.claude) return;
  const claudeSrc = path.join(ROOT, 'testdata', corpus.claude);
  deckfold([claudeSrc, '-o', path.join(OUT, 'claude.html'), '--force']);
  // Auto mode with a 1 s interval
  deckfold([claudeSrc, '-o', path.join(OUT, 'claude-auto.html'), '--force', '--auto', '--auto-interval', '1', '--loop']);

  // Media deck: the claude.ai deck with a 4 s tone (autoplay) on slide 2
  const media = path.join(OUT, 'media');
  fs.mkdirSync(media, { recursive: true });
  wav(path.join(media, 'tone.wav'), 4);
  const html = injectIntoSlide(fs.readFileSync(claudeSrc, 'utf8'), 2, '<audio id="df-test-audio" autoplay src="tone.wav"></audio>');
  fs.writeFileSync(path.join(media, 'deck.html'), html);
  deckfold([path.join(media, 'deck.html'), '-o', path.join(OUT, 'media.html'), '--force', '--auto-interval', '1']);

  // Large file for the performance test (225 MB of video data, about 300 MB of HTML)
  const big = path.join(OUT, 'big');
  const bigOut = path.join(OUT, 'big.html');
  if (!fs.existsSync(bigOut)) {
    fs.mkdirSync(big, { recursive: true });
    const fd = fs.openSync(path.join(big, 'film.mp4'), 'w');
    const chunk = Buffer.alloc(16 << 20);
    for (let i = 0; i < 14; i++) { for (let j = 0; j < chunk.length; j += 4) chunk.writeUInt32LE((Math.random() * 0xffffffff) >>> 0, j); fs.writeSync(fd, chunk); }
    fs.closeSync(fd);
    const h = injectIntoSlide(fs.readFileSync(claudeSrc, 'utf8'), 3, '<video src="film.mp4" preload="none"></video>');
    fs.writeFileSync(path.join(big, 'deck.html'), h);
    deckfold([path.join(big, 'deck.html'), '-o', bigOut, '--force']);
  }
}
