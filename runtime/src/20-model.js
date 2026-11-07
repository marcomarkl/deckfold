// State model (slide, step) per format, and adapters that drive the source player in the document.
// The runtime keeps the state itself; it never reads internal variables of the player.

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

DF.model = {
  first: function () { return chain({ slide: 0, step: 0 }); },
  last: function () { var n = slidesOf().length; return chain({ slide: Math.max(0, n - 1), step: 0 }); },
  goto: function (slide) { var n = slidesOf().length; return chain({ slide: Math.max(0, Math.min(n - 1, slide)), step: 0 }); },
  // next returns null at the end of the presentation.
  next: function (s) {
    var sl = slidesOf(), cur = sl[s.slide];
    if (!cur) return null;
    if (s.step < cur.steps - 1) return chain({ slide: s.slide, step: s.step + 1 });
    if (s.slide + 1 < sl.length) return chain({ slide: s.slide + 1, step: 0 });
    return null;
  },
  prev: function (s) {
    var sl = slidesOf();
    if (s.step > 0) {
      var st = s.step - 1, cur = sl[s.slide];
      // go back over auto steps to the last manual step
      while (st > 0 && cur.auto && cur.auto[st]) st--;
      return { slide: s.slide, step: st };
    }
    if (s.slide > 0) return { slide: s.slide - 1, step: Math.max(0, sl[s.slide - 1].steps - 1) };
    return null;
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
    // jump: slide via hash (the player goes to step 0), then manual steps up to the target.
    jump: function (target, current) {
      var self = this;
      var cur = current;
      var p = Promise.resolve();
      if (!cur || cur.slide !== target.slide || cur.step > target.step) {
        p = p.then(function () {
          if (location.hash === "#" + (target.slide + 1)) {
            // same hash: force a change through a neighbouring slide
            location.hash = "#" + (target.slide === 0 ? 2 : target.slide);
          }
          location.hash = "#" + (target.slide + 1);
          cur = chain({ slide: target.slide, step: 0 });
          return wait(80);
        });
      }
      return p.then(function loop() {
        if (!cur || cur.step >= target.step) return;
        self.forward();
        cur = chain({ slide: target.slide, step: cur.step + 1 });
        return wait(60).then(loop);
      });
    },
    mediaRoot: function (slide) { return document.querySelectorAll(".deck-slide")[slide] || null; }
  },
  keynote: {
    _ready: null,
    ready: function () {
      if (this._ready) return this._ready;
      this._ready = new Promise(function (res) {
        var done = false;
        document.addEventListener("ShowController:SlideIndexDidChangeEvent", function () { if (!done) { done = true; setTimeout(res, 300); } });
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
    mediaRoot: function () { return document.getElementById("stage") || document.body; }
  },
  generic: {
    ready: function () { return Promise.resolve(); },
    forward: function () { }, back: function () { },
    jump: function () { return Promise.resolve(); },
    mediaRoot: function () { return document.body; }
  }
};
DF.adapter = function () { return DF.adapters[DF.format()] || DF.adapters.generic; };

// Bring the player to a target state: single steps as a key press (animated), otherwise jump.
DF.drive = function (from, to) {
  var a = DF.adapter();
  if (!to) return Promise.resolve();
  if (from && DF.model.same(DF.model.next(from), to)) { a.forward(); return Promise.resolve(); }
  if (from && DF.model.same(DF.model.prev(from), to)) { a.back(); return Promise.resolve(); }
  return a.jump(to, from);
};
