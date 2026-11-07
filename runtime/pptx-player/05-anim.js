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
