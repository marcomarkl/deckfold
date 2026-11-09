// State model (slide, step) per format, and adapters that drive the source player in the document.
// The runtime keeps the state itself; it never reads internal variables of a source player. The
// player of PowerPoint decks belongs to deckfold and is driven through its public API.

function slidesOf() { return (DF.meta && DF.meta.deck && DF.meta.deck.slides) || []; }
DF.slides = slidesOf;
DF.format = function () { return (DF.meta && DF.meta.deck && DF.meta.deck.format) || "generic"; };

// chain: auto steps the source advances by itself (claude.ai: steps[].auto).
function chain(s) {
  var sl = slidesOf()[s.slide];
  if (!sl || !sl.auto) return s;
  var step = s.step;
  while (step + 1 < sl.steps && sl.auto[step + 1]) step++;
  return { slide: s.slide, step: step };
}

function rawPrev(s) {
  var sl = slidesOf();
  if (s.step > 0) {
    var st = s.step - 1, cur = sl[s.slide];
    // go back over auto steps to the last manual step
    while (st > 0 && cur.auto && cur.auto[st]) st--;
    return { slide: s.slide, step: st };
  }
  if (s.slide > 0) return { slide: s.slide - 1, step: Math.max(0, sl[s.slide - 1].steps - 1) };
  return null;
}
function hasAuto(i) { var sl = slidesOf()[i]; return !!(sl && sl.auto && sl.auto.some(Boolean)); }

DF.model = {
  first: function () { return chain({ slide: 0, step: 0 }); },
  last: function () { var n = slidesOf().length; return chain({ slide: Math.max(0, n - 1), step: 0 }); },
  goto: function (slide) { var n = slidesOf().length; return chain({ slide: Math.max(0, Math.min(n - 1, slide)), step: 0 }); },
  // end: slide i as it looks after all its steps (what the previews show)
  end: function (i) { var sl = slidesOf()[i] || {}; return { slide: i, step: Math.max(0, (sl.steps || 1) - 1) }; },
  // next returns null at the end of the presentation.
  next: function (s) {
    var sl = slidesOf(), cur = sl[s.slide];
    if (!cur) return null;
    if (s.step < cur.steps - 1) return chain({ slide: s.slide, step: s.step + 1 });
    if (s.slide + 1 < sl.length) return chain({ slide: s.slide + 1, step: 0 });
    return null;
  },
  // prev returns a state the source player rests in, like next: a state followed by an auto step
  // would not stay (the player runs on by itself), so back skips over whole auto runs. On a slide
  // whose steps after the first are all auto, back goes to the slide before.
  prev: function (s) {
    var c = rawPrev(s);
    while (c && DF.model.same(chain(c), s)) c = rawPrev(c);
    return c ? chain(c) : null;
  },
  same: function (a, b) { return a && b && a.slide === b.slide && a.step === b.step; }
};

// Synthetic key, marked so deckfold's own key handling lets it through.
function synthKey(key, keyCode, opts) {
  var init = { key: key, code: (opts && opts.code) || "", bubbles: true, cancelable: true, shiftKey: !!(opts && opts.shift) };
  var e = new KeyboardEvent("keydown", init);
  try {
    Object.defineProperty(e, "keyCode", { get: function () { return keyCode; } });
    Object.defineProperty(e, "which", { get: function () { return keyCode; } });
    Object.defineProperty(e, "charCode", { get: function () { return 0; } });
  } catch (err) { }
  e.__deckfold = true;
  (document.body || document.documentElement).dispatchEvent(e);
  var up = new KeyboardEvent("keyup", init);
  up.__deckfold = true;
  (document.body || document.documentElement).dispatchEvent(up);
}
DF.synthKey = synthKey;

function wait(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }
DF.wait = wait;

