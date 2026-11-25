// Key handling (capture phase on window) and the message channel between the windows.

function settings() {
  var key = "deckfold:" + ((DF.meta && DF.meta.deckHash) || "deck");
  return DF.merge(DF.cfg.defaults || {}, DF.ls.get(key) || {});
}
DF.settings = settings;
// presenterElements: the panes of the presenter view a stored list names (E20). A list of v0.9.1 or
// before may name "current" (the pane on the left, now "audience") and "time" and "overview", which
// are always shown now; config.CleanElements does the same for the file's own settings.
DF.presenterElements = function (list) {
  var out = [];
  (list || []).forEach(function (k) {
    if (k === "current") k = "audience";
    if ((k === "audience" || k === "next" || k === "notes") && out.indexOf(k) < 0) out.push(k);
  });
  return out;
};
// saveSettings stores the differences from the defaults. The list of hidden slides is written only
// by saveHidden: the start screen and the presenter view each keep the settings they read, and a
// copy read before the list changed would otherwise put the old list back.
DF.saveSettings = function (cur, withHidden) {
  var key = "deckfold:" + ((DF.meta && DF.meta.deckHash) || "deck");
  cur = DF.clone(cur);
  if (!withHidden) {
    var stored = DF.ls.get(key) || {};
    if (Array.isArray(stored.hidden)) cur.hidden = stored.hidden; else delete cur.hidden;
  }
  var d = DF.diff(DF.cfg.defaults || {}, cur);
  if (d) DF.ls.set(key, d); else DF.ls.del(key);
};
// hiddenList: the slides hidden from the show (E21), as stored for this deck ("hidden" among the
// settings), otherwise the deck's own: the slides PowerPoint hides. Where deckfold cannot skip a
// slide (DF.canHide), none.
function deckHidden() {
  var out = [];
  DF.slides().forEach(function (s, i) { if (s.hidden) out.push(i); });
  return out;
}
DF.hiddenList = function () {
  if (!DF.canHide()) return [];
  var h = settings().hidden;
  return Array.isArray(h) ? h.slice() : deckHidden();
};
// saveHidden stores the list; a list that is the deck's own is not stored, so that the deck
// decides again.
DF.saveHidden = function (list) {
  var cur = settings(), sorted = list.slice().sort(function (a, b) { return a - b; });
  if (JSON.stringify(sorted) === JSON.stringify(deckHidden())) delete cur.hidden; else cur.hidden = sorted;
  DF.saveSettings(cur, true);
};

DF.actionFor = actionFor;
function actionFor(e) {
  var keys = settings().keys || {};
  for (var act in keys) {
    var list = keys[act] || [];
    for (var i = 0; i < list.length; i++) if (list[i] === e.key) return act;
  }
  return null;
}

// installKeys: handler(action, event) returns true when the key is consumed.
// Digits + Enter jump to a slide; the digit sequence expires after 1.5 s.
// The capture listeners are registered when the runtime loads (in <head>, before any source
// script), so no source player with its own capture listener gets a key before deckfold does.
var keyRouters = { keydown: null, keyup: null, keypress: null };
// DF.modalKeys(type, event): while an overlay of deckfold is open (the search), it takes every
// key. None reaches the handlers below or the source player: Escape closes the overlay instead
// of ending the presentation, and the arrow keys move in its list instead of moving the slides.
DF.modalKeys = null;
["keydown", "keyup", "keypress"].forEach(function (type) {
  window.addEventListener(type, function (e) {
    if (DF.modalKeys && !e.__deckfold) { DF.modalKeys(type, e); return; }
    if (keyRouters[type]) keyRouters[type](e);
  }, true);
});

// Pointer input in the presentation (decisions E15 and E22). A source player may move by itself on
// a click, on the wheel or on a swipe (the claude.ai player does all three, the Keynote player moves
// on a swipe), which would leave deckfold's state behind. These capture listeners are registered
// with the key listeners, before any source script: a click on the slide becomes "next" through
// deckfold, the wheel and touch moves never reach the player. A click on something that acts by
// itself (a link, a medium, a form field, a trigger of the PowerPoint player, deckfold's own
// controls) is left to its target; where the player would take it as a step as well
// (adapter.guardClicks), it is stopped on <html>, before the player's listener on the document.
// A swipe with one finger that begins on the slide is read here and handed to opts.swipe (E22):
// the player still never sees it. The wheel, and with it a swipe on a trackpad, does nothing.
var pointerRouter = null;
// What counts as a swipe: this far along one axis (CSS pixels), at least this many times as far as
// across it, and within this time. Usual values, not measured on a device.
var SWIPE_MIN = 48, SWIPE_RATIO = 2, SWIPE_MS = 800;
["click", "wheel", "touchstart", "touchmove", "touchend", "touchcancel"].forEach(function (type) {
  window.addEventListener(type, function (e) { if (pointerRouter) pointerRouter(e); }, { capture: true, passive: false });
});
document.documentElement.addEventListener("click", function (e) { if (e.__dfOwn && pointerRouter) e.stopPropagation(); });

