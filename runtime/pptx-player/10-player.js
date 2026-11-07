// Player of decks that deckfold generates from PowerPoint files.
//
// The deck is static SVG, one <section class="df-pptx-slide"> per slide. The player shows one
// slide at a time, scales the stage to the window and keeps the state (slide, step). deckfold's
// control layer drives it through window.__dfPptx (see the "pptx" adapter in runtime/src/20-model.js);
// the player itself listens to no keys.
//
// API:
//   goTo(slide, step)  show the state after `step` of `slide` at once, without animation
//   next() / prev()    one step forward or back
//   showAll(slide)     show every object of the slide (the editing view, for image comparison)
//   state()            {slide, step}
// Events on window: "df-pptx-ready" once, "df-pptx-change" after every state change.

var PLAYER_CSS =
  "html[data-df-pptx-run],html[data-df-pptx-run] body{margin:0;width:100%;height:100%;overflow:hidden;background:#000}" +
  "html[data-df-pptx-run] #df-pptx-stage{position:absolute;left:50%;top:50%;transform-origin:50% 50%}" +
  "html[data-df-pptx-run] .df-pptx-slide{position:absolute!important;left:0;top:0}" +
  "html[data-df-pptx-run] .df-pptx-slide:not(.df-on){display:none}" +
  ".df-pptx-svg{display:block}";

var dataEl = document.getElementById("df-pptx-deck");
var deck = { width: 1280, height: 720, slides: [] };
try { deck = JSON.parse(dataEl.textContent); } catch (e) { }
var stage = document.getElementById("df-pptx-stage");
var sections = [].slice.call(document.querySelectorAll(".df-pptx-slide"));
var cur = { slide: 0, step: 0 };

var style = document.createElement("style");
style.textContent = PLAYER_CSS;
(document.head || document.documentElement).appendChild(style);
document.documentElement.setAttribute("data-df-pptx-run", "");

function stepsOf(i) {
  var s = deck.slides[i];
  return (s && s.steps) || 1;
}

function fit() {
  if (!stage) return;
  var w = window.innerWidth || deck.width, h = window.innerHeight || deck.height;
  var s = Math.min(w / deck.width, h / deck.height);
  stage.style.transform = "translate(-50%,-50%) scale(" + s + ")";
}

function emit(name) {
  var e;
  try { e = new CustomEvent(name, { detail: { slide: cur.slide, step: cur.step } }); }
  catch (err) { e = document.createEvent("CustomEvent"); e.initCustomEvent(name, false, false, { slide: cur.slide, step: cur.step }); }
  window.dispatchEvent(e);
}

// Previews in the presenter view never play media.
function isPreview() {
  try { return !!(window.__deckfold && window.__deckfold.role === "preview"); } catch (e) { return false; }
}

function media(el) { return [].slice.call(el.querySelectorAll("video[data-df-pptx-media],audio[data-df-pptx-media]")); }

function stopMedia(el) {
  media(el).forEach(function (m) { try { m.pause(); m.currentTime = 0; } catch (e) { } });
}

function startMedia(el) {
  if (isPreview()) return;
  media(el).forEach(function (m) {
    if (m.getAttribute("data-df-pptx-media") !== "auto") return;
    try { var p = m.play(); if (p && p["catch"]) p["catch"](function () { }); } catch (e) { }
  });
}

var shownIndex = -1;
function show(i) {
  for (var j = 0; j < sections.length; j++) {
    if (j === i) sections[j].classList.add("df-on");
    else sections[j].classList.remove("df-on");
  }
  if (i !== shownIndex) {
    if (shownIndex >= 0 && sections[shownIndex]) stopMedia(sections[shownIndex]);
    shownIndex = i;
    if (sections[i]) startMedia(sections[i]);
  }
}

// A click on a video starts or pauses it, as in the PowerPoint slide show.
document.addEventListener("click", function (e) {
  var m = e.target && e.target.closest && e.target.closest("video[data-df-pptx-media]");
  if (!m || isPreview()) return;
  try { if (m.paused) { var p = m.play(); if (p && p["catch"]) p["catch"](function () { }); } else m.pause(); } catch (err) { }
});

function clampSlide(i) { return Math.max(0, Math.min(sections.length - 1, i | 0)); }

// ---------- animations ----------
var anims = [];
function animOf(i) {
  if (!anims[i]) anims[i] = new Anim.Slide(sections[i], (deck.slides[i] || {}).anim, deck.width, deck.height);
  return anims[i];
}

var playing = null; // {raf, finish}
function finishPlaying() { if (playing) { var p = playing; playing = null; cancelAnimationFrame(p.raf); p.finish(); } }

// media commands of the timeline (playFrom, pause, stop) fire once per played step
function mediaCmd(fired) {
  return function (b, el) {
    if (fired.indexOf(b) >= 0 || isPreview()) return;
    fired.push(b);
    var m = el && el.querySelector("video,audio");
    if (!m) return;
    try {
      if (/^playFrom/.test(b.cmd)) { var p = m.play(); if (p && p["catch"]) p["catch"](function () { }); }
      else if (/^(pause|stop)/.test(b.cmd)) { m.pause(); if (/^stop/.test(b.cmd)) m.currentTime = 0; }
    } catch (e) { }
  };
}