// Adapters inside the document with a running source player (roles audience and preview).
DF.adapters = {
  claude: {
    ready: function () { return Promise.resolve(); },
    forward: function () { synthKey("ArrowRight", 39); },
    back: function () { synthKey("ArrowLeft", 37); },
    // jump: slide via hash (the player goes to step 0), then single steps up to the target.
    // The player runs auto steps by itself, each after a timer of at least 140 ms, and a key
    // press during such a run moves it on by one step only. So when the target is the end of the
    // auto run that starts the slide, the audience sees the run play; otherwise every step is
    // pressed, 60 ms apart, which is faster than the player's own timer. A preview reaches the
    // target at once (jumpAtOnce).
    jump: function (target, current) {
      if (DF.role === "preview") return this.jumpAtOnce(target);
      var self = this;
      var cur = current;
      var p = Promise.resolve();
      var playRun = DF.role !== "preview" && DF.model.same(chain({ slide: target.slide, step: 0 }), target);
      if (!cur || cur.slide !== target.slide || cur.step > target.step) {
        p = p.then(function () {
          if (location.hash === "#" + (target.slide + 1)) {
            // same hash: force a change through a neighbouring slide
            location.hash = "#" + (target.slide === 0 ? 2 : target.slide);
          }
          location.hash = "#" + (target.slide + 1);
          cur = playRun ? target : { slide: target.slide, step: 0 };
          return wait(80);
        });
      }
      return p.then(function loop() {
        if (!cur || cur.step >= target.step) return;
        self.forward();
        cur = { slide: target.slide, step: cur.step + 1 };
        return wait(60).then(loop);
      });
    },
    // jumpAtOnce: the whole move in one task, so that the browser paints only the target state. A
    // real change of the address would deliver hashchange as a task of its own, with a paint of
    // step 0 before the steps; so the address is changed with replaceState and the player's
    // hashchange listener gets the event at once. The player goes to the slide of the address
    // only when it is another one, so the slide is entered through a neighbouring slide first;
    // then every step is pressed. The player's own timers for auto steps find the state changed
    // and do nothing.
    jumpAtOnce: function (target) {
      var enter = function (n) {
        // the full address of this document (about:srcdoc in a preview) with the new fragment
        try { history.replaceState(history.state, "", location.href.split("#")[0] + "#" + n); }
        catch (e) { return false; }
        window.dispatchEvent(new HashChangeEvent("hashchange"));
        return true;
      };
      var n = slidesOf().length;
      if (n > 1) enter(target.slide === 0 ? 2 : target.slide);
      if (!enter(target.slide + 1)) return this.jumpAtOnceFallback(target);
      for (var i = 0; i < target.step; i++) this.forward();
      return Promise.resolve();
    },
    // without replaceState (a sandboxed document) the address is changed for real
    jumpAtOnceFallback: function (target) {
      var self = this;
      return new Promise(function (res) {
        var on = function () { window.removeEventListener("hashchange", on); for (var i = 0; i < target.step; i++) self.forward(); res(); };
        window.addEventListener("hashchange", on);
        location.hash = "#" + (target.slide + 1);
      });
    },
    mediaRoot: function (slide) { return document.querySelectorAll(".deck-slide")[slide] || null; },
    // the player's click listener on the document moves it on any click outside links and text
    guardClicks: true,
    // shown: the slide the player shows, the one .deck-slide without data-off. In a viewport not
    // wider than high the player shows all slides in a scroll view; then there is no single one.
    shown: function () {
      var all = document.querySelectorAll(".deck-slide"), at = null, n = 0;
      for (var i = 0; i < all.length; i++) if (!all[i].hasAttribute("data-off")) { at = i; n++; }
      return n === 1 ? at : null;
    }
  },
  keynote: {
    _ready: null,
    _at: null,
    ready: function () {
      if (this._ready) return this._ready;
      var self = this;
      this._ready = new Promise(function (res) {
        var done = false;
        document.addEventListener("ShowController:SlideIndexDidChangeEvent", function (e) {
          if (e.detail && typeof e.detail.slideIndex === "number") self._at = e.detail.slideIndex;
          if (!done) { done = true; setTimeout(res, 300); }
        });
        setTimeout(function () { if (!done) { done = true; res(); } }, 15000);
      });
      return this._ready;
    },
    forward: function () { synthKey("ArrowRight", 39); },
    back: function () { synthKey("[", 219); },
    jump: function (target, current) {
      var self = this, cur = current;
      var p = Promise.resolve();
      if (!cur || cur.slide !== target.slide || cur.step > target.step) {
        p = p.then(function () {
          String(target.slide + 1).split("").forEach(function (d) { synthKey(d, 48 + parseInt(d, 10)); });
          synthKey("Enter", 13);
          cur = { slide: target.slide, step: 0 };
          return wait(400);
        });
      }
      return p.then(function loop() {
        if (!cur || cur.step >= target.step) return;
        self.forward();
        cur = { slide: target.slide, step: cur.step + 1 };
        return wait(250).then(loop);
      });
    },
    mediaRoot: function () { return document.getElementById("stage") || document.body; },
    // shown: the slide index of the player's last slide change event. A self-playing export (its
    // steps carry timings) may move on by itself, which deckfold cannot tell from a drift, so it
    // is not compared there.
    shown: function () {
      var selfPlaying = slidesOf().some(function (s) { return s.delay && s.delay.some(function (d) { return d != null && d >= 0; }); });
      return selfPlaying ? null : this._at;
    }
  },
  // Decks generated from PowerPoint: deckfold's own player (runtime/pptx-player) with a public API.
  pptx: {
    _ready: null,
    ready: function () {
      if (this._ready) return this._ready;
      this._ready = new Promise(function (res) {
        if (window.__dfPptx) { res(); return; }
        window.addEventListener("df-pptx-ready", function () { res(); });
      });
      return this._ready;
    },
    forward: function () { if (window.__dfPptx) window.__dfPptx.next(); },
    back: function () { if (window.__dfPptx) window.__dfPptx.prev(); },
    jump: function (target) {
      if (window.__dfPptx) window.__dfPptx.goTo(target.slide, target.step);
      return Promise.resolve();
    },
    mediaRoot: function (slide) { return document.querySelectorAll(".df-pptx-slide")[slide] || document.body; },
    // videos, audio icons and trigger shapes act by themselves (their handlers are on the document)
    interactive: function (t) { return !!(window.__dfPptx && window.__dfPptx.interactive && window.__dfPptx.interactive(t)); },
    shown: function () { return window.__dfPptx ? window.__dfPptx.state().slide : null; }
  },
  generic: {
    ready: function () { return Promise.resolve(); },
    forward: function () { }, back: function () { },
    jump: function () { return Promise.resolve(); },
    mediaRoot: function () { return document.body; },
    shown: function () { return null; }
  }
};
DF.adapter = function () { return DF.adapters[DF.format()] || DF.adapters.generic; };

// Bring the player to a target state: single steps as a key press (animated), otherwise jump.
// On a claude.ai slide with auto steps the player may still be in its auto run, where a key
// press moves it by one step only, so a move away from such a slide is a jump. A preview always
// jumps, which reaches every state at once.
DF.drive = function (from, to) {
  var a = DF.adapter();
  if (!to) return Promise.resolve();
  // a preview shows every state at once, without the steps in between
  if (DF.role === "preview") return a.jump(to, from);
  var claude = DF.format() === "claude";
  if (claude && from && hasAuto(from.slide)) return a.jump(to, null);
  if (from && DF.model.same(DF.model.next(from), to)) { a.forward(); return Promise.resolve(); }
  if (from && DF.model.same(DF.model.prev(from), to)) { a.back(); return Promise.resolve(); }
  return a.jump(to, from);
};
