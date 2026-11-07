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

var stageScale = 1;
function fit() {
  if (!stage) return;
  var w = window.innerWidth || deck.width, h = window.innerHeight || deck.height;
  var s = Math.min(w / deck.width, h / deck.height);
  stageScale = s;
  stage.style.transform = "translate(-50%,-50%) scale(" + s + ")";
}

// draw3D draws the 3D shapes of slide i with WebGL (not in previews).
function draw3D(i) {
  if (isPreview() || !sections[i] || typeof Shapes3D === "undefined") return;
  Shapes3D.slide(sections[i], Math.max(2, stageScale * (window.devicePixelRatio || 1)));
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

// GIF animations start again from their first frame whenever their slide is entered, as in the
// PowerPoint slide show. A browser keeps one animation clock per image URL, so each entry gives
// the image a new fragment on its (blob) URL.
var gifEntered = -1, gifRun = 0;
function restartGifs(i) {
  if (i === gifEntered || isPreview() || !sections[i]) return;
  gifEntered = i;
  gifRun++;
  [].forEach.call(sections[i].querySelectorAll("image[data-df-pptx-gif]"), function (im) {
    var href = im.getAttribute("href");
    if (!href || /^deckfold:/.test(href)) return;
    im.setAttribute("href", href.replace(/#.*$/, "") + "#df-run" + gifRun);
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
    restartGifs(i);
    draw3D(i);
  }
}

// A click on a video, or on the icon of an audio clip, starts or pauses it, as in the
// PowerPoint slide show.
document.addEventListener("click", function (e) {
  if (!e.target || !e.target.closest || isPreview()) return;
  var m = e.target.closest("video[data-df-pptx-media]");
  if (!m) { var g = e.target.closest("g[data-sid]"); m = g && g.querySelector("audio[data-df-pptx-media]"); }
  if (!m) return;
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
      if (/^playFrom/.test(b.cmd) || (/^togglePause/.test(b.cmd) && m.paused)) { var p = m.play(); if (p && p["catch"]) p["catch"](function () { }); }
      else if (/^togglePause/.test(b.cmd)) m.pause();
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
// TILE_TYPES are the transitions drawn with three.js (08-tiles.js).
var TILE_TYPES = { glitter: 1, honeycomb: 1, vortex: 1, shred: 1, ripple: 1, fracture: 1, crush: 1, curtains: 1, wind: 1,
  prestige: 1, airplane: 1, origami: 1 };

function pct(x, y) { return x + "% " + y + "%"; }

// corner returns a polygon that covers the slide behind a diagonal edge moving from one corner
// (sx, sy: -1 or 1) across the slide; e runs from 0 to 2.
function corner(sx, sy, e) {
  var x0 = sx < 0 ? 100 : 0, y0 = sy < 0 ? 100 : 0, d = e * 100;
  var px = x0 + (sx < 0 ? -d : d), py = y0 + (sy < 0 ? -d : d);
  return [pct(x0, y0), pct(px, y0), pct(x0, py)].join(",");
}

// bands returns a polygon of bands across the slide (blinds, checker, comb, random bars): each
// band opens with progress p; the bands are joined by zero-width bridges along one edge.
function bands(type, dir, p) {
  var vert = dir === "vert", n = type === "checker" ? 10 : type === "randomBar" ? 24 : 8, pts = [];
  for (var i = 0; i < n; i++) {
    var a0 = i * 100 / n, a1 = (i + 1) * 100 / n, f = p;
    if (type === "randomBar") { var r = Math.sin(i * 91.7) * 43758.5; r -= Math.floor(r); f = p >= r ? 1 : 0; }
    if (type === "checker") f = Math.min(1, Math.max(0, p * 2 - (i % 2) * 0.5));
    var lo, hi;
    if (type === "comb") { lo = i % 2 ? 100 - f * 100 : 0; hi = i % 2 ? 100 : f * 100; }
    else if (type === "randomBar") { lo = 0; hi = f * 100; }
    else { lo = 0; hi = f * 100; a1 = a0 + (a1 - a0) * (type === "checker" ? 1 : f); hi = type === "checker" ? hi : 100; }
    var q = vert ? [[a0, lo], [a1, lo], [a1, hi], [a0, hi]] : [[lo, a0], [hi, a0], [hi, a1], [lo, a1]];
    pts.push(vert ? [a0, 0] : [0, a0]);
    q.forEach(function (c) { pts.push(c); });
    pts.push(q[0]);
    pts.push(vert ? [a0, 0] : [0, a0]);
  }
  return pts.map(function (c) { return pct(c[0], c[1]); }).join(",");
}

// sectors returns a polygon of sectors around the center (wheel with spokes, wedge).
function sectors(type, spokes, p) {
  var pts = [], n = type === "wedge" ? 1 : spokes, R = 75;
  for (var s = 0; s < n; s++) {
    var a0 = s * 2 * Math.PI / n, sweep = 2 * Math.PI / n * p;
    if (type === "wedge") { a0 = -Math.PI / 2 - Math.PI * p; sweep = 2 * Math.PI * p; }
    if (type === "wheelReverse") sweep = -sweep;
    pts.push([50, 50]);
    for (var k = 0; k <= 12; k++) {
      var t = a0 + sweep * k / 12;
      pts.push([50 + R * Math.cos(t), 50 + R * Math.sin(t)]);
    }
    pts.push([50, 50]);
  }
  return pts.map(function (c) { return pct(c[0], c[1]); }).join(",");
}

// morphPairs pairs the objects of two slides by their morph key (data-mk, written by the
// renderer), each object at most once, in document order.
function morphPairs(a, b) {
  var box = function (g) { return (g.getAttribute("data-box") || "0 0 0 0 0").split(" ").map(parseFloat); };
  var free = {};
  [].forEach.call(a.querySelectorAll("g[data-mk]"), function (g) {
    var k = g.getAttribute("data-mk");
    (free[k] = free[k] || []).push(g);
  });
  var out = [];
  [].forEach.call(b.querySelectorAll("g[data-mk]"), function (g) {
    var list = free[g.getAttribute("data-mk")];
    if (!list || !list.length) return;
    var ga = list.shift();
    if (ga.style.transform || g.style.transform) return; // animated objects keep their own motion
    out.push({ a: { el: ga, box: box(ga) }, b: { el: g, box: box(g) } });
  });
  return out;
}

// morphPlace draws an object (box x, y, w, h, rotation) in the box t.
function morphPlace(o, t) {
  var b = o.box;
  if (!(b[2] > 0) || !(b[3] > 0)) return;
  var cx = b[0] + b[2] / 2, cy = b[1] + b[3] / 2, tx = t[0] + t[2] / 2, ty = t[1] + t[3] / 2;
  o.el.setAttribute("transform", "translate(" + tx + " " + ty + ") rotate(" + (t[4] - b[4]) + ") scale(" + t[2] / b[2] + " " + t[3] / b[3] + ") translate(" + -cx + " " + -cy + ")");
}

function transition(from, to, done) {
  var tr = ((deck.slides[to] || {}).anim || {}).transition;
  var a = sections[from], b = sections[to];
  if (!tr || !tr.dur || isPreview() || !a || !b || from === to) { done(); return; }
  var W = deck.width, H = deck.height, dir = String(tr.dir || "l").split("-")[0], inv = /-inv$/.test(tr.dir || "");
  a.classList.add("df-on"); b.classList.add("df-on");
  b.style.zIndex = "2"; a.style.zIndex = "1";
  if (TILE_TYPES[tr.type] && typeof Tiles !== "undefined") {
    var tl = Tiles.start(stage, a, b, tr, W, H, function () {
      playing = null;
      a.style.zIndex = ""; b.style.zIndex = "";
      done();
    });
    if (tl) { playing = { raf: 0, finish: tl.finish }; return; }
  }
  // 3D transitions: the slides turn in a space seen from the front
  stage.style.perspective = (2 * W) + "px";
  var halves = null;
  if (tr.type === "doors" || tr.type === "window") {
    // the old slide opens in two halves
    halves = [a, a.cloneNode(true)];
    halves[1].removeAttribute("data-index");
    a.parentNode.insertBefore(halves[1], a);
  }
  function vec(d) { return { l: [-1, 0], r: [1, 0], u: [0, -1], d: [0, 1], lu: [-1, -1], ru: [1, -1], ld: [-1, 1], rd: [1, 1] }[d] || [-1, 0]; }
  var v = vec(dir);
  var pairs = tr.type === "morph" ? morphPairs(a, b) : [];
  var set = function (p) {
    switch (tr.type) {
      case "morph":
        // the new slide fades in over the old one; objects found on both slides move, scale
        // and turn from their old to their new place together, so that the new one fades in
        // over the old one (which also blends their colors)
        a.style.zIndex = "1"; b.style.zIndex = "2";
        b.style.opacity = String(p);
        var q = p < 0.5 ? 2 * p * p : 1 - 2 * (1 - p) * (1 - p);
        pairs.forEach(function (m) {
          var t = {}, k;
          for (k = 0; k < 5; k++) t[k] = m.a.box[k] + (m.b.box[k] - m.a.box[k]) * q;
          morphPlace(m.a, t); morphPlace(m.b, t);
        });
        break;
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
      case "prism": // a cube turning
        var ax = dir === "u" || dir === "d" ? "X" : "Y", sg = dir === "r" || dir === "d" ? 1 : -1, depth = (ax === "Y" ? W : H) / 2;
        if (ax === "X") sg = -sg;
        if (inv) sg = -sg;
        a.style.transform = "translateZ(" + -depth + "px) rotate" + ax + "(" + sg * 90 * p + "deg) translateZ(" + depth + "px)";
        b.style.transform = "translateZ(" + -depth + "px) rotate" + ax + "(" + -sg * 90 * (1 - p) + "deg) translateZ(" + depth + "px)";
        a.style.backfaceVisibility = b.style.backfaceVisibility = "hidden";
        break;
      case "flip": case "switch":
        var fs = dir === "r" ? 1 : -1;
        var deg = tr.type === "flip" ? 180 : 90;
        a.style.transform = "rotateY(" + fs * deg * p + "deg)" + (tr.type === "switch" ? " translateX(" + fs * -W * 0.5 * p + "px)" : "");
        b.style.transform = "rotateY(" + -fs * deg * (1 - p) + "deg)" + (tr.type === "switch" ? " translateX(" + fs * W * 0.5 * (1 - p) + "px)" : "");
        a.style.backfaceVisibility = b.style.backfaceVisibility = "hidden";
        if (tr.type === "switch") { a.style.opacity = String(1 - p); b.style.opacity = String(p); }
        break;
      case "gallery": case "conveyor":
        var gs = dir === "r" ? 1 : -1, z = tr.type === "conveyor" ? -W * 0.3 * Math.sin(Math.PI * p) : 0;
        a.style.transform = "translateX(" + gs * W * 1.05 * p + "px) translateZ(" + z + "px) rotateY(" + -gs * 25 * p + "deg)";
        b.style.transform = "translateX(" + -gs * W * 1.05 * (1 - p) + "px) translateZ(" + z + "px) rotateY(" + gs * 25 * (1 - p) + "deg)";
        break;
      case "doors": case "window":
        var open = tr.type === "doors" ? 1 : -1;
        halves[0].style.clipPath = "inset(0 0 0 50%)"; halves[0].style.transformOrigin = "100% 50%";
        halves[1].style.clipPath = "inset(0 50% 0 0)"; halves[1].style.transformOrigin = "0 50%";
        halves[0].style.transform = "rotateY(" + -open * 90 * p + "deg)";
        halves[1].style.transform = "rotateY(" + open * 90 * p + "deg)";
        halves.forEach(function (h) { h.style.zIndex = "3"; });
        b.style.transform = "translateZ(" + -W * 0.3 * (1 - p) + "px)";
        break;
      case "pageCurlDouble": case "pageCurlSingle": case "peelOff":
        // the old slide turns away about its left edge
        a.style.transformOrigin = "0 50%";
        a.style.transform = "rotateY(" + -100 * p + "deg)";
        a.style.backfaceVisibility = "hidden";
        a.style.zIndex = "3";
        break;
      case "ferris":
        var fr = dir === "r" ? 1 : -1;
        a.style.transformOrigin = b.style.transformOrigin = "50% 250%";
        a.style.transform = "rotate(" + fr * 25 * p + "deg)"; a.style.opacity = String(1 - p);
        b.style.transform = "rotate(" + -fr * 25 * (1 - p) + "deg)"; b.style.opacity = String(p);
        break;
      case "pan":
        var pv = dir === "d" ? -1 : 1;
        a.style.transform = "translateY(" + -pv * H * p + "px) rotateX(" + pv * 20 * p + "deg)";
        b.style.transform = "translateY(" + pv * H * (1 - p) + "px) rotateX(" + -pv * 20 * (1 - p) + "deg)";
        break;
      case "flythrough":
        a.style.transform = "scale(" + (1 + p) + ")"; a.style.opacity = String(1 - p);
        b.style.transform = "scale(" + (0.5 + 0.5 * p) + ")"; b.style.opacity = String(p);
        break;
      case "warp":
        a.style.transform = "scale(" + (1 - 0.5 * p) + ")"; a.style.opacity = String(1 - p);
        b.style.transform = "scale(" + (1.5 - 0.5 * p) + ")"; b.style.opacity = String(p);
        break;
      case "flash":
        a.style.filter = "brightness(" + (1 + 4 * Math.min(1, p * 2)) + ")";
        b.style.opacity = String(Math.max(0, p * 2 - 1));
        b.style.filter = "brightness(" + (1 + 4 * (1 - Math.max(0, p * 2 - 1))) + ")";
        break;
      case "reveal":
        a.style.opacity = String(Math.max(0, 1 - 2 * p));
        b.style.opacity = String(Math.max(0, 2 * p - 1));
        stage.style.background = "#000";
        break;
      case "newsflash":
        b.style.transform = "rotate(" + 720 * (1 - p) + "deg) scale(" + Math.max(0.01, p) + ")";
        break;
      case "strips":
        var sx = dir.indexOf("l") >= 0 ? -1 : 1, sy = dir.indexOf("u") >= 0 ? -1 : 1, e = 2 * p;
        // a diagonal edge moving across the slide
        b.style.clipPath = "polygon(" + corner(sx, sy, e) + ")";
        break;
      case "blinds": case "checker": case "comb": case "randomBar":
        b.style.clipPath = "polygon(" + bands(tr.type, dir, p) + ")";
        break;
      case "wheel": case "wheelReverse": case "wedge":
        b.style.clipPath = "polygon(" + sectors(tr.type, tr.spokes || 4, p) + ")";
        break;
      case "diamond":
        var dd = p * 100;
        b.style.clipPath = "polygon(50% " + (50 - dd) + "%, " + (50 + dd) + "% 50%, 50% " + (50 + dd) + "%, " + (50 - dd) + "% 50%)";
        break;
      case "plus":
        var pl = 50 * (1 - p), pa = p * 50;
        b.style.clipPath = "polygon(" + [[pl, 0], [100 - pl, 0], [100 - pl, pl], [100, pl], [100, 100 - pl], [100 - pl, 100 - pl], [100 - pl, 100], [pl, 100], [pl, 100 - pl], [0, 100 - pl], [0, pl], [pl, pl]]
          .map(function (q) { return q[0] + "% " + q[1] + "%"; }).join(",") + ")";
        if (pa >= 50) b.style.clipPath = "";
        break;
      default: // fade and every transition deckfold has no drawing for
        b.style.opacity = String(p);
    }
  };
  var clear = function () {
    [a, b].forEach(function (s) {
      s.style.transform = ""; s.style.opacity = ""; s.style.clipPath = ""; s.style.zIndex = "";
      s.style.transformOrigin = ""; s.style.backfaceVisibility = ""; s.style.filter = "";
    });
    stage.style.perspective = ""; stage.style.background = "";
    if (halves && halves[1].parentNode) halves[1].parentNode.removeChild(halves[1]);
    pairs.forEach(function (m) { m.a.el.removeAttribute("transform"); m.b.el.removeAttribute("transform"); });
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
// Trigger steps played on the slide are dropped, as when the slide is shown again in PowerPoint.
function goTo(slide, step) {
  finishPlaying();
  slide = clampSlide(slide);
  step = Math.max(0, Math.min(stepsOf(slide) - 1, step | 0));
  cur = { slide: slide, step: step };
  animOf(slide).clearTriggers();
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
  restartGifs(to);
  draw3D(to);
  transition(from, to, function () {
    show(to);
    playStep(to, 0);
  });
  emit("df-pptx-change");
}

// A click on the shape of a trigger plays the next step of its interactive sequence.
function playTrigger(i, t) {
  finishPlaying();
  var a = animOf(i), from = a.stateAt(cur.step), step = a.nextTrigger(t, cur.step);
  var fired = [], fire = mediaCmd(fired), dur = (step && step.dur) || 0;
  if (dur <= 0) { a.show(a.atStep(step, 1e12, from, fire)); return; }
  var t0 = performance.now();
  var frame = function (now) {
    var el = now - t0;
    a.show(a.atStep(step, el, from, fire));
    if (el < dur) playing.raf = requestAnimationFrame(frame);
    else playing = null;
  };
  playing = { raf: requestAnimationFrame(frame), finish: function () { a.show(a.atStep(step, 1e12, from, fire)); } };
}

document.addEventListener("click", function (e) {
  if (isPreview()) return;
  var sec = sections[cur.slide];
  if (!sec || !e.target || !e.target.closest) return;
  var a = animOf(cur.slide);
  for (var g = e.target.closest("g[data-sid]"); g && sec.contains(g); g = g.parentNode && g.parentNode.closest ? g.parentNode.closest("g[data-sid]") : null) {
    var t = a.triggerIndex(g.getAttribute("data-sid"));
    if (t >= 0) { playTrigger(cur.slide, t); return; }
  }
});

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