// opts: next() moves on; interactive(target) tells whether the player handles a click on target
// itself; guardClicks keeps such clicks away from the player's document listener; link(target,
// event) returns the target of a link to another slide under the click, which follow(target)
// then goes to (through deckfold's state, like every other move); swipe(dir) takes a swipe to the
// "left", "right", "up" or "down".
DF.installPointer = function (opts) {
  function own(t) {
    if (!t || !t.closest) return true;
    if (t.closest(".df-blank.df-on, .df-endslide.df-on")) return false; // black, white, end: like "next"
    if (t.closest("[data-df-ui]")) return true;
    if (t.closest("a[href], video, audio, input, select, textarea, button, label, [contenteditable]")) return true;
    if (opts.interactive && opts.interactive(t)) return true;
    try { var sel = window.getSelection && window.getSelection(); if (sel && String(sel) !== "") return true; } catch (e) { }
    return false;
  }
  var touch = null;
  pointerRouter = function (e) {
    if (e.__deckfold) return;
    if (e.type === "click") {
      if (e.button !== 0) return;
      if (own(e.target)) { if (opts.guardClicks) e.__dfOwn = true; return; }
      e.preventDefault(); e.stopImmediatePropagation();
      var to = opts.link ? opts.link(e.target, e) : null;
      if (to != null && opts.follow) opts.follow(to);
      else opts.next();
      return;
    }
    // wheel and touch moves: never to the player (a tap still ends in a click, see above)
    e.stopImmediatePropagation();
    if (e.type === "wheel" || !opts.swipe) return;
    if (e.type === "touchstart") {
      // one finger on the slide begins a gesture; a second finger ends it (two fingers stay with
      // the browser), and so does a touch on something that acts by itself, such as the band of
      // the strip, which scrolls under the finger
      var t0 = e.touches && e.touches.length === 1 && !own(e.target) ? e.touches[0] : null;
      touch = t0 ? { x: t0.clientX, y: t0.clientY, lx: t0.clientX, ly: t0.clientY, at: Date.now() } : null;
      return;
    }
    if (!touch) return;
    if (e.type === "touchcancel") { touch = null; return; }
    if (e.type === "touchmove") {
      if (e.touches.length !== 1) { touch = null; return; }
      touch.lx = e.touches[0].clientX; touch.ly = e.touches[0].clientY;
      // from the first move on: once the browser has begun to scroll, a later preventDefault does
      // nothing. Without it a pull down may reload the page where the browser has that gesture
      // (not measured on a device)
      if (e.cancelable) e.preventDefault();
      return;
    }
    // touchend
    var g = touch, c = e.changedTouches && e.changedTouches[0];
    touch = null;
    if (e.touches && e.touches.length) return;
    var dx = (c ? c.clientX : g.lx) - g.x, dy = (c ? c.clientY : g.ly) - g.y, ax = Math.abs(dx), ay = Math.abs(dy);
    if (Date.now() - g.at > SWIPE_MS) return;
    var dir = ax >= SWIPE_MIN && ax >= SWIPE_RATIO * ay ? (dx < 0 ? "left" : "right")
      : ay >= SWIPE_MIN && ay >= SWIPE_RATIO * ax ? (dy < 0 ? "up" : "down") : null;
    if (!dir) return;
    if (e.cancelable) e.preventDefault(); // no click after a swipe
    opts.swipe(dir);
  };
};

// Keyboard Lock (Chrome, Edge): in full screen, Escape reaches the key handler above instead of only
// leaving full screen, so that one Escape ends the presentation; holding Escape still leaves full
// screen. Browsers without Keyboard Lock leave full screen on the first Escape as before.
document.addEventListener("fullscreenchange", function () {
  var kb = navigator.keyboard;
  if (!kb || !kb.lock) return;
  try {
    if (document.fullscreenElement) kb.lock(["Escape"]).catch(function () { });
    else kb.unlock();
  } catch (e) { }
});

