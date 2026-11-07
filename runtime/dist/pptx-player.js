/* deckfold PowerPoint player, generated from runtime/pptx-player, do not edit by hand */
(function(){"use strict";
// --- 05-anim.js ---
// Animation engine of the PowerPoint player.
//
// A target (a shape group g[data-sid] or a paragraph group g[data-p] inside it) has a state:
// visibility, opacity, translation, scale, rotation and a clip. The state after a step is computed
// from the behaviors of the step (internal/pptx/anim), so a jump to any step needs no animation:
// every behavior is applied with its end value. Playing a step evaluates the same behaviors over
// time with requestAnimationFrame.

var Anim = (function () {
  function base() { return { vis: true, op: 1, tx: 0, ty: 0, sx: 1, sy: 1, rot: 0, clip: "" }; }
  function copy(s) { var o = {}; for (var k in s) o[k] = s[k]; return o; }

  function find(slideEl, target) {
    var parts = String(target).split(":p");
    var g = slideEl.querySelector('g[data-sid="' + parts[0] + '"]');
    if (!g || parts.length < 2) return g;
    return g.querySelector('g[data-p="' + parts[1] + '"]') || g;
  }

  function render(el, st) {
    if (!el) return;
    el.style.visibility = st.vis ? "" : "hidden";
    el.style.opacity = st.op >= 0.999 ? "" : String(Math.max(0, st.op));
    var t = "";
    if (st.tx || st.ty) t += "translate(" + st.tx + "px," + st.ty + "px) ";
    if (st.rot) t += "rotate(" + st.rot + "deg) ";
    if (st.sx !== 1 || st.sy !== 1) t += "scale(" + st.sx + "," + st.sy + ")";
    el.style.transform = t;
    el.style.transformBox = t ? "fill-box" : "";
    el.style.transformOrigin = t ? "center" : "";
    el.style.clipPath = st.clip || "";
  }

  // --- small formula evaluator for #ppt_x, $, sin(...) etc. ---
  function evalExpr(src, vars) {
    var s = String(src).replace(/\s+/g, ""), i = 0;
    function peek() { return s[i]; }
    function num() {
      var m = /^(\d+\.?\d*|\.\d+)(e[+-]?\d+)?/i.exec(s.slice(i));
      if (!m) return NaN;
      i += m[0].length;
      return parseFloat(m[0]);
    }
    function ident() {
      var m = /^[#$a-z_][a-z0-9_]*/i.exec(s.slice(i));
      if (!m) return null;
      i += m[0].length;
      return m[0];
    }
    function atom() {
      var c = peek();
      if (c === "(") { i++; var v = add(); if (peek() === ")") i++; return v; }
      if (c === "-") { i++; return -atom(); }
      if (c === "+") { i++; return atom(); }
      if (/[0-9.]/.test(c)) return num();
      var id = ident();
      if (id === null) return NaN;
      if (peek() === "(") {
        i++;
        var args = [add()];
        while (peek() === ",") { i++; args.push(add()); }
        if (peek() === ")") i++;
        var f = { sin: Math.sin, cos: Math.cos, tan: Math.tan, abs: Math.abs, sqrt: Math.sqrt, ln: Math.log, exp: Math.exp,
          asin: Math.asin, acos: Math.acos, atan: Math.atan, max: Math.max, min: Math.min, rand: function (x) { return Math.random() * x; } }[id.toLowerCase()];
        return f ? f.apply(null, args) : NaN;
      }
      var l = id.toLowerCase();
      if (l === "pi") return Math.PI;
      if (l === "e") return Math.E;
      return id in vars ? vars[id] : (l in vars ? vars[l] : NaN);
    }
    function pow() { var v = atom(); while (peek() === "^") { i++; v = Math.pow(v, atom()); } return v; }
    function mul() { var v = pow(); while (peek() === "*" || peek() === "/") { var o = s[i++]; var r = pow(); v = o === "*" ? v * r : v / r; } return v; }
    function add() { var v = mul(); while (peek() === "+" || peek() === "-") { var o = s[i++]; var r = mul(); v = o === "+" ? v + r : v - r; } return v; }
    var out = add();
    return isFinite(out) ? out : NaN;
  }

  // progress of a behavior at local time t (ms), with accel/decel, auto reverse and repeat
  function progress(b, t) {
    var d = b.dur > 0 ? b.dur : 1;
    var reps = b.repeat === -1 ? 1 : (b.repeat > 1 ? b.repeat : 1);
    var total = d * (b.autoRev ? 2 : 1) * reps;
    if (t >= total) t = total;
    var cycle = d * (b.autoRev ? 2 : 1);
    var tc = t >= total ? (b.autoRev ? 0 : d) : t % cycle;
    var p = b.autoRev && tc > d ? (2 * d - tc) / d : Math.min(1, tc / d);
    if (t >= total) p = b.autoRev ? 0 : 1;
    var a = b.accel || 0, dc = b.decel || 0;
    if (a + dc > 0 && a + dc <= 1) {
      // constant acceleration, cruise, deceleration (SMIL accelerate/decelerate)
      var r = 1 / (1 - a / 2 - dc / 2);
      if (p < a) p = r * p * p / (2 * a);
      else if (p <= 1 - dc) p = r * (p - a / 2);
      else { var q = 1 - p; p = 1 - r * q * q / (2 * dc); }
    }
    return Math.max(0, Math.min(1, p));
  }

  function numOr(v, d) { var n = parseFloat(v); return isFinite(n) ? n : d; }

  // value of a p:anim at progress p
  function animValue(b, p, vars) {
    var keys = b.keys || [];
    function val(k, pp) {
      var raw = /^[-+]?(\d|\.\d)/.test(k.v) ? parseFloat(k.v) : evalExpr(k.v, vars);
      if (k.fmla) { var v2 = copy(vars); v2.$ = isFinite(raw) ? raw : pp; return evalExpr(k.fmla, v2); }
      return raw;
    }
    if (keys.length) {
      if (p <= keys[0].t) return val(keys[0], p);
      for (var i = 1; i < keys.length; i++) {
        if (p <= keys[i].t) {
          var a = keys[i - 1], c = keys[i];
          var f = c.t > a.t ? (p - a.t) / (c.t - a.t) : 1;
          var va = val(a, p), vc = val(c, p);
          if (!isFinite(va) || !isFinite(vc)) return f < 1 ? va : vc;
          return va + (vc - va) * f;
        }
      }
      return val(keys[keys.length - 1], p);
    }
    var from = b.from !== "" && b.from != null ? evalExpr(b.from, vars) : NaN;
    var to = b.to !== "" && b.to != null ? evalExpr(b.to, vars) : NaN;
    var by = b.by !== "" && b.by != null ? evalExpr(b.by, vars) : NaN;
    if (!isFinite(from)) from = NaN;
    if (isFinite(by) && !isFinite(to)) return { by: by * p };
    if (!isFinite(from) || !isFinite(to)) return to;
    return from + (to - from) * p;
  }

  var motionPaths = {};
  var svgNS = "http://www.w3.org/2000/svg";
  function motionPoint(path, p, W, H) {
    var key = path + "|" + W + "|" + H, el = motionPaths[key];
    if (!el) {
      // scale the path (fractions of the slide) to px; "E" ends a PowerPoint motion path
      var d = String(path).replace(/E/g, "").replace(/(-?\d*\.?\d+(?:e[-+]?\d+)?)\s+(-?\d*\.?\d+(?:e[-+]?\d+)?)/gi, function (m, x, y) {
        return (parseFloat(x) * W) + " " + (parseFloat(y) * H);
      });
      var svg = document.createElementNS(svgNS, "svg");
      svg.setAttribute("style", "position:absolute;width:0;height:0;overflow:hidden");
      el = document.createElementNS(svgNS, "path");
      el.setAttribute("d", d);
      svg.appendChild(el);
      (document.body || document.documentElement).appendChild(svg);
      motionPaths[key] = el;
    }
    try {
      // PowerPoint motion paths are offsets from the shape's own position
      var q = el.getPointAtLength(el.getTotalLength() * p);
      return { x: q.x, y: q.y };
    } catch (e) { return { x: 0, y: 0 }; }
  }

  function clipFor(filter, transition, p) {
    var m = /^(\w+)(?:\(([^)]*)\))?/.exec(filter || "") || [];
    var name = m[1] || "", arg = (m[2] || "").toLowerCase();
    if (transition === "out") p = 1 - p;
    var r = (1 - p) * 100;
    switch (name) {
      case "wipe":
        if (arg === "up") return "inset(" + r + "% 0 0 0)";
        if (arg === "left") return "inset(0 0 0 " + r + "%)";
        if (arg === "right") return "inset(0 " + r + "% 0 0)";
        return "inset(0 0 " + r + "% 0)";
      case "barn":
        if (arg.indexOf("horizontal") >= 0) return "inset(" + r / 2 + "% 0 " + r / 2 + "% 0)";
        return "inset(0 " + r / 2 + "% 0 " + r / 2 + "%)";
      case "box":
        return "inset(" + r / 2 + "%)";
      case "circle":
        return "circle(" + p * 75 + "% at 50% 50%)";
      case "diamond":
        var d = p * 100;
        return "polygon(50% " + (50 - d) + "%, " + (50 + d) + "% 50%, 50% " + (50 + d) + "%, " + (50 - d) + "% 50%)";
      case "plus":
        return "inset(" + r / 2 + "% " + r / 2 + "%)";
    }
    return "";
  }

  // apply one behavior at local time t to state st (vars: base geometry of the target)
  function applyBhv(b, t, st, vars, el, fire) {
    if (t < 0) return;
    var p = progress(b, t);
    var attr = (b.attr && b.attr[0]) || "";
    switch (b.type) {
      case "set":
        if (attr === "style.visibility") st.vis = b.to !== "hidden";
        else if (attr === "style.opacity") st.op = numOr(b.to, st.op);
        break;
      case "anim":
        var v = animValue(b, p, vars);
        if (v && typeof v === "object") {
          if (attr === "r" || attr === "style.rotation" || attr === "ppt_r") st.rot += v.by;
          break;
        }
        if (!isFinite(v)) break;
        if (attr === "ppt_x") st.tx = (v - vars["#ppt_x"]) * vars.W;
        else if (attr === "ppt_y") st.ty = (v - vars["#ppt_y"]) * vars.H;
        else if (attr === "ppt_w") st.sx = vars["#ppt_w"] > 0 ? v / vars["#ppt_w"] : 1;
        else if (attr === "ppt_h") st.sy = vars["#ppt_h"] > 0 ? v / vars["#ppt_h"] : 1;
        else if (attr === "style.opacity") st.op = v;
        else if (attr === "r" || attr === "style.rotation" || attr === "ppt_r") st.rot = v;
        break;
      case "effect":
        var name = (b.filter || "").split("(")[0];
        if (name === "fade" || name === "dissolve" || name === "" || name === "image") {
          st.op = (st.op === undefined ? 1 : st.op) * (b.transition === "out" ? 1 - p : p);
        } else {
          var c = clipFor(b.filter, b.transition, p);
          if (c) st.clip = p >= 1 && b.transition !== "out" ? "" : c;
          else st.op = b.transition === "out" ? 1 - p : p;
        }
        break;
      case "motion":
        var q = motionPoint(b.path, p, vars.W, vars.H);
        st.tx = (st._mx0 || 0) + q.x; st.ty = (st._my0 || 0) + q.y;
        break;
      case "rot":
        var by = numOr(b.by, 0);
        if (b.from !== "" && b.to !== "" && b.from != null && b.to != null && isFinite(parseFloat(b.to))) st.rot = numOr(b.from, 0) / 60000 + (numOr(b.to, 0) - numOr(b.from, 0)) / 60000 * p;
        else st.rot = (st._r0 || 0) + by * p;
        break;
      case "scale":
        var parts = String(b.by || b.to || "1 1").split(" ");
        var bx = numOr(parts[0], 1), byy = numOr(parts[1], bx);
        var s0x = st._sx0 || 1, s0y = st._sy0 || 1;
        if (b.by) { st.sx = s0x * (1 + (bx - 1) * p); st.sy = s0y * (1 + (byy - 1) * p); }
        else { st.sx = s0x + (bx - s0x) * p; st.sy = s0y + (byy - s0y) * p; }
        break;
      case "cmd":
        if (fire) fire(b, el);
        break;
    }
  }

  function varsFor(el, W, H) {
    var bb = { x: 0, y: 0, width: 0, height: 0 };
    try { bb = el.getBBox(); } catch (e) { }
    return { W: W, H: H, "#ppt_x": (bb.x + bb.width / 2) / W, "#ppt_y": (bb.y + bb.height / 2) / H,
      "#ppt_w": bb.width / W, "#ppt_h": bb.height / H, ppt_x: (bb.x + bb.width / 2) / W, ppt_y: (bb.y + bb.height / 2) / H,
      ppt_w: bb.width / W, ppt_h: bb.height / H };
  }

  // Slide holds the targets of one slide and computes states.
  function Slide(el, data, W, H) {
    this.el = el; this.data = data || { steps: [{ effects: [] }] }; this.W = W; this.H = H;
    this.targets = {}; this.vars = {};
    var self = this;
    (this.data.steps || []).forEach(function (st) {
      (st.effects || []).forEach(function (e) { if (!(e.target in self.targets)) self.targets[e.target] = find(el, e.target); });
    });
  }
  Slide.prototype.initial = function () {
    var s = {}, hidden = {};
    (this.data.hidden || []).forEach(function (t) { hidden[t] = true; });
    for (var t in this.targets) { s[t] = base(); if (hidden[t]) s[t].vis = false; }
    return s;
  };
  Slide.prototype.varsOf = function (t) {
    if (!this.vars[t]) {
      var el = this.targets[t];
      // measure without the effect of earlier animations
      var saved = el ? el.style.transform : "";
      if (el) el.style.transform = "";
      this.vars[t] = varsFor(el, this.W, this.H);
      if (el) el.style.transform = saved;
    }
    return this.vars[t];
  };
  // state at time t (ms) of step k, starting from state `from`
  Slide.prototype.at = function (k, t, from, fire) {
    var st = {}, self = this;
    for (var key in from) { st[key] = copy(from[key]); var s0 = st[key]; s0._mx0 = s0.tx; s0._my0 = s0.ty; s0._r0 = s0.rot; s0._sx0 = s0.sx; s0._sy0 = s0.sy; }
    var step = (this.data.steps || [])[k];
    if (!step) return st;
    (step.effects || []).forEach(function (e) {
      var s = st[e.target];
      if (!s) return;
      var vars = self.varsOf(e.target);
      e.bhv.forEach(function (b) { applyBhv(b, t - e.begin - b.begin, s, vars, self.targets[e.target], fire); });
    });
    return st;
  };
  Slide.prototype.after = function (k, from) { return this.at(k, 1e12, from); };
  Slide.prototype.stateAt = function (step) {
    var s = this.initial();
    for (var k = 0; k <= step; k++) s = this.after(k, s);
    return s;
  };
  Slide.prototype.show = function (state) {
    for (var t in this.targets) render(this.targets[t], state[t] || base());
  };
  Slide.prototype.reset = function () { for (var t in this.targets) render(this.targets[t], base()); };
  Slide.prototype.dur = function (k) { var s = (this.data.steps || [])[k]; return (s && s.dur) || 0; };

  return { Slide: Slide, evalExpr: evalExpr, progress: progress };
})();

// --- 10-player.js ---
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

})();
