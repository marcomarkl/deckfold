/* deckfold runtime, generated from runtime/src, do not edit by hand */
(function(){"use strict";
// --- 00-core.js ---
// Core: configuration, role, texts, storage, small helpers.
var DF = window.__deckfold = window.__deckfold || {};

function readJSON(id) {
  var el = document.getElementById(id);
  if (!el) return null;
  try { return JSON.parse(el.textContent); } catch (e) { console.error("deckfold: " + id + " nicht lesbar", e); return null; }
}

DF.cfg = readJSON("df-config") || { defaults: {} };
DF.version = DF.cfg.version;

// Role: "preview" (srcdoc iframe of the presenter view), "audience" (audience window),
// "start" (the window the file was opened in; it becomes the presenter view, or the solo
// audience view after a reload). The solo role is also kept in sessionStorage because WebKit
// drops window.name when a file:// page reloads.
function soloFlag() { try { return sessionStorage.getItem("df-solo"); } catch (e) { return null; } }
DF.role = (function () {
  try {
    if (window.frameElement && window.frameElement.hasAttribute("data-df-preview")) return "preview";
  } catch (e) { /* cross-origin */ }
  var n = window.name || "";
  if (n.indexOf("df-audience:") === 0 || n.indexOf("df-solo:") === 0 || soloFlag()) return "audience";
  return "start";
})();
DF.solo = DF.role === "audience" && ((window.name || "").indexOf("df-solo:") === 0 || (!!soloFlag() && (window.name || "").indexOf("df-audience:") !== 0));
DF.token = DF.role === "audience" ? ((window.name || "").split(":")[1] || soloFlag() || "") : "";
DF.setSolo = function (token) {
  try { if (token) sessionStorage.setItem("df-solo", token); else sessionStorage.removeItem("df-solo"); } catch (e) { }
  window.name = token ? "df-solo:" + token : "";
};

var LANG = String(navigator.language || "en").toLowerCase().indexOf("de") === 0 ? "de" : "en";
var TEXT = {
  de: {
    present: "Pr\u00e4sentieren", audienceOnly: "Nur Publikumsansicht", settings: "Einstellungen",
    slides: "Folien", generic: "Unbekanntes Format: ohne Referentenansicht, \u00dcbersicht und Auto-Modus.",
    screens: "Bildschirme", audienceScreen: "Publikum", presenterScreen: "Referent", presenterFs: "Referentenansicht im Vollbild",
    remember: "Auswahl merken", start: "Start", cancel: "Abbrechen", close: "Schlie\u00dfen", save: "Speichern",
    reset: "Zur\u00fccksetzen", screensHint: "Dieser Browser kann Fenster nicht selbst auf einen Bildschirm legen. Zieh das Publikumsfenster auf den Beamer und klick dort auf \u201eVollbild\u201c.",
    screensDenied: "Kein Zugriff auf die Bildschirme. Das Publikumsfenster \u00f6ffnet sich am Standardplatz; zieh es auf den Beamer.",
    presenterStays: "Die Referentenansicht bleibt, wo dieses Fenster liegt.",
    clickFs: "Klicken f\u00fcr Vollbild", fullscreen: "Vollbild", popupBlocked: "Das Publikumsfenster wurde blockiert.",
    openAudience: "Publikumsfenster \u00f6ffnen", audienceLost: "Publikumsfenster nicht erreichbar.", reopen: "Neu \u00f6ffnen",
    searching: "Suche Publikumsfenster \u2026", screensChanged: "Die Bildschirme haben sich ge\u00e4ndert.", chooseAgain: "Neu w\u00e4hlen",
    current: "Aktuell", next: "Als N\u00e4chstes", notes: "Notizen", time: "Zeit", overview: "\u00dcbersicht",
    end: "Ende", endShow: "Beenden", noNotes: "Keine Notizen", elapsed: "Verstrichen", remaining: "Rest",
    clock: "Uhr", pause: "Pause", resume: "Weiter", auto: "Auto", autoOn: "Auto an", autoOff: "Auto aus",
    elements: "Elemente der Referentenansicht", notesSize: "Schriftgr\u00f6\u00dfe Notizen (px)", target: "Zieldauer (Minuten, 0 = keine)",
    autoInterval: "Auto-Intervall (Sekunden)", autoLoop: "Auto-Modus in Schleife", autoStart: "Im Auto-Modus starten",
    endOfShow: "Ende der Pr\u00e4sentation", step: "Schritt", noPresenter: "F\u00fcr dieses Format gibt es keine Referentenansicht. Tasten werden an das Publikumsfenster weitergegeben.",
    offline: "Live-Inhalt \u2013 ohne Netz nicht verf\u00fcgbar", media: "Medien laufen",
    keysHelp: "Weiter: \u2192 \u2193 Bild\u2193 Leertaste \u00b7 Zur\u00fcck: \u2190 \u2191 Bild\u2191 \u00b7 Folie: Ziffern + Enter \u00b7 B schwarz \u00b7 W wei\u00df \u00b7 A Auto \u00b7 F Vollbild"
  },
  en: {
    present: "Present", audienceOnly: "Audience view only", settings: "Settings",
    slides: "slides", generic: "Unknown format: no presenter view, overview or auto mode.",
    screens: "Screens", audienceScreen: "Audience", presenterScreen: "Presenter", presenterFs: "Presenter view in full screen",
    remember: "Remember choice", start: "Start", cancel: "Cancel", close: "Close", save: "Save",
    reset: "Reset", screensHint: "This browser cannot place windows on a screen. Drag the audience window to the projector and click \u201cFull screen\u201d there.",
    screensDenied: "No access to the screens. The audience window opens at the default position; drag it to the projector.",
    presenterStays: "The presenter view stays where this window is.",
    clickFs: "Click for full screen", fullscreen: "Full screen", popupBlocked: "The audience window was blocked.",
    openAudience: "Open audience window", audienceLost: "Audience window not reachable.", reopen: "Reopen",
    searching: "Looking for the audience window \u2026", screensChanged: "The screen setup changed.", chooseAgain: "Choose again",
    current: "Current", next: "Next", notes: "Notes", time: "Time", overview: "Overview",
    end: "End", endShow: "End show", noNotes: "No notes", elapsed: "Elapsed", remaining: "Left",
    clock: "Clock", pause: "Pause", resume: "Resume", auto: "Auto", autoOn: "Auto on", autoOff: "Auto off",
    elements: "Presenter view elements", notesSize: "Notes font size (px)", target: "Target duration (minutes, 0 = none)",
    autoInterval: "Auto interval (seconds)", autoLoop: "Loop auto mode", autoStart: "Start in auto mode",
    endOfShow: "End of presentation", step: "Step", noPresenter: "This format has no presenter view. Keys are passed to the audience window.",
    offline: "Live content \u2013 not available offline", media: "media playing",
    keysHelp: "Next: \u2192 \u2193 PgDn Space \u00b7 Back: \u2190 \u2191 PgUp \u00b7 Slide: digits + Enter \u00b7 B black \u00b7 W white \u00b7 A auto \u00b7 F full screen"
  }
};
function t(k) { return (TEXT[LANG] && TEXT[LANG][k]) || TEXT.en[k] || k; }
DF.t = t;

function el(tag, attrs, children) {
  var e = document.createElement(tag);
  if (attrs) for (var k in attrs) {
    if (k === "text") e.textContent = attrs[k];
    else if (k === "class") e.className = attrs[k];
    else if (k.indexOf("on") === 0 && typeof attrs[k] === "function") e.addEventListener(k.slice(2), attrs[k]);
    else if (attrs[k] !== false && attrs[k] != null) e.setAttribute(k, attrs[k] === true ? "" : attrs[k]);
  }
  (children || []).forEach(function (c) { if (c != null) e.appendChild(typeof c === "string" ? document.createTextNode(c) : c); });
  return e;
}
DF.el = el;

// Storage: localStorage for settings (per deck), sessionStorage for the running session.
function store(kind) {
  return {
    get: function (k) { try { var v = window[kind].getItem(k); return v == null ? null : JSON.parse(v); } catch (e) { return null; } },
    set: function (k, v) { try { window[kind].setItem(k, JSON.stringify(v)); } catch (e) { /* private mode or blocked */ } },
    del: function (k) { try { window[kind].removeItem(k); } catch (e) { } }
  };
}
DF.ls = store("localStorage");
DF.ss = store("sessionStorage");

function clone(o) { return JSON.parse(JSON.stringify(o)); }
function isObj(o) { return o && typeof o === "object" && !Array.isArray(o); }
function merge(base, over) {
  var out = clone(base);
  if (!isObj(over)) return out;
  for (var k in over) out[k] = isObj(over[k]) && isObj(out[k]) ? merge(out[k], over[k]) : clone(over[k]);
  return out;
}
// diff returns only the differences from base (for localStorage).
function diff(base, cur) {
  var out = {}, any = false;
  for (var k in cur) {
    if (isObj(cur[k]) && isObj(base[k])) {
      var d = diff(base[k], cur[k]);
      if (d) { out[k] = d; any = true; }
    } else if (JSON.stringify(cur[k]) !== JSON.stringify(base[k])) { out[k] = clone(cur[k]); any = true; }
  }
  return any ? out : null;
}
DF.merge = merge; DF.diff = diff; DF.clone = clone;

function randomToken() {
  var a = new Uint8Array(12);
  (window.crypto || window.msCrypto).getRandomValues(a);
  return Array.prototype.map.call(a, function (b) { return ("0" + b.toString(16)).slice(-2); }).join("");
}
DF.randomToken = randomToken;

function fmtTime(ms) {
  var neg = ms < 0; ms = Math.abs(ms);
  var s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
  s = s % 60;
  var p = function (n) { return (n < 10 ? "0" : "") + n; };
  return (neg ? "\u2212" : "") + (h ? h + ":" + p(m) : p(m)) + ":" + p(s);
}
DF.fmtTime = fmtTime;

// --- 10-loader.js ---
// Loader: decode asset blocks, activate styles and attributes, shim runtime loads,
// start the deferred source scripts in their original order.
var PH = /deckfold:([0-9a-f]{20})/g;

DF.assets = (function () {
  var urls = {};
  function parts(h) {
    var first = document.getElementById("df-a-" + h + "-0");
    if (!first) return null;
    var n = parseInt(first.getAttribute("data-parts") || "1", 10), out = [];
    for (var i = 0; i < n; i++) {
      var node = i === 0 ? first : document.getElementById("df-a-" + h + "-" + i);
      var bin = atob(node.textContent);
      var u = new Uint8Array(bin.length);
      for (var j = 0; j < bin.length; j++) u[j] = bin.charCodeAt(j);
      out.push(u);
    }
    return { mime: first.getAttribute("data-mime") || "application/octet-stream", parts: out };
  }
  function url(h) {
    if (DF.role === "preview") {
      try { return window.parent.__deckfold.assets.url(h); } catch (e) { /* fall back to local decoding */ }
    }
    if (urls[h]) return urls[h];
    var p = parts(h);
    if (!p) { console.warn("deckfold: Asset fehlt", h); return "data:,"; }
    urls[h] = URL.createObjectURL(new Blob(p.parts, { type: p.mime }));
    return urls[h];
  }
  return { url: url, has: function (h) { return !!document.getElementById("df-a-" + h + "-0") || !!urls[h]; } };
})();

function fillPlaceholders(s) { return String(s).replace(PH, function (_, h) { return DF.assets.url(h); }); }
DF.fill = fillPlaceholders;

// Snapshot for the srcdoc previews: the document before activation, without asset blocks.
function snapshot() {
  var root = document.documentElement.cloneNode(true);
  var drop = root.querySelectorAll('script[type="application/octet-stream"][id^="df-a-"]');
  for (var i = 0; i < drop.length; i++) drop[i].parentNode.removeChild(drop[i]);
  var ui = root.querySelectorAll("[data-df-ui]");
  for (var k = 0; k < ui.length; k++) ui[k].parentNode.removeChild(ui[k]);
  root.classList.remove("df-loading", "df-presenter-mode");
  return "<!DOCTYPE html>" + root.outerHTML;
}

function activateCSS() {
  var nodes = [].slice.call(document.querySelectorAll('script[type="text/df-css"]'));
  var waits = [];
  nodes.forEach(function (n) {
    var css = fillPlaceholders(n.textContent), media = n.getAttribute("data-df-media"), repl;
    if (DF.cfg.cssMode === "link") {
      repl = document.createElement("link");
      repl.rel = "stylesheet";
      repl.href = URL.createObjectURL(new Blob([css], { type: "text/css" }));
      waits.push(new Promise(function (res) { repl.onload = repl.onerror = res; }));
    } else {
      repl = document.createElement("style");
      repl.textContent = css;
    }
    if (media) repl.setAttribute("media", media);
    n.parentNode.replaceChild(repl, n);
  });
  return Promise.all(waits);
}

var ATTRS = ["src", "srcset", "href", "xlink:href", "poster", "data", "style", "background"];
function restoreAttrs(quiet) {
  var all = document.querySelectorAll("*");
  for (var i = 0; i < all.length; i++) {
    var e = all[i];
    if (e.tagName === "SCRIPT" && e.type === "text/df-deferred") continue;
    if (!e.attributes.length) continue;
    for (var j = 0; j < ATTRS.length; j++) {
      var a = "data-df-" + ATTRS[j];
      if (!e.hasAttribute(a)) continue;
      var v = fillPlaceholders(e.getAttribute(a));
      e.removeAttribute(a);
      if (ATTRS[j] === "xlink:href") e.setAttributeNS("http://www.w3.org/1999/xlink", "xlink:href", v);
      else e.setAttribute(ATTRS[j], v);
    }
    if (quiet && (e.tagName === "VIDEO" || e.tagName === "AUDIO")) quietMedia(e);
  }
}
function quietMedia(m) {
  if (m.hasAttribute("autoplay")) { m.setAttribute("data-df-autoplay", ""); m.removeAttribute("autoplay"); }
  m.muted = true;
  try { m.pause(); } catch (e) { }
}

// ---------- shim for runtime loads ----------
function lookup(u) {
  if (!u || typeof u !== "string" || /^(blob:|data:|deckfold:|about:|javascript:)/i.test(u)) return null;
  var shim = (DF.meta && DF.meta.shim) || {};
  var abs, base;
  try { abs = new URL(u, document.baseURI); base = new URL(".", document.baseURI).href; } catch (e) { return null; }
  var href = abs.href.split("#")[0].split("?")[0];
  if (href.indexOf(base) !== 0) return null;
  var rel = href.slice(base.length);
  try { rel = decodeURIComponent(rel); } catch (e) { }
  var h = shim[rel];
  return h ? DF.assets.url(h) : null;
}
DF.lookup = lookup;

function wrapSetter(proto, prop) {
  var d = Object.getOwnPropertyDescriptor(proto, prop);
  if (!d || !d.set) return;
  Object.defineProperty(proto, prop, {
    configurable: true, enumerable: d.enumerable, get: d.get,
    set: function (v) { var r = lookup(v); return d.set.call(this, r || v); }
  });
}

var shimInstalled = false;
function installShim() {
  if (shimInstalled) return;
  shimInstalled = true;
  if (DF.role === "preview") {
    // In a srcdoc iframe, location is "about:srcdoc"; players (e.g. pdf.js) resolve relative paths
    // against window.location. That base is redirected to the document base URL (the output file).
    var OrigURL = window.URL;
    var DFURL = function (u, b) {
      if (b !== undefined && String(b).indexOf("about:srcdoc") === 0) b = document.baseURI;
      return b === undefined ? new OrigURL(u) : new OrigURL(u, b);
    };
    DFURL.prototype = OrigURL.prototype;
    Object.getOwnPropertyNames(OrigURL).forEach(function (k) {
      if (["prototype", "length", "name"].indexOf(k) < 0) { try { DFURL[k] = OrigURL[k]; } catch (e) { } }
    });
    window.URL = DFURL;
  }
  var origSetAttr = Element.prototype.setAttribute;
  Element.prototype.setAttribute = function (name, value) {
    var n = String(name).toLowerCase();
    if ((n === "src" || n === "href") && typeof value === "string") { var r = lookup(value); if (r) value = r; }
    return origSetAttr.call(this, name, value);
  };
  [[HTMLScriptElement, "src"], [HTMLImageElement, "src"], [HTMLMediaElement, "src"], [HTMLSourceElement, "src"],
   [HTMLLinkElement, "href"], [HTMLEmbedElement, "src"], [HTMLIFrameElement, "src"]].forEach(function (p) {
    if (p[0]) wrapSetter(p[0].prototype, p[1]);
  });
  var origOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url) {
    var args = [].slice.call(arguments), r = lookup(String(url));
    if (r) args[1] = r;
    return origOpen.apply(this, args);
  };
  if (window.fetch) {
    var origFetch = window.fetch;
    window.fetch = function (input, init) {
      var u = typeof input === "string" ? input : (input && input.url);
      var r = lookup(u);
      return origFetch.call(this, r || input, init);
    };
  }
  if (window.Worker) {
    var OrigWorker = window.Worker;
    var W = function (url, opts) { var r = lookup(String(url)); return new OrigWorker(r || url, opts); };
    W.prototype = OrigWorker.prototype;
    window.Worker = W;
  }
  // Styles inserted at runtime (e.g. webpack style-loader in the Keynote player): rewrite url(...).
  var fixStyle = function (s) {
    var txt = s.textContent;
    if (!txt || txt.indexOf("url(") < 0) return;
    var out = txt.replace(/url\(\s*(['"]?)([^'")]+)\1\s*\)/g, function (m, q, u) { var r = lookup(u); return r ? "url(" + q + r + q + ")" : m; });
    if (out !== txt) s.textContent = out;
  };
  new MutationObserver(function (recs) {
    recs.forEach(function (r) {
      for (var i = 0; i < r.addedNodes.length; i++) {
        var n = r.addedNodes[i];
        if (n.nodeName === "STYLE") fixStyle(n);
        else if (n.nodeType === 3 && r.target.nodeName === "STYLE") fixStyle(r.target);
      }
    });
  }).observe(document.documentElement, { childList: true, subtree: true });
}

// ---------- deferred source scripts ----------
function runDeferred() {
  var list = [].slice.call(document.querySelectorAll('script[type="text/df-deferred"]'));
  var captured = [];
  var origAdd = EventTarget.prototype.addEventListener;
  // Collect listeners for DOMContentLoaded/load that already fired, and call them once afterwards.
  EventTarget.prototype.addEventListener = function (type, fn, opt) {
    if (fn && (((this === document || this === window) && type === "DOMContentLoaded") ||
        (this === window && type === "load" && document.readyState === "complete"))) captured.push([this, type, fn]);
    return origAdd.call(this, type, fn, opt);
  };
  var onloadBefore = window.onload;
  var i = 0;
  return new Promise(function (done) {
    function step() {
      if (i >= list.length) return finish();
      var old = list[i++], s = document.createElement("script");
      for (var a = 0; a < old.attributes.length; a++) {
        var at = old.attributes[a];
        if (at.name === "type" || at.name === "data-df-type" || at.name === "data-df-src") continue;
        s.setAttribute(at.name, at.value);
      }
      var typ = old.getAttribute("data-df-type");
      if (typ) s.setAttribute("type", typ);
      var src = old.getAttribute("data-df-src");
      if (src) {
        s.async = false;
        s.onload = s.onerror = function () { step(); };
        s.src = fillPlaceholders(src);
        old.parentNode.replaceChild(s, old);
      } else {
        s.textContent = old.textContent;
        old.parentNode.replaceChild(s, old);
        step();
      }
    }
    function finish() {
      EventTarget.prototype.addEventListener = origAdd;
      captured.forEach(function (c) {
        try { var ev = new Event(c[1]); typeof c[2] === "function" ? c[2].call(c[0], ev) : c[2].handleEvent(ev); }
        catch (e) { console.error(e); }
      });
      if (window.onload && window.onload !== onloadBefore && document.readyState === "complete") {
        try { window.onload(new Event("load")); } catch (e) { console.error(e); }
      }
      done();
    }
    step();
  });
}

// activate: styles, attributes, fonts; with runScripts also the shim and the source scripts.
DF.activate = function (opts) {
  DF.meta = readJSON("df-meta") || { deck: { slides: [] }, shim: {} };
  return activateCSS().then(function () {
    restoreAttrs(!opts.runScripts);
    var fonts = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
    return Promise.race([fonts, new Promise(function (r) { setTimeout(r, 3000); })]);
  }).then(function () {
    if (!opts.runScripts) return;
    installShim();
    return runDeferred();
  });
};
DF.snapshot = snapshot;
DF.quietMedia = quietMedia;

// --- 20-model.js ---
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

// --- 30-keys.js ---
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

// --- 40-audience.js ---
// Audience window: owns the state and drives the source player; auto mode, black/white screen,
// full-screen fallback, wake lock, mouse cursor and the heartbeat to the presenter view.

DF.startAudience = function () {
  var opener = null;
  try { opener = DF.solo ? null : window.opener; } catch (e) { opener = null; }
  var send = DF.channel(function () { return opener; }, function () { return DF.token; }, onMsg);
  var generic = DF.format() === "generic";
  var adapter = DF.adapter();
  var st = { slide: 0, step: 0 };
  var driving = Promise.resolve();
  var blank = null;
  var auto = { on: false, due: 0, timer: 0, waiting: false };
  var cfg = DF.settings();
  document.documentElement.classList.add("df-audience");

  // ---------- Overlays ----------
  var blankEl = DF.el("div", { "data-df-ui": "", class: "df-blank" });
  var fsEl = DF.el("div", { "data-df-ui": "", class: "df-fs-overlay" }, [DF.el("button", { type: "button", text: DF.t("clickFs") })]);
  var endEl = DF.el("div", { "data-df-ui": "", class: "df-endslide" }, [DF.el("span", { text: DF.t("endOfShow") })]);
  document.body.appendChild(blankEl);
  document.body.appendChild(endEl);
  document.body.appendChild(fsEl);
  function setBlank(mode) {
    blank = blank === mode ? null : mode;
    blankEl.className = "df-blank" + (blank ? " df-on df-" + blank : "");
    report();
  }

  // ---------- full screen ----------
  function needFs() {
    return !document.fullscreenElement && !(window.innerHeight >= screen.height - 2 && window.innerWidth >= screen.width - 2);
  }
  function updateFsOverlay() { fsEl.classList.toggle("df-on", needFs() && fsWanted); }
  var fsWanted = true;
  fsEl.addEventListener("click", function () { DF.requestFs().then(updateFsOverlay, updateFsOverlay); });
  document.addEventListener("fullscreenchange", updateFsOverlay);
  window.addEventListener("resize", updateFsOverlay);
  DF.requestFs().then(updateFsOverlay, function () { setTimeout(updateFsOverlay, 300); });

  // ---------- live embeds without network (E9): neutral placeholder ----------
  function liveFrames() {
    return [].slice.call(document.querySelectorAll("iframe[src]")).filter(function (f) { return /^https?:/i.test(f.getAttribute("src") || "") && !f.hasAttribute("data-df-preview"); });
  }
  function markOffline() {
    var off = navigator.onLine === false;
    liveFrames().forEach(function (f) {
      var ph = f.__dfPlaceholder;
      if (off && !ph) {
        ph = DF.el("div", { "data-df-ui": "", class: "df-offline" }, [DF.el("span", { text: DF.t("offline") })]);
        f.__dfPlaceholder = ph;
        f.style.visibility = "hidden";
        if (f.parentNode) {
          if (getComputedStyle(f.parentNode).position === "static") f.parentNode.style.position = "relative";
          ph.style.left = f.offsetLeft + "px"; ph.style.top = f.offsetTop + "px";
          ph.style.width = f.offsetWidth + "px"; ph.style.height = f.offsetHeight + "px";
          f.parentNode.insertBefore(ph, f.nextSibling);
        }
      } else if (!off && ph) {
        f.style.visibility = "";
        if (ph.parentNode) ph.parentNode.removeChild(ph);
        f.__dfPlaceholder = null;
      }
    });
  }
  window.addEventListener("offline", markOffline);
  window.addEventListener("online", markOffline);
  setTimeout(markOffline, 500);

  // ---------- wake lock and mouse cursor ----------
  var lock = null;
  function wake() {
    if (!navigator.wakeLock || document.visibilityState !== "visible") return;
    navigator.wakeLock.request("screen").then(function (l) { lock = l; }, function () { });
  }
  document.addEventListener("visibilitychange", function () { if (document.visibilityState === "visible") wake(); });
  wake();
  var cursorTimer = 0;
  function showCursor() {
    document.documentElement.classList.remove("df-nocursor");
    clearTimeout(cursorTimer);
    cursorTimer = setTimeout(function () { document.documentElement.classList.add("df-nocursor"); }, 2000);
  }
  window.addEventListener("mousemove", showCursor, true);
  showCursor();

  // ---------- state ----------
  function report() {
    send({ type: "state", state: st, blank: blank, auto: { on: auto.on, due: auto.due, waiting: auto.waiting }, ended: !!st.ended });
    DF.ss.set("df-audience-state", { state: st, auto: auto.on });
  }
  function go(target, opts) {
    if (generic) return;
    if (!target) {
      // end reached
      if (auto.on && cfg.auto.loop) return go(DF.model.first());
      st = { slide: st.slide, step: st.step, ended: true };
      endEl.classList.add("df-on");
      stopAuto();
      report();
      return;
    }
    endEl.classList.remove("df-on");
    var from = st.ended ? null : st;
    st = { slide: target.slide, step: target.step };
    driving = driving.then(function () { return (opts && opts.jump) ? adapter.jump(st, from) : DF.drive(from, st); });
    report();
    scheduleAuto();
  }
  function act(a, arg) {
    switch (a) {
      case "next": return go(st.ended ? null : DF.model.next(st));
      case "prev": return go(st.ended ? st : DF.model.prev(st));
      case "first": return go(DF.model.first());
      case "last": return go(DF.model.last());
      case "goto": return go(DF.model.goto(arg));
      case "state": return go(arg, { jump: true });
      case "black": return setBlank("black");
      case "white": return setBlank("white");
      case "auto": return auto.on ? stopAuto(true) : startAuto();
      case "fullscreen": fsWanted = true; return DF.toggleFs();
      case "end":
        if (document.fullscreenElement) return;
        send({ type: "ended" });
        if (DF.solo) { DF.setSolo(null); DF.ss.del("df-audience-state"); location.reload(); } else window.close();
        return;
      case "timerReset": return send({ type: "action", action: "timerReset" });
    }
  }

  // ---------- auto mode (E7): the timer runs here, in the visible window ----------
  function sourceDelay() {
    var sl = DF.slides()[st.slide];
    if (sl && sl.delay && sl.delay[st.step] != null) return sl.delay[st.step] * 1000;
    return null;
  }
  function playingMedia() {
    var root = adapter.mediaRoot(st.slide);
    if (!root) return [];
    return [].slice.call(root.querySelectorAll("video,audio")).filter(function (m) { return !m.paused && !m.ended && isFinite(m.duration); });
  }
  function scheduleAuto() {
    clearTimeout(auto.timer);
    auto.waiting = false;
    if (!auto.on || st.ended) { auto.due = 0; return; }
    var d = sourceDelay();
    if (d == null) d = (cfg.auto.intervalSeconds || 8) * 1000;
    auto.due = Date.now() + d;
    tick();
  }
  function tick() {
    clearTimeout(auto.timer);
    if (!auto.on) return;
    var left = auto.due - Date.now();
    if (left > 0) { auto.timer = setTimeout(tick, Math.min(left, 250)); return; }
    var media = playingMedia();
    if (media.length) {
      // wait for the longest playing medium to end
      var longest = media.reduce(function (a, b) { return (b.duration - b.currentTime) > (a.duration - a.currentTime) ? b : a; });
      auto.waiting = true; report();
      var onEnd = function () { longest.removeEventListener("ended", onEnd); if (auto.on) act("next"); };
      longest.addEventListener("ended", onEnd);
      return;
    }
    act("next");
  }
  function startAuto() { auto.on = true; if (st.ended) go(DF.model.first()); else scheduleAuto(); report(); }
  function stopAuto(rep) { auto.on = false; clearTimeout(auto.timer); auto.due = 0; auto.waiting = false; if (rep) report(); }

  // ---------- keys ----------
  var forwardKey = function (e) { /* generic: the source gets the key unchanged */ };
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    act(a, e && e.slide);
    return true; // Escape in full screen too: the browser leaves full screen itself, the player never gets the key
  }, generic ? { passthrough: true, forward: forwardKey } : { swallowAll: true });

  // ---------- messages from the presenter view ----------
  function onMsg(d) {
    if (d.type === "init") { if (!DF.audienceInit(d.state)) act("state", d.state); if (d.settings) cfg = DF.merge(cfg, d.settings); return; }
    if (d.type === "cmd") act(d.cmd, d.arg);
    else if (d.type === "settings") { cfg = DF.merge(cfg, d.settings || {}); if (auto.on) scheduleAuto(); }
    else if (d.type === "key" && generic) {
      DF.synthKey(d.key, d.keyCode, { code: d.code, shift: d.shift });
    } else if (d.type === "end") { window.close(); }
  }
  setInterval(function () { send({ type: "hb", state: st, blank: blank, auto: { on: auto.on, due: auto.due, waiting: auto.waiting }, ended: !!st.ended }); }, 1000);
  window.addEventListener("pagehide", function () { send({ type: "ended", reload: true }); });

  // ---------- start ----------
  // With a presenter view: send "hello" first, then take over its state (it owns the session).
  // Without an answer within 1.5 s (or in solo mode) the stored state or the initial state applies.
  var prev = DF.ss.get("df-audience-state");
  var started = false, presenterState = null;
  function begin() {
    if (started) return;
    started = true;
    if (generic) return;
    var init = presenterState || (prev && prev.state) || DF.model.first();
    st = { slide: 0, step: 0 };
    go(init, { jump: true });
    if ((prev && prev.auto) || (!prev && cfg.auto.enabled)) startAuto();
  }
  DF.audienceInit = function (state) { if (!started) { presenterState = state; begin(); return true; } return false; };
  adapter.ready().then(function () {
    if (!opener) return begin();
    send({ type: "hello" });
    setTimeout(begin, 1500);
  });
  DF.audience = { act: act, state: function () { return st; } };
};

// --- 45-dialogs.js ---
// Dialogs: settings (E4) and screen choice (E3).

function dialog(title, body, buttons) {
  var box = DF.el("div", { "data-df-ui": "", class: "df-dialog-backdrop" });
  var dlg = DF.el("div", { class: "df-dialog", role: "dialog", "aria-modal": "true", "aria-label": title }, [
    DF.el("h2", { text: title }), body,
    DF.el("div", { class: "df-dialog-buttons" }, buttons)
  ]);
  box.appendChild(dlg);
  document.body.appendChild(box);
  var first = dlg.querySelector("input,select,button");
  if (first) first.focus();
  return { close: function () { if (box.parentNode) box.parentNode.removeChild(box); }, root: dlg };
}

DF.openSettings = function (onSave) {
  var s = DF.settings();
  var generic = DF.format() === "generic";
  var checks = DF.el("fieldset", { class: "df-checks" }, [DF.el("legend", { text: DF.t("elements") })]);
  ["current", "next", "notes", "time", "overview"].forEach(function (k) {
    var id = "df-el-" + k;
    checks.appendChild(DF.el("label", { for: id }, [
      DF.el("input", { type: "checkbox", id: id, "data-k": k, checked: s.presenter.elements.indexOf(k) >= 0 }), " " + DF.t(k)
    ]));
  });
  function num(id, label, val, min, max, step) {
    return DF.el("label", { class: "df-row", for: id }, [DF.el("span", { text: label }),
      DF.el("input", { type: "number", id: id, value: String(val), min: String(min), max: String(max), step: String(step || 1) })]);
  }
  function chk(id, label, val) {
    return DF.el("label", { class: "df-row", for: id }, [DF.el("span", { text: label }), DF.el("input", { type: "checkbox", id: id, checked: !!val })]);
  }
  var body = DF.el("div", { class: "df-settings" }, generic ? [DF.el("p", { text: DF.t("generic") })] : [
    checks,
    num("df-notes-size", DF.t("notesSize"), s.presenter.notesFontSize, 8, 96),
    num("df-target", DF.t("target"), s.presenter.targetMinutes, 0, 600),
    num("df-interval", DF.t("autoInterval"), s.auto.intervalSeconds, 1, 3600, 0.5),
    chk("df-autostart", DF.t("autoStart"), s.auto.enabled),
    chk("df-loop", DF.t("autoLoop"), s.auto.loop),
    chk("df-remember", DF.t("remember") + " (" + DF.t("screens") + ")", s.screens.rememberChoice)
  ]);
  var d;
  function clamp(v, lo, hi, dflt) { v = parseFloat(v); return isFinite(v) ? Math.max(lo, Math.min(hi, v)) : dflt; }
  var save = DF.el("button", { type: "button", class: "df-primary", text: DF.t("save"), onclick: function () {
    if (!generic) {
      var r = d.root;
      s.presenter.elements = [].slice.call(r.querySelectorAll(".df-checks input")).filter(function (i) { return i.checked; }).map(function (i) { return i.getAttribute("data-k"); });
      s.presenter.notesFontSize = Math.round(clamp(r.querySelector("#df-notes-size").value, 8, 96, 22));
      s.presenter.targetMinutes = Math.round(clamp(r.querySelector("#df-target").value, 0, 600, 0));
      s.auto.intervalSeconds = clamp(r.querySelector("#df-interval").value, 1, 3600, 8);
      s.auto.enabled = r.querySelector("#df-autostart").checked;
      s.auto.loop = r.querySelector("#df-loop").checked;
      s.screens.rememberChoice = r.querySelector("#df-remember").checked;
      if (!s.screens.rememberChoice) DF.ls.del("deckfold:screens");
    }
    DF.saveSettings(s);
    d.close();
    if (onSave) onSave(DF.settings());
  } });
  var reset = DF.el("button", { type: "button", text: DF.t("reset"), onclick: function () {
    DF.saveSettings(DF.clone(DF.cfg.defaults));
    d.close();
    if (onSave) onSave(DF.settings());
  } });
  var cancel = DF.el("button", { type: "button", text: DF.t("cancel"), onclick: function () { d.close(); } });
  d = dialog(DF.t("settings"), body, [reset, cancel, save]);
  return d;
};

function screenKey(s) { return { label: s.label || "", w: s.width, h: s.height, l: s.left, t: s.top }; }
function matchScreen(list, k) {
  if (!k) return -1;
  for (var i = 0; i < list.length; i++) if (list[i].label === k.label && list[i].width === k.w && list[i].height === k.h) return i;
  for (var j = 0; j < list.length; j++) if (list[j].width === k.w && list[j].height === k.h && list[j].left === k.l) return j;
  return -1;
}

// chooseScreens(onStart): Chromium with the Window Management API shows a dialog; otherwise onStart(null).
// onStart(choice) must run synchronously inside the click (user gesture): choice = {audience, presenter, presenterFs, granted}.
DF.chooseScreens = function (onStart) {
  if (!("getScreenDetails" in window)) { onStart(null); return; }
  window.getScreenDetails().then(function (det) {
    DF.screenDetails = det;
    var list = det.screens;
    var saved = DF.settings().screens.rememberChoice ? DF.ls.get("deckfold:screens") : null;
    var ai = matchScreen(list, saved && saved.aud), pi = matchScreen(list, saved && saved.pre);
    if (ai < 0) ai = list.length > 1 ? list.indexOf(list.filter(function (s) { return s !== det.currentScreen; })[0]) : 0;
    if (pi < 0) pi = Math.max(0, list.indexOf(det.currentScreen));
    function sel(id, idx) {
      var s = DF.el("select", { id: id });
      list.forEach(function (sc, i) {
        var label = (sc.label || ("Screen " + (i + 1))) + " \u2013 " + sc.width + "\u00d7" + sc.height + (sc.isPrimary ? " \u2605" : "");
        s.appendChild(DF.el("option", { value: String(i), text: label, selected: i === idx }));
      });
      return s;
    }
    var fs = DF.el("input", { type: "checkbox", id: "df-prefs", checked: saved ? !!saved.fs : true });
    var hint = DF.el("p", { class: "df-hint", text: DF.t("presenterStays") });
    var body = DF.el("div", { class: "df-settings" }, [
      DF.el("label", { class: "df-row", for: "df-aud" }, [DF.el("span", { text: DF.t("audienceScreen") }), sel("df-aud", ai)]),
      DF.el("label", { class: "df-row", for: "df-pre" }, [DF.el("span", { text: DF.t("presenterScreen") }), sel("df-pre", pi)]),
      DF.el("label", { class: "df-row", for: "df-prefs" }, [DF.el("span", { text: DF.t("presenterFs") }), fs]),
      hint
    ]);
    var d;
    function upd() { hint.style.display = fs.checked ? "none" : ""; }
    fs.addEventListener("change", upd); upd();
    var startBtn = DF.el("button", { type: "button", class: "df-primary", text: DF.t("start"), onclick: function () {
      var a = list[parseInt(d.root.querySelector("#df-aud").value, 10)], p = list[parseInt(d.root.querySelector("#df-pre").value, 10)];
      if (DF.settings().screens.rememberChoice) DF.ls.set("deckfold:screens", { aud: screenKey(a), pre: screenKey(p), fs: fs.checked });
      d.close();
      onStart({ audience: a, presenter: p, presenterFs: fs.checked, granted: true });
    } });
    d = dialog(DF.t("screens"), body, [DF.el("button", { type: "button", text: DF.t("cancel"), onclick: function () { d.close(); } }), startBtn]);
  }, function () {
    // permission denied: default position
    var d = dialog(DF.t("screens"), DF.el("p", { text: DF.t("screensDenied") }), [
      DF.el("button", { type: "button", class: "df-primary", text: DF.t("start"), onclick: function () { d.close(); onStart(null); } })
    ]);
  });
};

// --- 50-presenter.js ---
// Start screen and presenter view (E5): current slide and preview as muted srcdoc iframes,
// notes, time, slide overview; window management with heartbeat and recovery.

DF.startPresenter = function (snapshotHTML) {
  var generic = DF.format() === "generic";
  var slides = DF.slides();
  var meta = DF.meta.deck || {};
  var session = DF.ss.get("df-session");
  var popup = null, token = session ? session.token : "";
  var st = session && session.state ? session.state : DF.model.first();
  var remote = { blank: null, auto: { on: false, due: 0 }, ended: false };
  var lastBeat = 0, active = !!(session && session.active);
  var cfg = DF.settings();
  var timer = (session && session.timer) || { start: 0, accum: 0, paused: false };
  var send = DF.channel(function () { return popup; }, function () { return token; }, onMsg);

  function saveSession() { DF.ss.set("df-session", { token: token, state: st, active: active, timer: timer }); }

  // ---------- start screen ----------
  var start = DF.el("div", { "data-df-ui": "", class: "df-start" }, [
    DF.el("div", { class: "df-start-box" }, [
      DF.el("h1", { text: meta.title || document.title || "deckfold" }),
      DF.el("p", { class: "df-sub", text: generic ? DF.t("generic") : (slides.length + " " + DF.t("slides")) }),
      DF.el("div", { class: "df-start-buttons" }, [
        DF.el("button", { type: "button", class: "df-primary", text: DF.t("present"), onclick: present }),
        DF.el("button", { type: "button", text: DF.t("audienceOnly"), onclick: soloAudience }),
        DF.el("button", { type: "button", text: DF.t("settings"), onclick: function () { DF.openSettings(applySettings); } })
      ]),
      DF.el("p", { class: "df-keys", text: DF.t("keysHelp") }),
      DF.el("p", { class: "df-ver", text: "deckfold " + (DF.version || "") })
    ])
  ]);
  document.body.appendChild(start);

  function soloAudience() {
    DF.setSolo(DF.randomToken());
    DF.ss.del("df-audience-state");
    location.reload();
  }

  // ---------- present ----------
  function present() {
    DF.chooseScreens(function (choice) { launch(choice); });
  }
  function launch(choice) {
    if (!token) token = DF.randomToken();
    var url = location.href.split("#")[0];
    var feat = "popup";
    if (choice && choice.presenterFs) DF.requestFs(document.documentElement, choice.presenter).catch(function () { });
    if (choice && choice.audience) {
      var a = choice.audience;
      feat += ",left=" + a.availLeft + ",top=" + a.availTop + ",width=" + a.availWidth + ",height=" + a.availHeight;
      if (choice.granted) feat += ",fullscreen";
    } else {
      feat += ",width=1280,height=720";
    }
    DF.ss.del("df-audience-state");
    popup = window.open(url, "df-audience:" + token, feat);
    active = true;
    if (!timer.start) timer = { start: Date.now(), accum: 0, paused: false };
    saveSession();
    enterPresenter();
    if (!popup) banner("popupBlocked", "openAudience", function () { reopen(); });
    else if (!choice && !("getScreenDetails" in window)) banner("screensHint");
    lastBeat = Date.now();
  }
  function reopen() {
    var url = location.href.split("#")[0];
    DF.ss.del("df-audience-state");
    popup = window.open(url, "df-audience:" + token, "popup,width=1280,height=720");
    lastBeat = Date.now();
    hideBanner();
    if (!popup) banner("popupBlocked", "openAudience", reopen);
  }

  // ---------- presenter view ----------
  var ui = null, frames = {}, notesEl, counterEl, clockEl, elapsedEl, remainEl, autoBtn, autoInfo, pauseBtn, bannerEl, overviewEl;
  function enterPresenter() {
    start.style.display = "none";
    document.documentElement.classList.add("df-presenter-mode");
    if (ui) return render();
    ui = DF.el("div", { "data-df-ui": "", class: "df-presenter" });
    bannerEl = DF.el("div", { class: "df-banner" });
    counterEl = DF.el("span", { class: "df-counter" });
    clockEl = DF.el("span", { class: "df-clock", title: DF.t("clock") });
    elapsedEl = DF.el("span", { class: "df-elapsed", title: DF.t("elapsed") });
    remainEl = DF.el("span", { class: "df-remain", title: DF.t("remaining") });
    pauseBtn = DF.el("button", { type: "button", onclick: togglePause });
    autoBtn = DF.el("button", { type: "button", onclick: function () { command("auto"); } });
    autoInfo = DF.el("span", { class: "df-autoinfo" });
    var bar = DF.el("header", { class: "df-bar" }, [
      DF.el("strong", { class: "df-title", text: meta.title || document.title || "" }), counterEl,
      DF.el("span", { class: "df-time-group", "data-k": "time" }, [clockEl, elapsedEl, remainEl, pauseBtn,
        DF.el("button", { type: "button", text: "\u21ba", title: DF.t("reset"), onclick: resetTimer })]),
      generic ? null : DF.el("span", { class: "df-auto-group" }, [autoBtn, autoInfo]),
      generic ? null : DF.el("button", { type: "button", "data-k": "overview", text: DF.t("overview"), onclick: toggleOverview }),
      DF.el("button", { type: "button", text: DF.t("settings"), onclick: function () { DF.openSettings(applySettings); } }),
      "getScreenDetails" in window ? DF.el("button", { type: "button", text: DF.t("screens"), onclick: function () { DF.chooseScreens(function (c) { relaunch(c); }); } }) : null,
      DF.el("button", { type: "button", class: "df-danger", text: DF.t("endShow"), onclick: endShow })
    ]);
    ui.appendChild(bar);
    ui.appendChild(bannerEl);
    if (generic) {
      ui.appendChild(DF.el("main", { class: "df-generic" }, [DF.el("p", { text: DF.t("noPresenter") })]));
    } else {
      var cur = pane("current", DF.t("current"));
      var nxt = pane("next", DF.t("next"));
      notesEl = DF.el("div", { class: "df-notes-text" });
      var notes = DF.el("section", { class: "df-pane df-notes", "data-k": "notes" }, [DF.el("h3", { text: DF.t("notes") }), notesEl]);
      ui.appendChild(DF.el("main", { class: "df-grid" }, [cur, DF.el("div", { class: "df-side" }, [nxt, notes])]));
      frames.current = makeFrame(cur.querySelector(".df-stage"));
      frames.next = makeFrame(nxt.querySelector(".df-stage"));
      overviewEl = DF.el("div", { class: "df-overview" });
      ui.appendChild(overviewEl);
    }
    document.body.appendChild(ui);
    if (DF.screenDetails) DF.screenDetails.addEventListener("screenschange", function () { banner("screensChanged", "chooseAgain", function () { DF.chooseScreens(relaunch); }); });
    render();
    setInterval(tickClock, 500);
    setInterval(watch, 1000);
    // take the focus back if it still ends up in a preview
    var refocus = function () {
      var a = document.activeElement;
      if (a && a.tagName === "IFRAME" && a.hasAttribute("data-df-preview")) { a.blur(); document.body.focus && document.body.focus(); }
    };
    window.addEventListener("blur", function () { setTimeout(refocus, 0); });
    setInterval(refocus, 500);
  }
  function pane(k, title) {
    return DF.el("section", { class: "df-pane df-" + k, "data-k": k }, [DF.el("h3", { text: title }),
      DF.el("div", { class: "df-stage" }), DF.el("div", { class: "df-endlabel", text: DF.t("end") })]);
  }
  function makeFrame(host) {
    var f = DF.el("iframe", { "data-df-preview": "", tabindex: "-1", inert: true, title: "preview", "aria-hidden": "true" });
    f.srcdoc = snapshotHTML;
    var shield = DF.el("div", { class: "df-shield" });
    host.appendChild(f);
    host.appendChild(shield);
    var rec = { el: f, want: null, ready: false };
    f.addEventListener("load", function () {
      var w = f.contentWindow;
      var waitReady = function () {
        if (w.__deckfold && w.__deckfold.previewReady) {
          w.__deckfold.previewReady.then(function () { rec.ready = true; push(rec); });
        } else setTimeout(waitReady, 50);
      };
      waitReady();
    });
    return rec;
  }
  function push(rec) {
    if (!rec.ready || !rec.want) return;
    try { rec.el.contentWindow.__deckfold.setPreviewState(rec.want); } catch (e) { }
  }
  function render() {
    if (!ui) return;
    cfg = DF.settings();
    var els = cfg.presenter.elements;
    [].slice.call(ui.querySelectorAll("[data-k]")).forEach(function (n) { n.classList.toggle("df-hidden", els.indexOf(n.getAttribute("data-k")) < 0); });
    ui.classList.toggle("df-no-next", els.indexOf("next") < 0);
    if (generic) return;
    var n = slides.length;
    counterEl.textContent = (st.slide + 1) + " / " + n + (slides[st.slide] && slides[st.slide].steps > 1 ? "  \u00b7  " + DF.t("step") + " " + (st.step + 1) + "/" + slides[st.slide].steps : "");
    var hasNotes = slides.some(function (s) { return s.notes; });
    var notesPane = ui.querySelector(".df-notes");
    if (!hasNotes) notesPane.classList.add("df-hidden");
    var note = slides[st.slide] && slides[st.slide].notes;
    notesEl.textContent = note || DF.t("noNotes");
    notesEl.style.fontSize = cfg.presenter.notesFontSize + "px";
    frames.current.want = st; push(frames.current);
    var nx = DF.model.next(st);
    ui.querySelector(".df-next").classList.toggle("df-at-end", !nx);
    if (nx) { frames.next.want = nx; push(frames.next); }
    autoBtn.textContent = remote.auto.on ? DF.t("autoOn") : DF.t("autoOff");
    autoBtn.classList.toggle("df-active", !!remote.auto.on);
    if (overviewEl && overviewEl.classList.contains("df-on")) markOverview();
    saveSession();
  }
  function tickClock() {
    var d = new Date();
    clockEl.textContent = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
    var el = elapsed();
    elapsedEl.textContent = DF.fmtTime(el);
    pauseBtn.textContent = timer.paused ? "\u25b6" : "\u275a\u275a";
    pauseBtn.title = timer.paused ? DF.t("resume") : DF.t("pause");
    var target = cfg.presenter.targetMinutes * 60000;
    remainEl.style.display = target ? "" : "none";
    if (target) {
      var left = target - el;
      remainEl.textContent = DF.fmtTime(left);
      remainEl.className = "df-remain" + (left < 0 ? " df-over" : left < 120000 ? " df-warn" : "");
    }
    if (autoInfo) autoInfo.textContent = remote.auto.on ? (remote.auto.waiting ? "\u25b6 " + DF.t("media") : remote.auto.due ? Math.max(0, Math.ceil((remote.auto.due - Date.now()) / 1000)) + " s" : "") : "";
  }
  function elapsed() {
    if (!timer.start) return 0;
    return timer.accum + (timer.paused ? 0 : Date.now() - timer.start);
  }
  function togglePause() {
    if (timer.paused) { timer.start = Date.now(); timer.paused = false; }
    else { timer.accum = elapsed(); timer.paused = true; }
    saveSession(); tickClock();
  }
  function resetTimer() { timer = { start: Date.now(), accum: 0, paused: false }; saveSession(); tickClock(); }

  // ---------- overview ----------
  function toggleOverview() {
    if (!overviewEl) return;
    var on = !overviewEl.classList.contains("df-on");
    overviewEl.classList.toggle("df-on", on);
    if (on && !overviewEl.firstChild) buildOverview();
    if (on) markOverview();
  }
  function buildOverview() {
    var srcSlides = document.querySelectorAll(".deck-slide");
    var w = meta.width || 1920, h = meta.height || 1080;
    slides.forEach(function (s, i) {
      var tile = DF.el("button", { type: "button", class: "df-tile", "data-i": String(i), onclick: function () { command("goto", i); toggleOverview(); } });
      var box = DF.el("div", { class: "df-thumb" });
      box.style.aspectRatio = w + " / " + h;
      if (s.thumb) {
        box.appendChild(DF.el("img", { src: DF.assets.url(s.thumb), alt: "" }));
      } else if (srcSlides[i]) {
        var canvas = srcSlides[i].querySelector(".deck-scaler > *") || srcSlides[i];
        var c = canvas.cloneNode(true);
        [].slice.call(c.querySelectorAll("script,iframe,video,audio")).forEach(function (x) { x.parentNode.removeChild(x); });
        c.removeAttribute("id");
        var holder = DF.el("div", { class: "df-clone" });
        holder.style.width = w + "px"; holder.style.height = h + "px";
        holder.appendChild(c);
        box.appendChild(holder);
      }
      tile.appendChild(box);
      tile.appendChild(DF.el("span", { text: (i + 1) + (s.label ? " \u00b7 " + s.label : "") }));
      overviewEl.appendChild(tile);
    });
    scaleClones();
    window.addEventListener("resize", scaleClones);
  }
  function scaleClones() {
    var w = meta.width || 1920;
    [].slice.call(overviewEl.querySelectorAll(".df-clone")).forEach(function (c) {
      var bw = c.parentNode.clientWidth;
      c.style.transform = "scale(" + (bw / w) + ")";
    });
  }
  function markOverview() {
    [].slice.call(overviewEl.querySelectorAll(".df-tile")).forEach(function (t) { t.classList.toggle("df-cur", +t.getAttribute("data-i") === st.slide); });
  }

  // ---------- Banner ----------
  function banner(textKey, btnKey, fn) {
    bannerEl.innerHTML = "";
    bannerEl.appendChild(DF.el("span", { text: DF.t(textKey) }));
    if (btnKey) bannerEl.appendChild(DF.el("button", { type: "button", text: DF.t(btnKey), onclick: fn }));
    bannerEl.classList.add("df-on");
  }
  function hideBanner() { bannerEl.classList.remove("df-on"); }

  // ---------- commands ----------
  var pending = [], bootAt = Date.now();
  function command(cmd, arg) {
    if (popup && !popup.closed && send({ type: "cmd", cmd: cmd, arg: arg })) return;
    if (active && !popup && Date.now() - bootAt < 5000) { pending.push([cmd, arg]); return; } // wait for the reconnect
    // without an audience window: advance locally (rehearsal)
    if (generic) return;
    var nx = null;
    switch (cmd) {
      case "next": nx = DF.model.next(st); break;
      case "prev": nx = DF.model.prev(st); break;
      case "first": nx = DF.model.first(); break;
      case "last": nx = DF.model.last(); break;
      case "goto": nx = DF.model.goto(arg); break;
    }
    if (nx) { st = nx; render(); }
  }
  function onMsg(d, source) {
    if (!popup && source) {
      popup = source; // after a reload of the presenter view
      var q = pending; pending = [];
      setTimeout(function () { q.forEach(function (c) { command(c[0], c[1]); }); }, 0);
    }
    lastBeat = Date.now();
    hideBannerIfLost();
    if (d.type === "hello") {
      // new or reloaded audience window: send the session state and the auto settings
      send({ type: "init", state: generic ? null : st, settings: { auto: cfg.auto } });
      return;
    }
    if (d.type === "state" || d.type === "hb") {
      if (d.state && !generic) {
        var changed = !DF.model.same(d.state, st) || remote.auto.on !== (d.auto && d.auto.on);
        st = { slide: d.state.slide, step: d.state.step };
        remote = { blank: d.blank, auto: d.auto || { on: false }, ended: d.ended };
        if (changed || d.type === "state") render();
      }
      return;
    }
    if (d.type === "action" && d.action === "timerReset") resetTimer();
    if (d.type === "ended" && !d.reload) { popup = null; leave(); }
  }
  function hideBannerIfLost() { if (bannerEl && bannerEl.getAttribute("data-lost")) { bannerEl.removeAttribute("data-lost"); hideBanner(); } }
  function watch() {
    if (!active) return;
    var lost = (popup && popup.closed) || (!popup && Date.now() - lastBeat > 3000) || (popup && Date.now() - lastBeat > 4000);
    if (lost && bannerEl && !bannerEl.getAttribute("data-lost")) {
      banner(popup ? "audienceLost" : "searching", "reopen", reopen);
      bannerEl.setAttribute("data-lost", "1");
    }
  }
  function relaunch(choice) {
    if (popup && !popup.closed) { try { popup.close(); } catch (e) { } }
    popup = null;
    launch(choice);
  }
  function endShow() {
    send({ type: "end" });
    if (popup && !popup.closed) { try { popup.close(); } catch (e) { } }
    leave();
  }
  function leave() {
    active = false; popup = null;
    DF.ss.del("df-session"); DF.ss.del("df-audience-state");
    token = ""; timer = { start: 0, accum: 0, paused: false };
    if (document.fullscreenElement) document.exitFullscreen().catch(function () { });
    document.documentElement.classList.remove("df-presenter-mode");
    if (ui) ui.style.display = "none";
    start.style.display = "";
    location.reload();
  }
  function applySettings(s) {
    cfg = s;
    send({ type: "settings", settings: { auto: s.auto } });
    render();
  }

  // ---------- keys in the presenter view ----------
  DF.installKeys(function (a, e) {
    if (!active) {
      if (a === "fullscreen") { DF.toggleFs(); return true; }
      return false;
    }
    switch (a) {
      case "digits": return true;
      case "timerReset": resetTimer(); return true;
      case "fullscreen": DF.toggleFs(); return true;
      case "end": if (!document.fullscreenElement) endShow(); return true;
      case "goto": command("goto", e.slide); return true;
      default: command(a); return true;
    }
  }, generic ? { passthrough: true, forward: function (e) {
    if (active && popup && !popup.closed) {
      send({ type: "key", key: e.key, code: e.code, keyCode: e.keyCode, shift: e.shiftKey });
      e.preventDefault();
    }
  } } : null);

  // recovery after a reload of the presenter view
  if (active) {
    enterPresenter();
    lastBeat = Date.now();
  }
};

// --- 90-main.js ---
// Boot per role.
document.documentElement.classList.add("df-loading");

function bootPreview() {
  // Preview: muted, without its own keys; driven by the presenter view.
  // The preview must never take the keyboard focus (the clicker would stop working).
  HTMLElement.prototype.focus = function () { };
  window.focus = function () { };
  var origPlay = HTMLMediaElement.prototype.play;
  HTMLMediaElement.prototype.play = function () { this.muted = true; this.volume = 0; return origPlay.apply(this, arguments); };
  var cur = { slide: 0, step: 0 }, queue = Promise.resolve(), ready;
  DF.previewReady = new Promise(function (res) { ready = res; });
  DF.setPreviewState = function (s) {
    queue = queue.then(function () {
      if (DF.model.same(cur, s)) return;
      var from = cur;
      cur = { slide: s.slide, step: s.step };
      return DF.drive(from, cur).then(function () { return DF.wait(30); });
    });
  };
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    document.documentElement.classList.add("df-preview");
    return DF.adapter().ready();
  }).then(function () { ready(); });
}

function bootAudience() {
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startAudience();
  });
}

function bootStart() {
  var snap = DF.snapshot();
  DF.activate({ runScripts: false }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startPresenter(snap);
  });
}

function boot() {
  try {
    if (DF.role === "preview") bootPreview();
    else if (DF.role === "audience") bootAudience();
    else bootStart();
  } catch (e) {
    console.error("deckfold:", e);
    document.documentElement.classList.remove("df-loading");
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
else boot();

})();
