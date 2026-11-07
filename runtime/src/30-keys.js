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
["keydown", "keyup", "keypress"].forEach(function (type) {
  window.addEventListener(type, function (e) { if (keyRouters[type]) keyRouters[type](e); }, true);
});

DF.installKeys = function (handler, opts) {
  var digits = "", timer = 0;
  keyRouters.keydown = function (e) {
    if (e.__deckfold) return; // eigene synthetische Taste
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    var tag = e.target && e.target.tagName;
    if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;
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
    var tag = e.target && e.target.tagName;
    if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;
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