// typing: keys go to a text or number field (not to a checkbox, whose focus must not stop the
// clicker after it was clicked).
function typing(t) {
  var tag = t && t.tagName;
  if (tag === "SELECT" || tag === "TEXTAREA") return true;
  return tag === "INPUT" && !/^(checkbox|radio|button|submit|range)$/i.test(t.type || "");
}

DF.installKeys = function (handler, opts) {
  var digits = "", timer = 0;
  keyRouters.keydown = function (e) {
    if (e.__deckfold) return; // eigene synthetische Taste
    // Ctrl+F and Cmd+F open the search (fixed, next to the keys bound to "find")
    if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && (e.key === "f" || e.key === "F") && !(opts && opts.passthrough)) {
      if (handler("find", e)) { e.preventDefault(); e.stopImmediatePropagation(); }
      return;
    }
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (typing(e.target)) return;
    if (opts && opts.passthrough) {
      // generic: only intercept deckfold's own keys
      var act0 = actionFor(e);
      if (act0 === "fullscreen" || act0 === "black" || act0 === "white" || act0 === "end") {
        if (handler(act0, e)) { e.preventDefault(); e.stopImmediatePropagation(); }
      } else if (opts.forward) opts.forward(e);
      return;
    }
    if (/^[0-9]$/.test(e.key)) {
      digits += e.key;
      clearTimeout(timer);
      timer = setTimeout(function () { digits = ""; handler("digits", { digits: "" }); }, 1500);
      handler("digits", { digits: digits });
      e.preventDefault(); e.stopImmediatePropagation();
      return;
    }
    if (e.key === "Enter" && digits) {
      var n = parseInt(digits, 10);
      digits = ""; clearTimeout(timer);
      handler("digits", { digits: "" });
      handler("goto", { slide: n - 1 });
      e.preventDefault(); e.stopImmediatePropagation();
      return;
    }
    var act = actionFor(e);
    // Enter without digits starts the presentation from the start screen (fixed, like digits +
    // Enter); unless the configuration binds it, it does nothing else
    if (!act && e.key === "Enter") act = "start";
    if (!act) {
      // Unbound keys do not reach the source player (its state would drift apart),
      // except in input fields (excluded above).
      if (opts && opts.swallowAll) { e.stopImmediatePropagation(); }
      return;
    }
    if (handler(act, e)) { e.preventDefault(); e.stopImmediatePropagation(); }
  };
  // keep keyup/keypress of bound keys away from the source player as well
  var guard = function (e) {
    if (e.__deckfold || (opts && opts.passthrough)) return;
    if (typing(e.target)) return;
    if (opts && opts.swallowAll) { e.stopImmediatePropagation(); return; }
    if (actionFor(e) || /^[0-9]$/.test(e.key) || e.key === "Enter") e.stopImmediatePropagation();
  };
  keyRouters.keyup = guard;
  keyRouters.keypress = guard;
};

// Channel: postMessage with sender and token checks (file:// has the origin "null").
DF.channel = function (getPeer, getToken, onMsg) {
  window.addEventListener("message", function (e) {
    var d = e.data, token = getToken();
    if (!token || !d || d.df !== "deckfold" || d.token !== token) return;
    var peer = getPeer();
    if (peer && e.source !== peer) return;
    onMsg(d, e.source);
  });
  return function send(msg) {
    var peer = getPeer();
    if (!peer || peer.closed) return false;
    msg.df = "deckfold"; msg.token = getToken();
    try { peer.postMessage(msg, "*"); return true; } catch (err) { return false; }
  };
};

// DF.mobile: a phone or tablet (iOS, iPadOS, Android), read from what the browser says of itself;
// an iPad calls itself a Mac and is told by its touch points (a Mac has none). There the show always
// runs in full screen and has no presenter view (decision E23).
DF.mobile = function () {
  var ua = navigator.userAgent || "";
  return /Android|iPhone|iPad|iPod/.test(ua) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
};
DF.requestFs = function (target, screen) {
  var elx = target || document.documentElement;
  if (!elx.requestFullscreen) return Promise.reject(new Error("no fullscreen"));
  try { return screen ? elx.requestFullscreen({ screen: screen }) : elx.requestFullscreen(); }
  catch (e) { return Promise.reject(e); }
};
DF.toggleFs = function () {
  if (document.fullscreenElement) return document.exitFullscreen();
  return DF.requestFs();
};
