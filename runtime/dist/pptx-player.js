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
          // an entrance starts from full opacity, even after an exit left the target at 0
          st.op = b.transition === "out" ? (st.op === undefined ? 1 : st.op) * (1 - p) : p;
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
  function effectEnd(e) {
    var end = 0;
    (e.bhv || []).forEach(function (b) {
      var d = (b.dur || 0) * (b.autoRev ? 2 : 1) * (b.repeat > 1 ? b.repeat : 1);
      end = Math.max(end, (b.begin || 0) + d);
    });
    return e.begin + end;
  }

  // expandIterate replaces an effect that runs word by word or letter by letter with one effect
  // per unit (g[data-w] or g[data-c] in the target, in document order), each iterDelay later.
  // A hidden target becomes hidden units, so that the units can appear one after the other.
  function expandIterate(el, data) {
    var hidden = {};
    (data.hidden || []).forEach(function (t) { hidden[t] = true; });
    var units = {};
    var expand = function (steps) {
      return (steps || []).map(function (st) {
        var effects = [];
        (st.effects || []).forEach(function (e) {
          var g = e.iterate && find(el, e.target);
          var list = g ? g.querySelectorAll(e.iterate === "lt" ? "g[data-c]" : "g[data-w]") : [];
          if (!list.length) { effects.push(e); return; }
          [].forEach.call(list, function (u, k) {
            var key = e.target + "#" + k, c = {};
            for (var p in e) c[p] = e[p];
            c.target = key; c.begin = e.begin + k * (e.iterDelay || 0);
            delete c.iterate;
            units[key] = u;
            if (hidden[e.target]) hidden[key] = true;
            effects.push(c);
          });
          if (hidden[e.target]) hidden[e.target] = "units";
        });
        var dur = st.dur || 0;
        effects.forEach(function (e) { dur = Math.max(dur, effectEnd(e)); });
        return { effects: effects, dur: dur };
      });
    };
    var out = {};
    for (var k in data) out[k] = data[k];
    out.steps = expand(data.steps);
    out.triggers = (data.triggers || []).map(function (tr) { return { shape: tr.shape, steps: expand(tr.steps) }; });
    out.hidden = Object.keys(hidden).filter(function (t) { return hidden[t] === true; });
    return { data: out, units: units };
  }

  function Slide(el, data, W, H) {
    var ex = expandIterate(el, data || { steps: [{ effects: [] }] });
    this.el = el; this.data = ex.data; this.W = W; this.H = H;
    this.targets = {}; this.vars = {};
    // trig: the trigger steps played so far, as {t: trigger index, k: its step, after: main step}
    this.trig = [];
    var self = this;
    var steps = (this.data.steps || []).slice();
    (this.data.triggers || []).forEach(function (tr) { steps = steps.concat(tr.steps || []); });
    steps.forEach(function (st) {
      (st.effects || []).forEach(function (e) {
        if (!(e.target in self.targets)) self.targets[e.target] = ex.units[e.target] || find(el, e.target);
      });
    });    // trigger shapes show the hand cursor, as in the PowerPoint slide show
    (this.data.triggers || []).forEach(function (tr) { var g = find(el, tr.shape); if (g) g.style.cursor = "pointer"; });
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
  Slide.prototype.at = function (k, t, from, fire) { return this.atStep((this.data.steps || [])[k], t, from, fire); };
  Slide.prototype.atStep = function (step, t, from, fire) {
    var st = {}, self = this;
    for (var key in from) { st[key] = copy(from[key]); var s0 = st[key]; s0._mx0 = s0.tx; s0._my0 = s0.ty; s0._r0 = s0.rot; s0._sx0 = s0.sx; s0._sy0 = s0.sy; }
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
  // the state after main step `step`, including the trigger steps played after it
  Slide.prototype.stateAt = function (step) {
    var s = this.initial(), self = this;
    for (var k = 0; k <= step; k++) {
      s = this.after(k, s);
      this.trig.forEach(function (p) { if (p.after === k) s = self.atStep(self.data.triggers[p.t].steps[p.k], 1e12, s); });
    }
    return s;
  };
  // triggerIndex returns the index of the trigger started by a click on shape sid, or -1.
  Slide.prototype.triggerIndex = function (sid) {
    var tr = this.data.triggers || [];
    for (var i = 0; i < tr.length; i++) if (tr[i].shape === String(sid)) return i;
    return -1;
  };
  // nextTrigger records the next step of trigger i (the sequence starts over after its last
  // step) and returns it.
  Slide.prototype.nextTrigger = function (i, mainStep) {
    var n = 0;
    this.trig.forEach(function (p) { if (p.t === i) n++; });
    var p = { t: i, k: n % this.data.triggers[i].steps.length, after: mainStep };
    this.trig.push(p);
    return this.data.triggers[i].steps[p.k];
  };
  Slide.prototype.clearTriggers = function () { this.trig = []; };
  Slide.prototype.show = function (state) {
    for (var t in this.targets) render(this.targets[t], state[t] || base());
  };
  Slide.prototype.reset = function () { for (var t in this.targets) render(this.targets[t], base()); };
  Slide.prototype.dur = function (k) { var s = (this.data.steps || [])[k]; return (s && s.dur) || 0; };

  return { Slide: Slide, evalExpr: evalExpr, progress: progress };
})();

// --- 07-three.js ---
// 3D shapes drawn with three.js (window.__dfThree, embedded only in decks with 3D content).
//
// The renderer writes every 3D shape as an SVG approximation and marks its path with
// data-df-pptx-3d (bevel, extrusion, contour, material, camera, light; see
// internal/pptx/render/threed.go). When WebGL is available, the player builds the shape as an
// extruded, bevelled mesh, renders it once into a picture and lays the picture over the shape;
// the path keeps only its outline. Previews and browsers without WebGL keep the approximation.