// plays step k of slide i from the state of step k-1 (or the initial state for k = 0)
function playStep(i, k) {
  var a = animOf(i), from = k > 0 ? a.stateAt(k - 1) : a.initial(), dur = a.dur(k);
  var fired = [], fire = mediaCmd(fired);
  if (isPreview() || dur <= 0) { a.show(a.after(k, from)); a.at(k, 1e12, from, fire); return; }
  var t0 = performance.now();
  var frame = function (now) {
    var t = now - t0;
    a.show(a.at(k, t, from, fire));
    if (t < dur) playing.raf = requestAnimationFrame(frame);
    else { playing = null; a.show(a.after(k, from)); }
  };
  playing = { raf: requestAnimationFrame(frame), finish: function () { a.show(a.after(k, from)); } };
}

// ---------- transitions ----------
function transition(from, to, done) {
  var tr = ((deck.slides[to] || {}).anim || {}).transition;
  var a = sections[from], b = sections[to];
  if (!tr || !tr.dur || isPreview() || !a || !b || from === to) { done(); return; }
  var W = deck.width, H = deck.height, dir = tr.dir || "l";
  a.classList.add("df-on"); b.classList.add("df-on");
  b.style.zIndex = "2"; a.style.zIndex = "1";
  function vec(d) { return { l: [-1, 0], r: [1, 0], u: [0, -1], d: [0, 1], lu: [-1, -1], ru: [1, -1], ld: [-1, 1], rd: [1, 1] }[d] || [-1, 0]; }
  var v = vec(dir);
  var set = function (p) {
    switch (tr.type) {
      case "push":
        a.style.transform = "translate(" + v[0] * W * p + "px," + v[1] * H * p + "px)";
        b.style.transform = "translate(" + -v[0] * W * (1 - p) + "px," + -v[1] * H * (1 - p) + "px)";
        break;
      case "cover":
        b.style.transform = "translate(" + -v[0] * W * (1 - p) + "px," + -v[1] * H * (1 - p) + "px)";
        break;
      case "pull":
        a.style.zIndex = "3";
        a.style.transform = "translate(" + v[0] * W * p + "px," + v[1] * H * p + "px)";
        break;
      case "wipe":
        var r = (1 - p) * 100;
        b.style.clipPath = dir === "r" ? "inset(0 " + r + "% 0 0)" : dir === "u" ? "inset(" + r + "% 0 0 0)" : dir === "d" ? "inset(0 0 " + r + "% 0)" : "inset(0 0 0 " + r + "%)";
        break;
      case "split":
        b.style.clipPath = "inset(0 " + (1 - p) * 50 + "% 0 " + (1 - p) * 50 + "%)";
        break;
      case "circle":
        b.style.clipPath = "circle(" + p * 75 + "% at 50% 50%)";
        break;
      case "zoom":
        b.style.opacity = String(p); b.style.transform = "scale(" + (0.3 + 0.7 * p) + ")";
        break;
      default: // fade and every transition deckfold has no drawing for
        b.style.opacity = String(p);
    }
  };
  var clear = function () {
    [a, b].forEach(function (s) { s.style.transform = ""; s.style.opacity = ""; s.style.clipPath = ""; s.style.zIndex = ""; });
  };
  set(0);
  var t0 = performance.now();
  var frame = function (now) {
    var p = Math.min(1, (now - t0) / tr.dur);
    set(p);
    if (p < 1) playing.raf = requestAnimationFrame(frame);
    else { playing = null; clear(); done(); }
  };
  playing = { raf: requestAnimationFrame(frame), finish: function () { clear(); done(); } };
}

// goTo shows the state after `step` at once, without animation (previews, back, jumps).
function goTo(slide, step) {
  finishPlaying();
  slide = clampSlide(slide);
  step = Math.max(0, Math.min(stepsOf(slide) - 1, step | 0));
  cur = { slide: slide, step: step };
  show(slide);
  animOf(slide).show(animOf(slide).stateAt(step));
  emit("df-pptx-change");
}

// next plays the next step, or the transition to the next slide and its entry effects.
function next() {
  finishPlaying();
  if (cur.step < stepsOf(cur.slide) - 1) {
    cur = { slide: cur.slide, step: cur.step + 1 };
    playStep(cur.slide, cur.step);
    emit("df-pptx-change");
    return;
  }
  if (cur.slide >= sections.length - 1) return;
  var from = cur.slide, to = from + 1;
  cur = { slide: to, step: 0 };
  // the new slide starts in its initial state (objects with entrance effects hidden)
  animOf(to).show(animOf(to).initial());
  transition(from, to, function () {
    show(to);
    playStep(to, 0);
  });
  emit("df-pptx-change");
}

function prev() {
  if (cur.step > 0) goTo(cur.slide, cur.step - 1);
  else if (cur.slide > 0) goTo(cur.slide - 1, stepsOf(cur.slide - 1) - 1);
  else goTo(0, 0);
}

// showAll shows every object of a slide as in the editing view (for the image comparison).
function showAll(slide) {
  goTo(slide, 0);
  animOf(cur.slide).reset();
}

window.__dfPptx = {
  goTo: goTo, next: next, prev: prev, showAll: showAll,
  state: function () { return { slide: cur.slide, step: cur.step }; },
  deck: deck
};

// Load every embedded face now: the browser would otherwise load a face only when a visible
// slide uses it, and its text would be missing for a moment on the first change of slides.
try { if (document.fonts && document.fonts.forEach) document.fonts.forEach(function (f) { if (f.status === "unloaded") f.load()["catch"](function () { }); }); } catch (e) { }

window.addEventListener("resize", fit);
fit();
goTo(0, 0);
emit("df-pptx-ready");
