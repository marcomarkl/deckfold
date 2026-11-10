// Key handling (capture phase on window) and the message channel between the windows.

function settings() {
  var key = "deckfold:" + ((DF.meta && DF.meta.deckHash) || "deck");
  return DF.merge(DF.cfg.defaults || {}, DF.ls.get(key) || {});
}
DF.settings = settings;
DF.saveSettings = function (cur) {
  var key = "deckfold:" + ((DF.meta && DF.meta.deckHash) || "deck");
  var d = DF.diff(DF.cfg.defaults || {}, cur);
  if (d) DF.ls.set(key, d); else DF.ls.del(key);
};

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

// Pointer input in the presentation (decision E15). A source player may move by itself on a click,
// on the wheel or on a swipe (the claude.ai player does all three, the Keynote player moves on a
// swipe), which would leave deckfold's state behind. These capture listeners are registered with
// the key listeners, before any source script: a click on the slide becomes "next" through deckfold,
// the wheel and touch moves never reach the player. A click on something that acts by itself (a
// link, a medium, a form field, a trigger of the PowerPoint player, deckfold's own controls) is left
// to its target; where the player would take it as a step as well (adapter.guardClicks), it is
// stopped on <html>, before the player's listener on the document.
var pointerRouter = null;
["click", "wheel", "touchstart", "touchmove", "touchend"].forEach(function (type) {
  window.addEventListener(type, function (e) { if (pointerRouter) pointerRouter(e); }, { capture: true, passive: false });
});
document.documentElement.addEventListener("click", function (e) { if (e.__dfOwn && pointerRouter) e.stopPropagation(); });

// opts: next() moves on; interactive(target) tells whether the player handles a click on target
// itself; guardClicks keeps such clicks away from the player's document listener; link(target)
// returns the target of a link to another slide under the click, which follow(target) then goes
// to (through deckfold's state, like every other move).
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
  pointerRouter = function (e) {
    if (e.__deckfold) return;
    if (e.type === "click") {
      if (e.button !== 0) return;
      if (own(e.target)) { if (opts.guardClicks) e.__dfOwn = true; return; }
      e.preventDefault(); e.stopImmediatePropagation();
      var to = opts.link ? opts.link(e.target) : null;
      if (to != null && opts.follow) opts.follow(to);
      else opts.next();
      return;
    }
    // wheel and touch moves: never to the player (a tap still ends in a click, see above)
    e.stopImmediatePropagation();
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