// hasWebGL tells whether the browser can create a WebGL context. It is asked before three.js
// creates a renderer, because three.js reports a failed context as console errors.
var hasWebGL = (function () {
  var known = null;
  return function () {
    if (known !== null) return known;
    known = false;
    try {
      var c = document.createElement("canvas");
      var gl = c.getContext("webgl2") || c.getContext("webgl");
      if (gl) {
        known = true;
        var lose = gl.getExtension("WEBGL_lose_context");
        if (lose) lose.loseContext();
      }
    } catch (e) {}
    return known;
  };
})();

var Shapes3D = (function () {
  var svgNS = "http://www.w3.org/2000/svg";
  var T = null, renderer = null, failed = false, probe = null;

  function ready() {
    if (failed) return false;
    T = window.__dfThree;
    if (!T || !hasWebGL()) { failed = true; return false; }
    if (renderer) return true;
    try {
      var c = document.createElement("canvas");
      renderer = new T.WebGLRenderer({ canvas: c, alpha: true, antialias: true, preserveDrawingBuffer: true });
      renderer.outputColorSpace = T.SRGBColorSpace;
      renderer.setClearColor(0x000000, 0);
    } catch (e) { failed = true; return false; }
    return true;
  }

  // probePath returns a detached path element (in an invisible svg) to measure path data.
  function probePath(d) {
    if (!probe) {
      var svg = document.createElementNS(svgNS, "svg");
      svg.setAttribute("style", "position:absolute;width:0;height:0;overflow:hidden");
      probe = document.createElementNS(svgNS, "path");
      svg.appendChild(probe);
      (document.body || document.documentElement).appendChild(svg);
    }
    probe.setAttribute("d", d);
    return probe;
  }

  // subpaths turns absolute path data (M L H V C Q A Z, as the renderer writes it) into one
  // polygon per subpath (slide px). Corners stay exact; curves are sampled every 1.5 px.
  function subpaths(d) {
    var tok = String(d).match(/[MLHVCQAZ]|-?\d*\.?\d+(?:e[-+]?\d+)?/gi) || [];
    var out = [], cur = null, x = 0, y = 0, i = 0, cmd = "";
    var num = function () { return parseFloat(tok[i++]); };
    var add = function (px, py) { if (cur) cur.push({ x: px, y: py }); x = px; y = py; };
    var curve = function (fn, len) {
      var n = Math.max(4, Math.min(400, Math.ceil(len / 1.5)));
      for (var k = 1; k <= n; k++) { var q = fn(k / n); add(q.x, q.y); }
    };
    while (i < tok.length) {
      if (/^[A-Z]$/i.test(tok[i])) cmd = tok[i++].toUpperCase();
      var x0 = x, y0 = y;
      switch (cmd) {
        case "M":
          if (cur && cur.length >= 3) out.push(cur);
          cur = []; add(num(), num()); cmd = "L"; break;
        case "L": add(num(), num()); break;
        case "H": add(num(), y); break;
        case "V": add(x, num()); break;
        case "C":
          var c1x = num(), c1y = num(), c2x = num(), c2y = num(), ex = num(), ey = num();
          curve(function (t) {
            var u = 1 - t;
            return { x: u * u * u * x0 + 3 * u * u * t * c1x + 3 * u * t * t * c2x + t * t * t * ex,
              y: u * u * u * y0 + 3 * u * u * t * c1y + 3 * u * t * t * c2y + t * t * t * ey };
          }, Math.hypot(c1x - x0, c1y - y0) + Math.hypot(c2x - c1x, c2y - c1y) + Math.hypot(ex - c2x, ey - c2y));
          break;
        case "Q":
          var qx = num(), qy = num(), qx2 = num(), qy2 = num();
          curve(function (t) {
            var u = 1 - t;
            return { x: u * u * x0 + 2 * u * t * qx + t * t * qx2, y: u * u * y0 + 2 * u * t * qy + t * t * qy2 };
          }, Math.hypot(qx - x0, qy - y0) + Math.hypot(qx2 - qx, qy2 - qy));
          break;
        case "A":
          var rx = num(), ry = num(), rot = num(), large = num(), sweep = num(), ax = num(), ay = num();
          var el = probePath("M" + x0 + " " + y0 + "A" + rx + " " + ry + " " + rot + " " + large + " " + sweep + " " + ax + " " + ay), len = 0;
          try { len = el.getTotalLength(); } catch (e) { }
          if (len > 0) curve(function (t) { var q = el.getPointAtLength(len * t); return { x: q.x, y: q.y }; }, len);
          add(ax, ay);
          break;
        case "Z":
          if (cur && cur.length >= 3) out.push(cur);
          if (cur && cur.length) { x = cur[0].x; y = cur[0].y; }
          cur = null; break;
        default: i++;
      }
    }
    if (cur && cur.length >= 3) out.push(cur);
    // drop repeated points (the closing point of a curve onto the start)
    return out.map(function (p) {
      var q = p.filter(function (a, k) { var b = p[(k + 1) % p.length]; return Math.abs(a.x - b.x) > 1e-3 || Math.abs(a.y - b.y) > 1e-3; });
      return q;
    }).filter(function (p) { return p.length >= 3; });
  }

  function inside(pt, poly) {
    var c = false;
    for (var i = 0, j = poly.length - 1; i < poly.length; j = i++) {
      var a = poly[i], b = poly[j];
      if ((a.y > pt.y) !== (b.y > pt.y) && pt.x < (b.x - a.x) * (pt.y - a.y) / (b.y - a.y) + a.x) c = !c;
    }
    return c;
  }

  // shapesOf turns polygons into three.js shapes: a polygon inside an odd number of others is a
  // hole of the smallest one around it. y is flipped (SVG y points down).
  function shapesOf(polys) {
    var depth = polys.map(function (p, i) {
      var n = 0;
      polys.forEach(function (q, j) { if (i !== j && inside(p[0], q)) n++; });
      return n;
    });
    var toV = function (p) { return p.map(function (q) { return new T.Vector2(q.x, -q.y); }); };
    var shapes = [], owner = [];
    polys.forEach(function (p, i) {
      if (depth[i] % 2 === 0) { owner[i] = shapes.length; shapes.push(new T.Shape(toV(p))); }
    });
    polys.forEach(function (p, i) {
      if (depth[i] % 2 === 0) return;
      var best = -1;
      polys.forEach(function (q, j) {
        if (depth[j] === depth[i] - 1 && inside(p[0], q) && (best < 0 || polys[j].length < polys[best].length)) best = j;
      });
      if (best >= 0) shapes[owner[best]].holes.push(new T.Path(toV(p)));
    });
    return shapes;
  }

  // colorOf returns the fill color of a path: its own color, or the mean of a gradient's stops.
  function colorOf(path) {
    var f = path.getAttribute("fill") || "";
    if (/^#[0-9a-f]{6}$/i.test(f)) return f;
    var m = /^url\(#([^)]+)\)$/.exec(f);
    var g = m && document.getElementById(m[1]);
    if (!g) return null;
    var stops = g.querySelectorAll("stop"), r = 0, gg = 0, b = 0, n = 0;
    [].forEach.call(stops, function (s) {
      var c = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(s.getAttribute("stop-color") || "");
      if (!c) return;
      r += parseInt(c[1], 16); gg += parseInt(c[2], 16); b += parseInt(c[3], 16); n++;
    });
    if (!n) return null;
    var h = function (v) { return ("0" + Math.round(v / n).toString(16)).slice(-2); };
    return "#" + h(r) + h(gg) + h(b);
  }

  // camera presets as rotations (latitude, longitude, revolution) and projection
  var presets = {
    orthographicFront: [0, 0, 0],
    // isometric presets: the shape turns 45 degrees in its plane, then tilts by 54.74 degrees,
    // so that its face becomes the top (bottom, left or right) of a block (measured against the
    // PowerPoint reference of isometricTopUp; the others follow by symmetry)
    isometricTopUp: { x: -54.74, z: 45 }, isometricTopDown: { x: -54.74, z: -45 },
    isometricBottomUp: { x: 54.74, z: 45 }, isometricBottomDown: { x: 54.74, z: -45 },
    isometricLeftUp: { y: 54.74, z: 45 }, isometricLeftDown: { y: 54.74, z: -45 },
    isometricRightUp: { y: -54.74, z: 45 }, isometricRightDown: { y: -54.74, z: -45 },
    isometricOffAxis1Left: [-20, 60, 0], isometricOffAxis1Right: [-20, -60, 0], isometricOffAxis1Top: [-60, 20, 0],
    isometricOffAxis2Left: [-20, 30, 0], isometricOffAxis2Right: [-20, -30, 0], isometricOffAxis2Top: [-60, -20, 0],
    isometricOffAxis3Left: [20, 60, 0], isometricOffAxis3Right: [20, -60, 0], isometricOffAxis3Bottom: [60, 20, 0],
    isometricOffAxis4Left: [20, 30, 0], isometricOffAxis4Right: [20, -30, 0], isometricOffAxis4Bottom: [60, -20, 0],
    obliqueTopLeft: [-15, 15, 0], obliqueTop: [-15, 0, 0], obliqueTopRight: [-15, -15, 0], obliqueLeft: [0, 15, 0],
    obliqueRight: [0, -15, 0], obliqueBottomLeft: [15, 15, 0], obliqueBottom: [15, 0, 0], obliqueBottomRight: [15, -15, 0],
    perspectiveFront: [0, 0, 0], perspectiveLeft: [0, 20, 0], perspectiveRight: [0, -20, 0],
    perspectiveAbove: [-20, 0, 0], perspectiveBelow: [20, 0, 0],
    perspectiveAboveLeftFacing: [-15, 25, 0], perspectiveAboveRightFacing: [-15, -25, 0],
    perspectiveContrastingLeftFacing: [-10, 40, 0], perspectiveContrastingRightFacing: [-10, -40, 0],
    perspectiveHeroicLeftFacing: [10, 35, 0], perspectiveHeroicRightFacing: [10, -35, 0],
    perspectiveHeroicExtremeLeftFacing: [15, 50, 0], perspectiveHeroicExtremeRightFacing: [15, -50, 0],
    perspectiveRelaxed: [-30, 0, 0], perspectiveRelaxedModerately: [-20, 0, 0]
  };

  function material(name, color, map) {
    var c = new T.Color(map ? "#ffffff" : color), o = { color: c };
    if (map) o.map = map;
    var ph = function (spec, shin) { o.specular = new T.Color(spec); o.shininess = shin; return new T.MeshPhongMaterial(o); };
    switch (name) {
      case "metal": return ph(0x999999, 60);
      case "softmetal": return ph(0x666666, 30);
      case "plastic": case "clear": case "translucentPowder": return ph(0x555555, 40);
      case "flat": return new T.MeshBasicMaterial(o);
      case "matte": case "powder": case "dkEdge": return new T.MeshLambertMaterial(o);
      default: return ph(0x222222, 15);
    }
  }

  // lights: a key light from the rig's direction and, except for the flat and single-light
  // rigs, a weaker fill light from the front right, as in the three-point rigs. The shares of
  // ambient, key and fill light add up to pi on a face turned to the viewer (three.js's Lambert
  // term divides by pi), so that such a face keeps its fill color.
  function lights(scene, d) {
    var dirs = { t: [0, 1], tl: [-1, 1], tr: [1, 1], l: [-1, 0], r: [1, 0], b: [0, -1], bl: [-1, -1], br: [1, -1] };
    var v = dirs[d.lightDir] || dirs.t, el = { harsh: 35, contrasting: 35, sunrise: 35, sunset: 35, soft: 55, balanced: 55, brightRoom: 55, glow: 55 }[d.light] || 45;
    var a = Math.atan2(v[1], v[0]) + (d.lightRev || 0) * Math.PI / 180, z = Math.sin(el * Math.PI / 180), xy = Math.cos(el * Math.PI / 180);
    if (d.light === "flat") { scene.add(new T.AmbientLight(0xffffff, Math.PI)); return; }
    var fillShare = /^(harsh|contrasting|flood)$/.test(d.light) ? 0.15 : 0.25, amb = 0.3, keyShare = 1 - amb - fillShare;
    var key = new T.DirectionalLight(0xffffff, Math.PI * keyShare / z);
    key.position.set(Math.cos(a) * xy, Math.sin(a) * xy, z);
    var f = new T.Vector3(0.7, 0.1, 0.7).normalize();
    var fill = new T.DirectionalLight(0xffffff, Math.PI * fillShare / f.z);
    fill.position.copy(f);
    scene.add(new T.AmbientLight(0xffffff, Math.PI * amb), key, fill);
  }

  // source returns what a 3D element is made of: a shape path (its fill color) or a picture
  // group (the clip path and the picture).
  function source(el) {
    if (el.tagName.toLowerCase() === "path") {
      var color = colorOf(el);
      return color ? { d: el.getAttribute("d"), color: color, transform: el.getAttribute("transform") } : null;
    }
    var clipped = el.querySelector("g[clip-path]"), image = el.querySelector("image");
    var m = clipped && /url\(#([^)]+)\)/.exec(clipped.getAttribute("clip-path"));
    var cp = m && document.getElementById(m[1]), cpath = cp && cp.querySelector("path");
    if (!cpath || !image || !image.getAttribute("href")) return null;
    var num = function (a) { return parseFloat(image.getAttribute(a)) || 0; };
    return { d: cpath.getAttribute("d"), href: image.getAttribute("href"), img: { x: num("x"), y: num("y"), w: num("width"), h: num("height") } };
  }

  // render draws one 3D element and lays the picture of it over the element. scale is the device
  // pixel size of one slide px; tex is the loaded picture of a picture element.
  function render(el, src, scale, tex) {
    var d;
    try { d = JSON.parse(el.getAttribute("data-df-pptx-3d")); } catch (e) { return; }
    var polys = subpaths(src.d);
    if (!polys.length) return;
    var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    polys.forEach(function (p) { p.forEach(function (q) { minX = Math.min(minX, q.x); minY = Math.min(minY, q.y); maxX = Math.max(maxX, q.x); maxY = Math.max(maxY, q.y); }); });
    var bt = d.bevelT || d.bevelB, depth = d.depth || 0;
    var bw = bt ? Math.min(bt.w, (maxX - minX) / 2 - 0.5, (maxY - minY) / 2 - 0.5) : 0;
    var opts = {
      depth: Math.max(depth, 0.01), steps: 1, curveSegments: 1,
      bevelEnabled: !!bt && bw > 0, bevelThickness: bt ? bt.h : 0, bevelSize: Math.max(0, bw), bevelOffset: -Math.max(0, bw),
      bevelSegments: bt && /^(angle|coolSlant|slope|hardEdge)$/.test(bt.prst) ? 1 : 8
    };
    if (tex) {
      // the picture is projected from the front onto the face and the bevel
      var im = src.img;
      var uv = function (v, k) { return new T.Vector2((v[k * 3] - im.x) / im.w, (v[k * 3 + 1] + im.y + im.h) / im.h); };
      opts.UVGenerator = {
        generateTopUV: function (g, v, a, b, c) { return [uv(v, a), uv(v, b), uv(v, c)]; },
        generateSideWallUV: function (g, v, a, b, c, e) { return [uv(v, a), uv(v, b), uv(v, c), uv(v, e)]; }
      };
    }
    var geo = new T.ExtrudeGeometry(shapesOf(polys), opts);
    var scene = new T.Scene(), group = new T.Group();
    // caps take the fill (or the picture); sides, that is the bevel and the extrusion, the
    // extrusion color when there is one
    var side = depth > 0 && d.depthColor ? material(d.material, d.depthColor) : material(d.material, src.color, tex);
    var front = new T.Mesh(geo, [material(d.material, src.color, tex), side]);
    group.add(front);
    // turn the shape about its center as the camera preset (or explicit rotation) says
    // PowerPoint turns a shape about the center of its front face; the extrusion lies behind
    var cx = (minX + maxX) / 2, cy = -(minY + maxY) / 2, cz = Math.max(depth, 0.01) + (opts.bevelEnabled ? opts.bevelThickness : 0);
    front.position.set(-cx, -cy, -cz);
    var rot = d.rot || presets[d.camera] || [0, 0, 0], deg = Math.PI / 180;
    if (rot.length) {
      // a camera rotation (latitude, longitude, revolution): the shape turns with the latitude
      // and against longitude and revolution (signs checked against the PowerPoint reference)
      group.rotation.order = "ZYX";
      group.rotation.set(rot[0] * deg, -rot[1] * deg, -rot[2] * deg);
    } else {
      // a turn of the shape itself: first in its plane (z), then about x and y
      group.rotation.order = "XYZ";
      group.rotation.set((rot.x || 0) * deg, (rot.y || 0) * deg, (rot.z || 0) * deg);
    }
    scene.add(group);
    lights(scene, d);
    // fit the camera around the turned mesh
    var box = new T.Box3().setFromObject(group), m = 2;
    var w = box.max.x - box.min.x + 2 * m, h = box.max.y - box.min.y + 2 * m;
    var cam, mx = (box.min.x + box.max.x) / 2, my = (box.min.y + box.max.y) / 2;
    if (/^perspective/.test(d.camera)) {
      // the picture covers the field of view at the distance of the shape's center, so that the
      // shape keeps its size on the slide
      var fov = d.fov || 45, dist = Math.max(w, h) * 1.6, vis = 2 * dist * Math.tan(fov * Math.PI / 360);
      cam = new T.PerspectiveCamera(fov, w / h, 1, dist * 4);
      cam.position.set(mx, my, dist);
      h = Math.max(vis, h); w = h * (box.max.x - box.min.x + 2 * m) / (box.max.y - box.min.y + 2 * m);
      cam.aspect = w / h; cam.fov = 2 * Math.atan(h / 2 / dist) * 180 / Math.PI; cam.updateProjectionMatrix();
      box.min.x = mx - w / 2 + m; box.max.x = mx + w / 2 - m; box.min.y = my - h / 2 + m; box.max.y = my + h / 2 - m;
    } else {
      cam = new T.OrthographicCamera(box.min.x - m, box.max.x + m, box.max.y + m, box.min.y - m, -1e5, 1e5);
      cam.position.set(0, 0, 1e4);
    }
    var pw = Math.max(1, Math.min(4096, Math.round(w * scale))), ph = Math.max(1, Math.min(4096, Math.round(h * scale)));
    renderer.setSize(pw, ph, false);
    renderer.render(scene, cam);
    var url = renderer.domElement.toDataURL("image/png");
    geo.dispose();
    if (tex) tex.dispose();
    // the picture covers the area of the turned mesh, centered on the shape
    var img = document.createElementNS(svgNS, "image");
    img.setAttribute("x", String((box.min.x - m) + cx));
    img.setAttribute("y", String(-(box.max.y + m) - cy));
    img.setAttribute("width", String(w));
    img.setAttribute("height", String(h));
    img.setAttribute("preserveAspectRatio", "none");
    img.setAttribute("data-df-pptx-3d-image", "");
    if (src.transform) img.setAttribute("transform", src.transform);
    img.setAttribute("href", url);
    el.parentNode.insertBefore(img, el);
    if (src.href) {
      el.setAttribute("visibility", "hidden"); // the picture group
    } else {
      // the path keeps its outline
      el.setAttribute("fill", "none");
      el.removeAttribute("filter");
    }
    // the approximation of the extrusion behind it is no longer needed
    for (var s = img.previousSibling; s && s.getAttribute; s = s.previousSibling) {
      if (s.hasAttribute("data-df-pptx-3d-base")) s.setAttribute("visibility", "hidden");
    }
    el.setAttribute("data-df-pptx-3d-done", "");
  }

  // chartMeshes builds the plot of a 3D chart (data from chart3DAttr in render/chart2.go) in
  // plot coordinates: x to the right, y up from the bottom of the plot, z toward the viewer.
  function chartMeshes(d, group) {
    var W = d.plot[2], H = d.plot[3], ns = d.series.length, m = 0;
    d.series.forEach(function (v) { m = Math.max(m, v.length); });
    var mat = function (c) { return new T.MeshPhongMaterial({ color: new T.Color(c || "#4472c4"), specular: new T.Color(0x111111), shininess: 10 }); };
    var val = function (v) { return (v - d.lo) / ((d.hi - d.lo) || 1) * H; };
    if (d.type === "pie") {
      var vals = d.series[0] || [], total = 0;
      vals.forEach(function (v) { if (v > 0) total += v; });
      var rad = Math.min(W, H) / 2 * 0.9, a = Math.PI / 2;
      vals.forEach(function (v, i) {
        if (!(v > 0) || !total) return;
        var sw = v / total * 2 * Math.PI, sh = new T.Shape();
        sh.moveTo(0, 0);
        sh.absarc(0, 0, rad, a, a - sw, true);
        sh.lineTo(0, 0);
        var geo = new T.ExtrudeGeometry(sh, { depth: rad * 0.25, bevelEnabled: false, curveSegments: 48 });
        var mesh = new T.Mesh(geo, mat(d.colors[i]));
        mesh.position.set(W / 2, H / 2, 0);
        group.add(mesh);
        a -= sw;
      });
      return;
    }
    if (d.type === "surface") {
      var rows = ns, cols = m;
      var geo2 = new T.PlaneGeometry(W, W * 0.6, cols - 1, rows - 1);
      var pos = geo2.attributes.position, colors = [];
      for (var k = 0; k < pos.count; k++) {
        var ci = k % cols, ri = Math.floor(k / cols), v = (d.series[rows - 1 - ri] || [])[ci] || 0;
        pos.setZ(k, val(v));
        var band = Math.max(0, Math.floor((v - d.lo) / ((d.hi - d.lo) || 1) * 6));
        var c = new T.Color(d.colors[band % d.colors.length] || "#4472c4");
        colors.push(c.r, c.g, c.b);
      }
      geo2.setAttribute("color", new T.Float32BufferAttribute(colors, 3));
      geo2.computeVertexNormals();
      var surf = new T.Mesh(geo2, new T.MeshLambertMaterial({ vertexColors: true, side: T.DoubleSide }));
      surf.rotation.x = -Math.PI / 2;
      surf.position.set(W / 2, 0, -W * 0.3);
      group.add(surf);
      return;
    }
    var band = W / Math.max(1, m), depth = d.type === "bar" ? band * 0.5 : band * 0.4;
    if (d.type === "bar") {
      var bw = band / (d.stacked ? 1.6 : ns + 1.5);
      for (var i = 0; i < m; i++) {
        var base = 0;
        for (var si = 0; si < ns; si++) {
          var v1 = (d.series[si] || [])[i] || 0, h = Math.abs(val(v1) - val(0));
          if (h <= 0) continue;
          var box = new T.Mesh(new T.BoxGeometry(bw, h, depth), mat(d.colors[si]));
          var x = i * band + (d.stacked ? (band - bw) / 2 : bw * 0.75 + si * bw) + bw / 2;
          var y0 = d.stacked ? base : val(0);
          box.position.set(x, y0 + (v1 >= 0 ? h / 2 : -h / 2), 0);
          if (d.stacked) base += h;
          group.add(box);
        }
      }
      return;
    }
    // line and area: one extruded band per series, the later series further back
    d.series.forEach(function (vals2, si) {
      var sh = new T.Shape(), n = vals2.length;
      if (n < 2) return;
      var xs = function (i) { return (i + 0.5) * band; };
      if (d.type === "area") {
        sh.moveTo(xs(0), val(0));
        vals2.forEach(function (v, i) { sh.lineTo(xs(i), val(v)); });
        sh.lineTo(xs(n - 1), val(0));
      } else {
        var t = Math.max(3, H * 0.02);
        vals2.forEach(function (v, i) { if (i === 0) sh.moveTo(xs(i), val(v) + t); else sh.lineTo(xs(i), val(v) + t); });
        for (var j = n - 1; j >= 0; j--) sh.lineTo(xs(j), val(vals2[j]) - t);
      }
      var g = new T.ExtrudeGeometry(sh, { depth: depth, bevelEnabled: false });
      var mesh = new T.Mesh(g, mat(d.colors[si]));
      mesh.position.z = -(ns - 1 - si) * depth * 1.5;
      group.add(mesh);
    });
  }

  // renderChart draws the plot of a 3D chart and lays the picture over the flat drawing.
  function renderChart(el, scale) {
    var d;
    try { d = JSON.parse(el.getAttribute("data-df-pptx-chart3d")); } catch (e) { return; }
    var scene = new T.Scene(), group = new T.Group(), inner = new T.Group();
    chartMeshes(d, inner);
    var W = d.plot[2], H = d.plot[3];
    inner.position.set(-W / 2, -H / 2, 0);
    group.add(inner);
    group.rotation.order = "XYZ";
    group.rotation.set((d.rotX || 0) * Math.PI / 180, -(d.rotY || 0) * Math.PI / 180, 0);
    scene.add(group);
    lights(scene, { light: "threePt", lightDir: "t" });
    var box = new T.Box3().setFromObject(group), m = 4;
    var w = box.max.x - box.min.x + 2 * m, h = box.max.y - box.min.y + 2 * m;
    // fit into the plot area: the picture keeps its aspect ratio
    var k = Math.min(W / w, H / h, 1.2);
    var cam = new T.OrthographicCamera(box.min.x - m, box.max.x + m, box.max.y + m, box.min.y - m, -1e5, 1e5);
    cam.position.set(0, 0, 1e4);
    renderer.setSize(Math.max(1, Math.min(4096, Math.round(w * k * scale))), Math.max(1, Math.min(4096, Math.round(h * k * scale))), false);
    renderer.render(scene, cam);
    var url = renderer.domElement.toDataURL("image/png");
    scene.traverse(function (o) { if (o.geometry) o.geometry.dispose(); });
    var img = document.createElementNS(svgNS, "image");
    img.setAttribute("x", String(d.plot[0] + (W - w * k) / 2));
    img.setAttribute("y", String(d.plot[1] + (H - h * k) / 2));
    img.setAttribute("width", String(w * k));
    img.setAttribute("height", String(h * k));
    img.setAttribute("preserveAspectRatio", "none");
    img.setAttribute("data-df-pptx-3d-image", "");
    img.setAttribute("href", url);
    el.parentNode.insertBefore(img, el);
    el.setAttribute("visibility", "hidden");
    el.setAttribute("data-df-pptx-3d-done", "");
  }

  // slide draws the 3D elements of a slide that are not drawn yet; pictures once they have
  // loaded.
  function slide(section, scale) {
    var charts = section.querySelectorAll("[data-df-pptx-chart3d]:not([data-df-pptx-3d-done])");
    if (charts.length && ready()) {
      [].forEach.call(charts, function (el) { try { renderChart(el, scale); } catch (e) { el.setAttribute("data-df-pptx-3d-done", "failed"); } });
    }
    var els = section.querySelectorAll("[data-df-pptx-3d]:not([data-df-pptx-3d-done])");
    if (!els.length || !ready()) return;
    [].forEach.call(els, function (el) {
      var src = source(el);
      if (!src) { el.setAttribute("data-df-pptx-3d-done", "skipped"); return; }
      var draw = function (tex) {
        try { render(el, src, scale, tex); } catch (e) { el.setAttribute("data-df-pptx-3d-done", "failed"); }
      };
      if (!src.href) { draw(null); return; }
      el.setAttribute("data-df-pptx-3d-done", "loading");
      var im = new Image();
      im.onload = function () {
        var tex = new T.Texture(im);
        tex.colorSpace = T.SRGBColorSpace;
        tex.needsUpdate = true;
        draw(tex);
      };
      im.onerror = function () { el.setAttribute("data-df-pptx-3d-done", "failed"); };
      im.src = src.href;
    });
  }

  return { slide: slide, available: function () { return ready(); } };
})();

// --- 08-tiles.js ---
// Tile transitions drawn with three.js (glitter, honeycomb, vortex, shred, ripple, fracture,
// crush, curtains, wind, prestige, airplane, origami).
//
// Both slides are drawn into textures: each slide's SVG is copied with its pictures and fonts as
// data URLs, so that it can be drawn as a standalone image. The old slide is cut into tiles that
// show the new slide on their back; each transition moves the tiles in its own way in front of
// the new slide. Without WebGL, or when a slide cannot be drawn, the caller fades instead.

var Tiles = (function () {
  var T = null, renderer = null, failed = false;
  var dataURLs = {}; // blob URL -> data URL

  function ready() {
    if (failed) return false;
    T = window.__dfThree;
    if (!T || !hasWebGL()) { failed = true; return false; }
    if (renderer) return true;
    try {
      renderer = new T.WebGLRenderer({ alpha: true, antialias: true });
      renderer.outputColorSpace = T.SRGBColorSpace;
    } catch (e) { failed = true; return false; }
    return true;
  }

  function toDataURL(url) {
    if (!/^blob:/.test(url)) return Promise.resolve(url);
    if (dataURLs[url]) return Promise.resolve(dataURLs[url]);
    return fetch(url).then(function (r) { return r.blob(); }).then(function (b) {
      return new Promise(function (ok, fail) {
        var fr = new FileReader();
        fr.onload = function () { dataURLs[url] = fr.result; ok(fr.result); };
        fr.onerror = fail;
        fr.readAsDataURL(b);
      });
    });
  }

  // fontCSS returns the @font-face rules of the deck with data URLs.
  var fontCSS = null;
  function fonts() {
    if (fontCSS) return fontCSS;
    var rules = [];
    [].forEach.call(document.styleSheets, function (sh) {
      var list;
      try { list = sh.cssRules; } catch (e) { return; }
      [].forEach.call(list || [], function (r) { if (r.type === 5) rules.push(r.cssText); });
    });
    fontCSS = Promise.all(rules.map(function (css) {
      var m = /url\("?([^")]+)"?\)/.exec(css);
      return m ? toDataURL(m[1]).then(function (d) { return css.replace(m[1], d); }, function () { return ""; }) : Promise.resolve(css);
    })).then(function (list) { return list.join("\n"); });
    return fontCSS;
  }

  // snapshot draws the current state of a slide into a canvas.
  function snapshot(section, W, H, scale) {
    var svg = section.querySelector("svg");
    if (!svg) return Promise.reject(new Error("no slide"));
    var copy = svg.cloneNode(true);
    [].forEach.call(copy.querySelectorAll("foreignObject"), function (f) { f.parentNode.removeChild(f); });
    var imgs = [].slice.call(copy.querySelectorAll("image"));
    return Promise.all(imgs.map(function (im) {
      var h = im.getAttribute("href");
      return h ? toDataURL(h).then(function (d) { im.setAttribute("href", d); }, function () { }) : null;
    })).then(fonts).then(function (css) {
      var st = document.createElementNS("http://www.w3.org/2000/svg", "style");
      st.textContent = css;
      copy.insertBefore(st, copy.firstChild);
      copy.setAttribute("width", W); copy.setAttribute("height", H);
      var src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(new XMLSerializer().serializeToString(copy));
      return new Promise(function (ok, fail) {
        var im = new Image();
        im.onload = function () {
          var c = document.createElement("canvas");
          c.width = Math.round(W * scale); c.height = Math.round(H * scale);
          var g = c.getContext("2d");
          var bg = getComputedStyle(section).backgroundColor;
          g.fillStyle = bg && bg !== "rgba(0, 0, 0, 0)" ? bg : "#fff";
          g.fillRect(0, 0, c.width, c.height);
          g.drawImage(im, 0, 0, c.width, c.height);
          ok(c);
        };
        im.onerror = function () { fail(new Error("slide could not be drawn")); };
        im.src = src;
      });
    });
  }

  function rnd(i) { var x = Math.sin(i * 12.9898 + 78.233) * 43758.5453; return x - Math.floor(x); }
  function clamp(v) { return v < 0 ? 0 : v > 1 ? 1 : v; }
  function ease(v) { v = clamp(v); return v * v * (3 - 2 * v); }

  // grid returns the tile layout of a transition: columns and rows.
  function grid(type) {
    switch (type) {
      case "shred": case "curtains": return [24, 1];
      case "prestige": return [12, 1];
      default: return [16, 9];
    }
  }

  // place sets tile k (center x, y in slide px, size w, h) at progress p of the transition.
  function place(type, dir, m, t, p, W, H) {
    var u = t.cx / W, v = t.cy / H, r = rnd(t.k), q;
    var back = function () { return dir === "l" || dir === "u" ? 1 - u : u; };
    m.position.set(t.cx - W / 2, H / 2 - t.cy, 0);
    m.rotation.set(0, 0, 0);
    m.scale.set(1, 1, 1);
    switch (type) {
      case "glitter":
        q = ease((p - back() * 0.6 - r * 0.15) / 0.25);
        m.rotation.y = Math.PI * q;
        break;
      case "prestige":
        q = ease((p - back() * 0.6) / 0.4);
        m.rotation.x = Math.PI * q;
        m.position.z = Math.sin(Math.PI * q) * 80;
        break;
      case "ripple":
        q = ease((p - Math.hypot(u - 0.5, v - 0.5) * 1.2) / 0.3);
        m.rotation.y = Math.PI * q;
        m.position.z = Math.sin(Math.PI * q) * 60;
        break;
      case "honeycomb":
        q = ease((p - r * 0.5) / 0.5);
        m.position.z = q * 900;
        m.rotation.z = q * (r - 0.5) * 3;
        m.scale.setScalar(1 - 0.5 * q);
        m.visible = q < 0.98;
        break;
      case "vortex":
        q = ease((p - back() * 0.4 - r * 0.2) / 0.4);
        var a = q * Math.PI * 3, rad = q * Math.max(W, H);
        m.position.x += Math.cos(a + r * 6) * rad;
        m.position.y += Math.sin(a + r * 6) * rad;
        m.position.z = q * 300;
        m.rotation.z = a;
        break;
      case "shred":
        q = ease((p - r * 0.3) / 0.7);
        m.position.y -= q * H * 1.3 * (t.k % 2 ? 1 : 1.2);
        m.rotation.x = q * (t.k % 2 ? 1 : -1);
        break;
      case "fracture": case "crush":
        q = ease((p - r * 0.4) / 0.6);
        if (type === "crush") { m.scale.setScalar(1 - q); m.position.x *= 1 - q; m.position.y *= 1 - q; }
        else { m.position.y -= q * q * H * 1.5; m.rotation.z = q * (r - 0.5) * 4; m.rotation.x = q * r * 3; }
        break;
      case "curtains":
        q = ease(p);
        var side = u < 0.5 ? -1 : 1;
        m.position.x += side * q * W * 0.6 * (1 - Math.abs(u - 0.5));
        m.rotation.y = side * q * 1.2;
        m.scale.y = 1 + q * 0.05;
        break;
      case "wind":
        q = ease((p - back() * 0.5 - r * 0.2) / 0.5);
        var s = dir === "l" ? -1 : 1;
        m.position.x += s * q * W * 1.2;
        m.position.y += q * (r - 0.3) * H * 0.4;
        m.rotation.z = s * q * (r + 0.5);
        break;
      default: // airplane, origami
        q = ease((p - back() * 0.5) / 0.5);
        m.rotation.x = q * 1.5;
        m.rotation.y = q * (type === "airplane" ? 0.8 : -0.8);
        m.position.x += q * W * 0.8;
        m.position.y += q * H * 0.8;
        m.scale.setScalar(1 - 0.6 * q);
        break;
    }
  }

  // start plays a tile transition over the stage; returns false when it cannot be drawn, so
  // that the caller fades instead. done runs at the end.
  function start(stage, a, b, tr, W, H, done) {
    if (!ready()) return null;
    var scale = Math.min(2, Math.max(1, window.devicePixelRatio || 1));
    var cancelled = false, raf = 0, canvas = null, cleanup = function () { };
    var finish = function () {
      if (cancelled) return;
      cancelled = true;
      cancelAnimationFrame(raf);
      cleanup();
      done();
    };
    Promise.all([snapshot(a, W, H, scale), snapshot(b, W, H, scale)]).then(function (c) {
      if (cancelled) return;
      var ta = new T.CanvasTexture(c[0]), tb = new T.CanvasTexture(c[1]);
      ta.colorSpace = tb.colorSpace = T.SRGBColorSpace;
      var scene = new T.Scene();
      var cam = new T.PerspectiveCamera(30, W / H, 1, 20000);
      cam.position.set(0, 0, H / 2 / Math.tan(15 * Math.PI / 180));
      // the new slide lies behind all tiles, also tiles that turn away behind its plane
      var full = new T.Mesh(new T.PlaneGeometry(W, H), new T.MeshBasicMaterial({ map: tb, depthWrite: false, depthTest: false }));
      full.renderOrder = -1;
      var flips = /^(glitter|prestige|ripple)$/.test(tr.type);
      if (!flips) scene.add(full);
      var g = grid(tr.type), tw = W / g[0], th = H / g[1], tiles = [];
      var matA = new T.MeshBasicMaterial({ map: ta, side: T.FrontSide });
      var matB = new T.MeshBasicMaterial({ map: tb, side: T.FrontSide });
      for (var j = 0; j < g[1]; j++) for (var i = 0; i < g[0]; i++) {
        var grp = new T.Group();
        var uv = function (geo, flipX) {
          var at = geo.attributes.uv;
          for (var n = 0; n < at.count; n++) {
            var x = at.getX(n), y = at.getY(n);
            if (flipX) x = 1 - x;
            at.setXY(n, (i + x) / g[0], 1 - (j + 1 - y) / g[1]);
          }
        };
        var fa = new T.PlaneGeometry(tw + 0.5, th + 0.5); uv(fa, false);
        grp.add(new T.Mesh(fa, matA));
        if (flips) {
          var fb = new T.PlaneGeometry(tw + 0.5, th + 0.5); uv(fb, true);
          var mb = new T.Mesh(fb, matB);
          mb.rotation.y = Math.PI;
          grp.add(mb);
        }
        scene.add(grp);
        tiles.push({ m: grp, k: j * g[0] + i, cx: (i + 0.5) * tw, cy: (j + 0.5) * th });
      }
      renderer.setPixelRatio(scale);
      renderer.setSize(W, H, false);
      canvas = renderer.domElement;
      canvas.style.cssText = "position:absolute;left:0;top:0;width:" + W + "px;height:" + H + "px;z-index:5";
      stage.appendChild(canvas);
      a.style.visibility = "hidden"; b.style.visibility = "hidden";
      cleanup = function () {
        if (canvas && canvas.parentNode) canvas.parentNode.removeChild(canvas);
        a.style.visibility = ""; b.style.visibility = "";
        ta.dispose(); tb.dispose();
        scene.traverse(function (o) { if (o.geometry) o.geometry.dispose(); });
      };
      var t0 = performance.now(), dir = String(tr.dir || "l").split("-")[0];
      var frame = function (now) {
        if (cancelled) return;
        var p = Math.min(1, (now - t0) / tr.dur);
        tiles.forEach(function (t) {
          t.m.visible = true;
          place(tr.type, dir, t.m, t, p, W, H);
        });
        renderer.render(scene, cam);
        if (p < 1) raf = requestAnimationFrame(frame);
        else finish();
      };
      raf = requestAnimationFrame(frame);
    }, function () {
      // a slide could not be drawn: fade instead
      if (cancelled) return;
      var t0 = performance.now();
      b.style.opacity = "0";
      var fade = function (now) {
        if (cancelled) return;
        var p = Math.min(1, (now - t0) / tr.dur);
        b.style.opacity = String(p);
        if (p < 1) raf = requestAnimationFrame(fade); else { b.style.opacity = ""; finish(); }
      };
      cleanup = function () { b.style.opacity = ""; };
      raf = requestAnimationFrame(fade);
    });
    return { finish: finish };
  }

  return { start: start, available: function () { return ready(); } };
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

})();
