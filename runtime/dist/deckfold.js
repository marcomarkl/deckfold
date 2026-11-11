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

// Roles:
// "show": the window the file was opened in. It shows the start screen first and runs the source
//   player only after "Start"; it owns the state.
// "presenter": the presenter view, a popup the show window opens (window.name "df-presenter:<token>",
//   kept in sessionStorage too, because WebKit drops window.name when a file:// page reloads).
// "preview": srcdoc iframes in the start screen and the presenter view.
DF.role = (function () {
  try {
    if (window.frameElement && window.frameElement.hasAttribute("data-df-preview")) return "preview";
  } catch (e) { /* cross-origin */ }
  var n = window.name || "", saved = null;
  try { saved = sessionStorage.getItem("df-presenter"); } catch (e) { }
  if (n.indexOf("df-presenter:") === 0 || saved) return "presenter";
  return "show";
})();
// Token of the session; both windows use the same one. The presenter view has it from the name
// its window was opened with. The show window keeps its token across reloads, per deck, and reads
// it once the deck metadata is there (35-start.js).
DF.token = (function () {
  if (DF.role === "preview") return "";
  try {
    if (DF.role === "presenter") {
      var t = (window.name || "").indexOf("df-presenter:") === 0 ? window.name.split(":")[1] : sessionStorage.getItem("df-presenter");
      sessionStorage.setItem("df-presenter", t);
      return t;
    }
  } catch (e) { }
  return null;
})();

var LANG = String(navigator.language || "en").toLowerCase().indexOf("de") === 0 ? "de" : "en";
var TEXT = {
  de: {
    slides: "Folien", generic: "Unbekanntes Format: ohne Referentenansicht, \u00dcbersicht und Auto-Modus.",
    start: "Start", presenterView: "Referentenansicht",
    fullscreen: "Vollbild", window: "Fenster", auto: "Auto-Modus", autoInterval: "Intervall (s)", autoLoop: "Schleife",
    slide: "Folie", previous: "Zur\u00fcck", following: "Weiter",
    popupBlocked: "Der Browser hat das Fenster der Referentenansicht blockiert. Erlaube Pop-ups f\u00fcr diese Datei und schalte die Referentenansicht noch einmal ein.",
    startInShow: "Gestartet wird im Pr\u00e4sentationsfenster: Klick dort auf \u201eStart\u201c.", showLost: "Das Pr\u00e4sentationsfenster ist nicht erreichbar.",
    loading: "Wird geladen \u2026",
    clickFs: "Klicken f\u00fcr Vollbild",
    current: "Aktuell", audience: "Publikum", next: "Als N\u00e4chstes", notes: "Notizen", time: "Zeit", overview: "\u00dcbersicht",
    end: "Ende", endShow: "Beenden", noNotes: "Keine Notizen", elapsed: "Verstrichen", remaining: "Rest",
    clock: "Uhr", pause: "Pause", resume: "Weiter", autoOn: "Auto an", autoOff: "Auto aus", reset: "Zur\u00fccksetzen",
    target: "Ziel", smaller: "Notizen kleiner", larger: "Notizen gr\u00f6\u00dfer",
    endOfShow: "Ende der Pr\u00e4sentation", step: "Schritt",
    offline: "Live-Inhalt \u2013 ohne Netz nicht verf\u00fcgbar", media: "Medien laufen",
    keysStart: "Startbildschirm", keysSlides: "Folien", keysDisplay: "Anzeige", keysRun: "Ablauf",
    chooseSlide: "Folie w\u00e4hlen", firstSlide: "Erste Folie", lastSlide: "Letzte Folie", gotoSlide: "Folie n",
    black: "Schwarz", white: "Wei\u00df", timerReset: "Zeit zur\u00fccksetzen", click: "Klick",
    search: "Suchen", noResults: "Keine Treffer", close: "Schlie\u00dfen", shortcuts: "Tastaturk\u00fcrzel",
    keySpace: "Leertaste", keyPgDn: "Bild \u2193", keyPgUp: "Bild \u2191", keyHome: "Pos1", keyEnd: "Ende", keyBackspace: "R\u00fcckschritt", keyDigits: "1 2 3 \u2026",
  },
  en: {
    slides: "slides", generic: "Unknown format: no presenter view, overview or auto mode.",
    start: "Start", presenterView: "Presenter view",
    fullscreen: "Full screen", window: "Window", auto: "Auto mode", autoInterval: "Interval (s)", autoLoop: "Loop",
    slide: "Slide", previous: "Back", following: "Forward",
    popupBlocked: "The browser blocked the presenter view window. Allow pop-ups for this file and switch the presenter view on again.",
    startInShow: "The presentation starts in the presentation window: click \u201cStart\u201d there.", showLost: "The presentation window is not reachable.",
    loading: "Loading \u2026",
    clickFs: "Click for full screen",
    current: "Current", audience: "Audience", next: "Next", notes: "Notes", time: "Time", overview: "Overview",
    end: "End", endShow: "End show", noNotes: "No notes", elapsed: "Elapsed", remaining: "Left",
    clock: "Clock", pause: "Pause", resume: "Resume", autoOn: "Auto on", autoOff: "Auto off", reset: "Reset",
    target: "Target", smaller: "Smaller notes", larger: "Larger notes",
    endOfShow: "End of presentation", step: "Step",
    offline: "Live content \u2013 not available offline", media: "media playing",
    keysStart: "Start screen", keysSlides: "Slides", keysDisplay: "Display", keysRun: "Show",
    chooseSlide: "Choose slide", firstSlide: "First slide", lastSlide: "Last slide", gotoSlide: "Slide n",
    black: "Black", white: "White", timerReset: "Reset time", click: "Click",
    search: "Search", noResults: "No results", close: "Close", shortcuts: "Keyboard shortcuts",
    keySpace: "Space", keyPgDn: "Page Down", keyPgUp: "Page Up", keyHome: "Home", keyEnd: "End", keyBackspace: "Backspace", keyDigits: "1 2 3 \u2026",
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
// deckKey: the name under which this deck keeps a value of the running session. A tab keeps its
// sessionStorage when it opens another file, and every file:// page shares it, so a value that
// belongs to one deck (the token of the session, the slide chosen last) has the deck in its name.
DF.deckKey = function (name) { return name + ":" + ((DF.meta && DF.meta.deckHash) || "deck"); };

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

// Target duration typed as minutes:seconds. parseMinSec returns seconds (0 = no target), or null
// for input it cannot read: "20" is 20:00, "5:3" is 05:03, "1:75" is 02:15, ":30" is 00:30;
// at most 600:00.
DF.parseMinSec = function (v) {
  v = String(v == null ? "" : v).trim();
  if (v === "") return 0;
  var m = /^(\d*)(?::(\d*))?$/.exec(v);
  if (!m || (m[1] === "" && !m[2])) return null;
  return Math.min(parseInt(m[1] || "0", 10) * 60 + parseInt(m[2] || "0", 10), 36000);
};
DF.fmtMinSec = function (t) {
  var m = Math.floor(t / 60), s = t % 60;
  return (m < 10 ? "0" : "") + m + ":" + (s < 10 ? "0" : "") + s;
};


// --- 10-loader.js ---
// Loader: decode asset blocks, activate styles and attributes, shim runtime loads,
// start the deferred source scripts in their original order.
var PH = /deckfold:([0-9a-f]{20})/g;

// Asset store. url(h) returns a cached blob URL that is never released: it serves placeholders in
// attributes and CSS and media, image and link sources set through the shim, all of which may stay
// in use while the page is open. fresh(h) returns a new, uncached URL for one-shot use (scripts,
// XHR, fetch, worker code); the caller releases it with revoke(u) after use. Releasing right after
// open(), new Worker() or a settled fetch is safe because the URL parser binds the blob when the
// URL is parsed. In the preview role all three go to the parent window, so the same window creates
// and releases every URL.
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
  function parent() {
    if (DF.role !== "preview") return null;
    try { return window.parent.__deckfold.assets; } catch (e) { return null; } // fall back to local decoding
  }
  function create(h) {
    var p = parts(h);
    if (!p) { console.warn("deckfold: asset missing", h); return null; }
    return URL.createObjectURL(new Blob(p.parts, { type: p.mime }));
  }
  function url(h) {
    var pa = parent();
    if (pa) return pa.url(h);
    if (!urls[h]) urls[h] = create(h);
    return urls[h] || "data:,";
  }
  function fresh(h) {
    var pa = parent();
    if (pa) return pa.fresh(h);
    return create(h) || "data:,";
  }
  function revoke(u) {
    var pa = parent();
    if (pa) return pa.revoke(u);
    if (typeof u === "string" && u.indexOf("blob:") === 0) URL.revokeObjectURL(u);
  }
  return { url: url, fresh: fresh, revoke: revoke, has: function (h) { return !!document.getElementById("df-a-" + h + "-0") || !!urls[h]; } };
})();

function fillPlaceholders(s) { return String(s).replace(PH, function (_, h) { return DF.assets.url(h); }); }
DF.fill = fillPlaceholders;

// Snapshot for the srcdoc previews: the document before activation, without asset blocks and
// without the content model (a preview searches nothing).
// The blocks are taken out of the document while it is cloned and put back right away, so
// that a large file is not copied in memory once more.
function snapshot() {
  var blocks = [].slice.call(document.querySelectorAll('script[type="application/octet-stream"][id^="df-a-"], script#df-content'));
  var spots = blocks.map(function (b) { var at = { parent: b.parentNode, next: b.nextSibling }; b.parentNode.removeChild(b); return at; });
  var root;
  try { root = document.documentElement.cloneNode(true); }
  finally {
    for (var j = blocks.length - 1; j >= 0; j--) spots[j].parent.insertBefore(blocks[j], spots[j].next);
  }
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
function restoreAttrs(quiet, root) {
  var all = [].slice.call((root || document).querySelectorAll("*"));
  if (root) all.unshift(root);
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
// lookupHash maps a path requested by the source player to the hash of an embedded asset.
function lookupHash(u) {
  if (!u || typeof u !== "string" || /^(blob:|data:|deckfold:|about:|javascript:)/i.test(u)) return null;
  var shim = (DF.meta && DF.meta.shim) || {};
  var abs, base;
  try { abs = new URL(u, document.baseURI); base = new URL(".", document.baseURI).href; } catch (e) { return null; }
  var href = abs.href.split("#")[0].split("?")[0];
  if (href.indexOf(base) !== 0) return null;
  var rel = href.slice(base.length);
  try { rel = decodeURIComponent(rel); } catch (e) { }
  return shim[rel] || null;
}
function lookup(u) { var h = lookupHash(u); return h ? DF.assets.url(h) : null; }
DF.lookup = lookup;

// oneShotScript gives a script element a fresh URL and releases it once the script has loaded.
function oneShotScript(el, h) {
  var u = DF.assets.fresh(h);
  var release = function () {
    el.removeEventListener("load", release);
    el.removeEventListener("error", release);
    DF.assets.revoke(u);
  };
  el.addEventListener("load", release);
  el.addEventListener("error", release);
  return u;
}

function wrapSetter(proto, prop, oneShot) {
  var d = Object.getOwnPropertyDescriptor(proto, prop);
  if (!d || !d.set) return;
  Object.defineProperty(proto, prop, {
    configurable: true, enumerable: d.enumerable, get: d.get,
    set: function (v) {
      if (oneShot) { var h = lookupHash(v); if (h) return d.set.call(this, oneShotScript(this, h)); }
      var r = lookup(v);
      return d.set.call(this, r || v);
    }
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
    if ((n === "src" || n === "href") && typeof value === "string") {
      var h = n === "src" && this.tagName === "SCRIPT" ? lookupHash(value) : null;
      if (h) value = oneShotScript(this, h);
      else { var r = lookup(value); if (r) value = r; }
    }
    return origSetAttr.call(this, name, value);
  };
  [[HTMLScriptElement, "src", true], [HTMLImageElement, "src"], [HTMLMediaElement, "src"], [HTMLSourceElement, "src"],
   [HTMLLinkElement, "href"], [HTMLEmbedElement, "src"], [HTMLIFrameElement, "src"]].forEach(function (p) {
    if (p[0]) wrapSetter(p[0].prototype, p[1], !!p[2]);
  });
  var origOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url) {
    var args = [].slice.call(arguments), h = lookupHash(String(url));
    if (h) {
      var u = DF.assets.fresh(h), xhr = this;
      args[1] = u;
      var release = function () { xhr.removeEventListener("loadend", release); DF.assets.revoke(u); };
      xhr.addEventListener("loadend", release);
    }
    return origOpen.apply(this, args);
  };
  if (window.fetch) {
    var origFetch = window.fetch;
    window.fetch = function (input, init) {
      var h = lookupHash(typeof input === "string" ? input : (input && input.url));
      if (!h) return origFetch.call(this, input, init);
      var u = DF.assets.fresh(h);
      var p = origFetch.call(this, u, init);
      p.then(function () { DF.assets.revoke(u); }, function () { DF.assets.revoke(u); });
      return p;
    };
  }
  if (window.Worker) {
    var OrigWorker = window.Worker;
    var W = function (url, opts) {
      var h = lookupHash(String(url));
      if (!h) return new OrigWorker(url, opts);
      var u = DF.assets.fresh(h), w = new OrigWorker(u, opts);
      DF.assets.revoke(u);
      return w;
    };
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
        var m = /^deckfold:([0-9a-f]{20})(#.*)?$/.exec(src);
        s.src = m ? oneShotScript(s, m[1]) + (m[2] || "") : fillPlaceholders(src);
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
// With cssOnly only styles and fonts: the show window before "Start", whose document stays inert
// until DF.activateScripts (so no medium plays and no source script runs on the start screen).
DF.activate = function (opts) {
  DF.meta = readJSON("df-meta") || { deck: { slides: [] }, shim: {} };
  // the content model (docs/content-model.md): what is written on the slides. A version this
  // runtime does not know counts as no model.
  var content = readJSON("df-content");
  DF.content = content && content.version === 1 && content.slides ? content : null;
  return activateCSS().then(function () {
    if (opts.cssOnly) return; // the start screen does not wait for the fonts of the deck
    restoreAttrs(!opts.runScripts);
    return fontsReady();
  }).then(function () {
    if (!opts.runScripts) return;
    installShim();
    return runDeferred();
  });
};
function fontsReady() {
  var fonts = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
  return Promise.race([fonts, new Promise(function (r) { setTimeout(r, 3000); })]);
}
// activateScripts: the rest of a cssOnly activation (attributes, fonts, shim, source scripts).
DF.activateScripts = function () {
  restoreAttrs(false);
  return fontsReady().then(function () {
    installShim();
    return runDeferred();
  });
};
// restoreIn: attributes of a copied subtree (slide tiles), with media kept quiet.
DF.restoreIn = function (node) { restoreAttrs(true, node); };
DF.snapshot = snapshot;
DF.quietMedia = quietMedia;

// --- 20-model.js ---
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
    // buildWait: how long to wait after the key of a build before the next key, in ms. The player
    // draws every build itself and loses a key that comes while one is still drawn (measured on
    // builds of 0.5 s: with keys 250 ms apart two of three builds arrived, from 400 ms on all
    // three), and it has no event for the end of a build. deck.extra.builds has the duration of
    // every build of a slide; a file converted before it had them waits for a build of 0.5 s.
    buildWait: function (slide, build) {
      var times = (((DF.meta.deck || {}).extra || {}).builds || [])[slide];
      var s = times && typeof times[build] === "number" ? times[build] : 0.5;
      return Math.max(250, Math.round(s * 1000) + 100);
    },
    // jump: the slide by its number (digits and Enter), then one key per build, each after the
    // build before it has been drawn. A preview reaches the end of a slide the same way, so it
    // shows the builds of the slide one after the other before it rests.
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
        var ms = self.buildWait(target.slide, cur.step);
        self.forward();
        cur = { slide: target.slide, step: cur.step + 1 };
        return wait(ms).then(loop);
      });
    },
    mediaRoot: function () { return document.getElementById("stage") || document.body; },
    // link: the target of a link to another slide under a click. The player draws the slide on a
    // canvas, so the link is found by its rectangle: deck.extra.links has, per slide, the links
    // of the export in slide units with the step in which each can be clicked, and the player's
    // stage element says where the slide is in the window and how large (it is scaled to fit).
    link: function (t, e, st) {
      var deck = DF.meta.deck || {}, links = ((deck.extra || {}).links || [])[st.slide];
      var stage = document.getElementById("stage");
      if (!links || !links.length || !stage || !e || !deck.width) return null;
      var r = stage.getBoundingClientRect();
      if (!r.width) return null;
      var k = deck.width / r.width, x = (e.clientX - r.left) * k, y = (e.clientY - r.top) * k;
      for (var i = 0; i < links.length; i++) {
        var l = links[i];
        if (l.step === st.step && x >= l.x && x <= l.x + l.w && y >= l.y && y <= l.y + l.h) return l.to;
      }
      return null;
    },
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
    // link: the target of a link to another slide under a click (see linkOf in the player)
    link: function (t) { return window.__dfPptx && window.__dfPptx.linkOf ? window.__dfPptx.linkOf(t) : null; },
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
// itself; guardClicks keeps such clicks away from the player's document listener; link(target,
// event) returns the target of a link to another slide under the click, which follow(target)
// then goes to (through deckfold's state, like every other move).
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
      var to = opts.link ? opts.link(e.target, e) : null;
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

// --- 32-ui.js ---
// Building blocks shared by the start screen and the presenter view: preview iframes and slide tiles.

// previewFrame: a muted srcdoc iframe that shows a state of the deck without animating it. The
// iframe runs the source player in the role "preview" and gets its assets from this window.
// The iframe always has the aspect ratio of the deck and is scaled into its pane: a player may
// change its layout with the shape of its viewport (the claude.ai player shows all slides in a
// scroll view when the viewport is not wider than high). Players scale their slides to the
// viewport, so a width of 1280 pixels is enough.
DF.previewFrame = function (host, snapshotHTML) {
  var meta = DF.meta.deck || {};
  var W = 1280, H = Math.round(W * (meta.height || 1080) / (meta.width || 1920));
  var f = DF.el("iframe", { "data-df-preview": "", tabindex: "-1", inert: true, title: "preview", "aria-hidden": "true" });
  f.style.width = W + "px"; f.style.height = H + "px";
  // until it has reached the first state asked of it, the iframe shows the first slide of a
  // freshly loaded player, which is not what it is meant to show
  f.style.visibility = "hidden";
  f.srcdoc = snapshotHTML;
  host.appendChild(f);
  host.appendChild(DF.el("div", { class: "df-shield" }));
  function fit() {
    var w = host.clientWidth, h = host.clientHeight;
    if (!w || !h) return;
    var k = Math.min(w / W, h / H);
    f.style.transform = "translate(" + ((w - W * k) / 2) + "px," + ((h - H * k) / 2) + "px) scale(" + k + ")";
  }
  if ("ResizeObserver" in window) new ResizeObserver(fit).observe(host);
  else window.addEventListener("resize", fit);
  fit();
  var rec = { el: f, want: null, ready: false };
  rec.show = function (s) { rec.want = s; push(); };
  function push() {
    if (!rec.ready || !rec.want) return;
    var done = null;
    try { done = f.contentWindow.__deckfold.setPreviewState(rec.want); } catch (e) { }
    if (f.style.visibility === "hidden") Promise.resolve(done).then(function () { f.style.visibility = ""; });
  }
  f.addEventListener("load", function () {
    var w = f.contentWindow;
    (function waitReady() {
      if (w.__deckfold && w.__deckfold.previewReady) w.__deckfold.previewReady.then(function () { rec.ready = true; push(); });
      else setTimeout(waitReady, 50);
    })();
  });
  return rec;
};

// slideTile: a button with a picture of slide i (the thumbnail of the source, otherwise a copy of
// the slide). The picture is built only when the tile becomes visible, so that a deck with
// hundreds of slides does not copy all of them at once.
var tileObserver = null;
function fillTile(box) {
  if (box.__dfFilled) return;
  box.__dfFilled = true;
  var meta = DF.meta.deck || {}, i = +box.getAttribute("data-i"), s = DF.slides()[i] || {};
  var w = meta.width || 1920;
  if (s.thumb) { box.appendChild(DF.el("img", { src: DF.assets.url(s.thumb), alt: "" })); return; }
  var src = document.querySelectorAll(meta.slideSelector || ".deck-slide")[i];
  if (!src) return;
  var canvas = src.querySelector(".deck-scaler > *") || src;
  var c = canvas.cloneNode(true);
  [].slice.call(c.querySelectorAll("script,iframe,video,audio")).forEach(function (x) { x.parentNode.removeChild(x); });
  c.removeAttribute("id");
  DF.restoreIn(c);
  var holder = DF.el("div", { class: "df-clone" });
  holder.style.width = w + "px"; holder.style.height = (meta.height || 1080) + "px";
  holder.appendChild(c);
  box.appendChild(holder);
  scaleTile(box);
}
function scaleTile(box) {
  var c = box.querySelector(".df-clone");
  if (c) c.style.transform = "scale(" + (box.clientWidth / ((DF.meta.deck || {}).width || 1920)) + ")";
}
DF.slideTile = function (i, onclick) {
  var meta = DF.meta.deck || {}, s = DF.slides()[i] || {};
  var box = DF.el("div", { class: "df-thumb", "data-i": String(i) });
  box.style.aspectRatio = (meta.width || 1920) + " / " + (meta.height || 1080);
  var tile = DF.el("button", { type: "button", class: "df-tile", "data-i": String(i), onclick: onclick }, [box,
    DF.el("span", { text: (i + 1) + (s.label ? " \u00b7 " + s.label : "") })]);
  if (!tileObserver && "IntersectionObserver" in window) {
    tileObserver = new IntersectionObserver(function (ents) {
      ents.forEach(function (en) { if (en.isIntersecting) { fillTile(en.target); tileObserver.unobserve(en.target); } });
    }, { rootMargin: "200px" });
  }
  if (tileObserver) tileObserver.observe(box); else setTimeout(function () { fillTile(box); }, 0);
  return tile;
};
DF.rescaleTiles = function (root) { [].slice.call(root.querySelectorAll(".df-thumb")).forEach(scaleTile); };

// slideStrip: a horizontally scrolling band of slide tiles with a subtle arrow button on each side,
// used by the start screen and by the presenter view. opts.pick(i) is called for a click on a
// tile, opts.step(-1 | 1) for the arrows. mark(i) highlights the tile and scrolls it into view;
// nav(canBack, canForward) enables or disables the arrows.
DF.slideStrip = function (opts) {
  var band = DF.el("div", { class: "df-strip", "data-df-strip": "" });
  DF.slides().forEach(function (s, i) { band.appendChild(DF.slideTile(i, function () { opts.pick(i); })); });
  var back = DF.el("button", { type: "button", class: "df-strip-nav", "data-df-strip-prev": "", "aria-label": DF.t("previous"), title: DF.t("previous"), text: "\u2039", onclick: function () { opts.step(-1); } });
  var fwd = DF.el("button", { type: "button", class: "df-strip-nav", "data-df-strip-next": "", "aria-label": DF.t("following"), title: DF.t("following"), text: "\u203a", onclick: function () { opts.step(1); } });
  var el = DF.el("div", { class: "df-strip-wrap" }, [back, band, fwd]);
  window.addEventListener("resize", function () { DF.rescaleTiles(band); });
  return {
    el: el, band: band,
    mark: function (i) {
      [].slice.call(band.querySelectorAll(".df-tile")).forEach(function (t) { t.classList.toggle("df-cur", +t.getAttribute("data-i") === i); });
      var cur = band.querySelector('.df-tile[data-i="' + i + '"]');
      if (cur && cur.scrollIntoView) cur.scrollIntoView({ block: "nearest", inline: "nearest" });
      // a tile with the keyboard focus (reached with Tab) hands it to the marked tile, so that the
      // focus ring and the mark are on the same tile
      var a = document.activeElement;
      if (cur && a && a !== cur && a.classList && a.classList.contains("df-tile") && band.contains(a)) {
        try { cur.focus({ preventScroll: true }); } catch (e) { cur.focus(); }
      }
    },
    nav: function (canBack, canFwd) { back.disabled = !canBack; fwd.disabled = !canFwd; },
    // scroll: move the band by most of its width without choosing a slide
    scroll: function (dir) { band.scrollBy({ left: dir * band.clientWidth * 0.8, behavior: "smooth" }); }
  };
};

// A control clicked with the mouse gives the focus back: the keys move through the slides, and a
// focused button would show its focus ring after the next key (next to the marked slide tile) or
// be pressed by Space. Text and number fields keep the focus for typing; a control reached with
// the keyboard keeps it too (a click from Enter or Space has detail 0).
document.addEventListener("click", function (e) {
  if (!e.detail) return;
  var c = e.target && e.target.closest && e.target.closest("[data-df-ui] button, [data-df-ui] input[type=checkbox]");
  if (!c) return;
  setTimeout(function () { if (document.activeElement === c && c.blur) c.blur(); }, 0);
}, true);

// keyHelp: the key bindings in groups, side by side. groups: [{ name, title, rows: [{ action,
// keys (as configured, e.g. "ArrowRight", " ", "f"), label, caps (the key caps to show instead of
// the ones of keys, for fixed keys and for what moves without a key, such as a click) }] }].
// Each row carries data-df-action and data-df-keys (the configured keys), so tests read it
// without depending on the language.
var KEYNAMES = { ArrowRight: "\u2192", ArrowLeft: "\u2190", ArrowUp: "\u2191", ArrowDown: "\u2193", " ": "keySpace", PageDown: "keyPgDn", PageUp: "keyPgUp",
  Home: "keyHome", End: "keyEnd", Backspace: "keyBackspace", Escape: "Esc" };
function keyCap(k) {
  var n = KEYNAMES[k];
  if (n) return n.length > 1 && n.indexOf("key") === 0 ? DF.t(n) : n;
  return k.length === 1 ? k.toUpperCase() : k;
}
DF.keyCap = keyCap;
// keyCaps: the caps of some keys, each once (two keys can have the same cap)
DF.keyCaps = function (keys) {
  var caps = [];
  keys.forEach(function (k) { var c = keyCap(k); if (caps.indexOf(c) < 0) caps.push(c); });
  return caps;
};
DF.keyHelp = function (groups) {
  return DF.el("div", { class: "df-keyhelp", "data-df-keyhelp": "" }, groups.map(function (g) {
    return DF.el("div", { class: "df-keygroup", "data-df-keygroup": g.name }, [DF.el("h4", { text: g.title })].concat(g.rows.map(function (r) {
      var caps = r.caps || DF.keyCaps(r.keys || []);
      return DF.el("div", { class: "df-keyrow", "data-df-action": r.action, "data-df-keys": JSON.stringify(r.keys || []) }, [
        DF.el("span", { class: "df-keylabel", text: r.label }),
        DF.el("span", { class: "df-caps" }, caps.map(function (c) { return DF.el("kbd", { text: c }); }))]);
    })));
  }));
};

// DF.overlay: what the overlays of deckfold share (the search, the key help of the start screen):
// a dimmed backdrop above the start screen or the presenter view, with a box in it that the caller
// appends to root. While it is open it takes every key (DF.modalKeys). Escape, its close button and
// a click beside the box close it, and the focus goes back to the document, so that the keys move
// the slides again and no button shows a focus ring. Tab moves the focus through the stops of the
// overlay and stays in it; on the close button Enter and Space press it.
// opts: attr (data attribute of the root), cls (its class), keys(type, e) (every key event the
// overlay does not take itself; optional), stops() (the elements Tab moves through; the close
// button by default), onOpen() (optional).
// Returns { root, closeButton, open, close, isOpen, remove }.
DF.overlay = function (opts) {
  var root = DF.el("div", { "data-df-ui": "", class: "df-overlay " + opts.cls });
  root.setAttribute(opts.attr, "");
  document.body.appendChild(root);
  var isOpen = false;
  function keys(type, e) {
    e.stopImmediatePropagation();
    var onClose = document.activeElement === closeButton;
    if (e.key === "Tab" || (onClose && (e.key === "Enter" || e.key === " "))) {
      // also on keyup and keypress, so that the browser neither moves the focus nor clicks
      e.preventDefault();
      if (type !== "keydown") return;
      if (e.key !== "Tab") { close(); return; }
      var stops = opts.stops ? opts.stops() : [closeButton];
      var at = stops.indexOf(document.activeElement);
      var to = stops[(at < 0 ? (e.shiftKey ? stops.length - 1 : 0) : at + (e.shiftKey ? stops.length - 1 : 1)) % stops.length];
      try { to.focus({ preventScroll: true }); } catch (err) { to.focus(); }
      return;
    }
    if (type === "keydown" && e.key === "Escape") { e.preventDefault(); close(); return; }
    if (opts.keys) opts.keys(type, e);
  }
  var closeButton = DF.el("button", { type: "button", class: "df-close", "data-df-close": "", title: DF.t("close"), "aria-label": DF.t("close"), text: "\u00d7", onclick: close });
  function open() {
    if (isOpen || !root.parentNode) return;
    isOpen = true;
    root.classList.add("df-on");
    DF.modalKeys = keys;
    if (opts.onOpen) opts.onOpen();
  }
  function close() {
    if (!isOpen) return;
    isOpen = false;
    root.classList.remove("df-on");
    if (DF.modalKeys === keys) DF.modalKeys = null;
    var a = document.activeElement;
    if (a && root.contains(a) && a.blur) a.blur();
  }
  root.addEventListener("mousedown", function (e) { if (e.target === root) close(); });
  return {
    root: root, closeButton: closeButton,
    open: open, close: close, isOpen: function () { return isOpen; },
    remove: function () { close(); if (root.parentNode) root.parentNode.removeChild(root); }
  };
};

// DF.finder: search and outline (docs/content-model.md). An overlay with a text field and a list:
// without a search text every slide with its title (a slide without one shows the start of its
// text), with a search text the slides that hold it, each with the first place found. Choosing
// an entry closes the overlay and calls opts.pick(slide).
// opts: notes (search the presenter notes too), pick(i), current() (the slide shown now).
// Returns null when the deck has no content model; otherwise { button, open, close, isOpen,
// remove }.
DF.finder = function (opts) {
  var model = DF.content, slides = DF.slides();
  if (!model || !slides.length || model.slides.length !== slides.length) return null;
  var input = DF.el("input", { type: "text", "data-df-find-input": "", placeholder: DF.t("search"), "aria-label": DF.t("search"),
    autocomplete: "off", autocapitalize: "off", spellcheck: "false" });
  var list = DF.el("div", { class: "df-find-list" });
  var ov = DF.overlay({ attr: "data-df-finder", cls: "df-finder", keys: keys, stops: function () { return [input, ov.closeButton]; }, onOpen: function () {
    input.value = "";
    render();
    try { input.focus({ preventScroll: true }); } catch (e) { input.focus(); }
  } });
  // the close button sits beside the text field, which would otherwise lie under it
  ov.root.appendChild(DF.el("div", { class: "df-overlay-box df-find-box" }, [DF.el("div", { class: "df-find-head" }, [input, ov.closeButton]), list]));
  var active = 0, items = [];

  // fold: lower case without accents, one character for each character of s, so that a place
  // found in the folded text is the same place in s
  function fold(s) {
    var out = "";
    for (var i = 0; i < s.length; i++) {
      var c = s.charAt(i), f = c;
      try { f = c.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase(); } catch (e) { f = c.toLowerCase(); }
      out += f.length === 1 ? f : (f.charAt(0) || c);
    }
    return out;
  }
  function cut(s, n) { return s.length > n ? s.slice(0, n).replace(/\s+\S*$/, "") + " \u2026" : s; }

  // entries: what the list shows for the search text q (folded)
  function entries(q) {
    var out = [];
    slides.forEach(function (sl, i) {
      var sc = model.slides[i], title = sc.title || "";
      if (!q) {
        out.push({ i: i, title: title, start: title || !sc.blocks.length ? "" : cut(sc.blocks[0].text, 90) });
        return;
      }
      var hits = 0, first = null;
      var look = function (text, notes) {
        var at = fold(text).indexOf(q);
        if (at < 0) return;
        hits++;
        if (!first) first = { text: text, at: at, notes: notes };
      };
      // the title counts once: as its block when the slide has one, otherwise by itself
      var ft = fold(title);
      if (title && !sc.blocks.some(function (b) { return fold(b.text) === ft; })) look(title, false);
      sc.blocks.forEach(function (b) { look(b.text, false); });
      if (opts.notes && sl.notes) look(sl.notes, true);
      if (hits) out.push({ i: i, title: title, hits: hits, first: first });
    });
    return out;
  }
  // place: the text around a place found, with the found part marked. Everything is a text node.
  function place(f, len) {
    var from = Math.max(0, f.at - 36), to = Math.min(f.text.length, f.at + len + 60);
    return DF.el("span", { class: "df-find-place" }, [
      f.notes ? DF.el("span", { class: "df-find-kind", "data-df-find-notes": "", text: DF.t("notes") }) : null,
      (from > 0 ? "\u2026" : "") + f.text.slice(from, f.at),
      DF.el("mark", { text: f.text.slice(f.at, f.at + len) }),
      f.text.slice(f.at + len, to) + (to < f.text.length ? "\u2026" : "")]);
  }
  function render() {
    var q = fold(input.value.replace(/^\s+|\s+$/g, ""));
    var cur = opts.current ? opts.current() : -1;
    while (list.firstChild) list.removeChild(list.firstChild);
    items = entries(q).map(function (e) {
      var b = DF.el("button", { type: "button", class: "df-find-item" + (e.i === cur ? " df-cur" : ""), "data-df-find-item": "", "data-i": String(e.i),
        onclick: function () { choose(e.i); } }, [
        DF.el("span", { class: "df-find-n", text: String(e.i + 1) }),
        DF.el("span", { class: "df-find-main" }, [
          DF.el("span", { class: "df-find-title", text: e.title || (DF.t("slide") + " " + (e.i + 1)) }),
          e.first ? place(e.first, q.length) : (e.start ? DF.el("span", { class: "df-find-place", text: e.start }) : null)]),
        e.hits > 1 ? DF.el("span", { class: "df-find-count", "data-df-find-count": String(e.hits), text: e.hits + "\u00d7" }) : null]);
      list.appendChild(b);
      return { i: e.i, el: b };
    });
    if (!items.length) list.appendChild(DF.el("p", { class: "df-find-none", "data-df-find-none": "", text: DF.t("noResults") }));
    // the outline starts at the slide shown now, a search at its first entry
    active = 0;
    if (!q) items.forEach(function (it, k) { if (it.i === cur) active = k; });
    mark();
  }
  function mark() {
    items.forEach(function (it, k) { it.el.classList.toggle("df-active", k === active); });
    var a = items[active];
    if (a && a.el.scrollIntoView) a.el.scrollIntoView({ block: "nearest" });
  }
  function choose(i) { ov.close(); opts.pick(i); }
  function keys(type, e) {
    if (type !== "keydown") return;
    switch (e.key) {
      case "ArrowDown": case "ArrowUp":
        e.preventDefault();
        if (items.length) { active = (active + (e.key === "ArrowDown" ? 1 : items.length - 1)) % items.length; mark(); }
        return;
      case "Enter": e.preventDefault(); if (items[active]) choose(items[active].i); return;
    }
    // Ctrl+F and Cmd+F stay with the overlay instead of opening the browser's own search
    if ((e.metaKey || e.ctrlKey) && (e.key === "f" || e.key === "F")) { e.preventDefault(); input.select(); return; }
    // typing goes to the text field wherever the focus is (a key such as Shift leaves it alone)
    if (document.activeElement !== input && (e.key.length === 1 || e.key === "Backspace" || e.key === "Delete")) input.focus();
  }
  input.addEventListener("input", render);
  return {
    button: DF.el("button", { type: "button", "data-df-find": "", text: DF.t("search"), onclick: ov.open }),
    open: ov.open, close: ov.close, isOpen: ov.isOpen, remove: ov.remove
  };
};

// The keyboard icon of the key help button: its case, two rows of keys and the space bar. It is
// drawn here so that it looks the same on every machine (a character such as U+2328 would depend
// on the fonts installed) and takes the text colour.
function keyboardIcon() {
  var ns = "http://www.w3.org/2000/svg";
  var make = function (tag, attrs) { var e = document.createElementNS(ns, tag); for (var k in attrs) e.setAttribute(k, attrs[k]); return e; };
  var svg = make("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "none", stroke: "currentColor", "stroke-width": "1.6",
    "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true", focusable: "false" });
  svg.appendChild(make("rect", { x: "2.5", y: "6", width: "19", height: "12", rx: "2" }));
  svg.appendChild(make("path", { d: "M6.5 9.75h.01M9.25 9.75h.01M12 9.75h.01M14.75 9.75h.01M17.5 9.75h.01M6.5 12.5h.01M9.25 12.5h.01M12 12.5h.01M14.75 12.5h.01M17.5 12.5h.01M8.5 15.25h7" }));
  return svg;
}

// DF.helpOverlay: the key help of the start screen behind a button with an icon: an overlay with
// the groups of DF.keyHelp and, below them, the version of deckfold that wrote the file. It has no
// key of its own; while it is open no key reaches the start screen.
// Returns { button, open, close, isOpen, remove }.
DF.helpOverlay = function (groups) {
  var name = DF.t("shortcuts");
  var ov = DF.overlay({ attr: "data-df-help-overlay", cls: "df-help", keys: function (type, e) {
    // Nothing in the box is used with the keys, so what a key would do by itself is cancelled: it
    // must not scroll the start screen behind the overlay, press a button or, with Ctrl+F or
    // Cmd+F, open the search of the browser. Other shortcuts of the browser (reload) stay.
    var find = (e.metaKey || e.ctrlKey) && (e.key === "f" || e.key === "F");
    if (find || !(e.metaKey || e.ctrlKey || e.altKey)) e.preventDefault();
  } });
  ov.root.appendChild(DF.el("div", { class: "df-overlay-box df-help-box" }, [
    DF.el("div", { class: "df-help-head" }, [DF.el("h2", { text: name }), ov.closeButton]),
    DF.keyHelp(groups),
    DF.el("p", { class: "df-ver", text: "deckfold " + (DF.version || "") })]));
  return {
    button: DF.el("button", { type: "button", class: "df-icon-btn", "data-df-help": "", title: name, "aria-label": name, onclick: ov.open }, [keyboardIcon()]),
    open: ov.open, close: ov.close, isOpen: ov.isOpen, remove: ov.remove
  };
};

// --- 35-start.js ---
// Start screen of the show window (decision E14): the title, a large preview of the chosen slide, a
// strip of all slides to choose where to start, the settings with "Start", and below the settings
// a footer with the button that opens the key help, the search and the number of slides. The
// window is the presentation itself: "Start" activates the source player here. The presenter view
// is a second window that the switch opens; deckfold never places windows and does not check where
// they are: the user drags them where they belong (decision E14).

DF.startShow = function (snapshotHTML) {
  var generic = DF.format() === "generic";
  var slides = DF.slides();
  var meta = DF.meta.deck || {};
  // the token of this deck's session: the one from before a reload of this window, otherwise a
  // new one. Another deck opened in the same tab has its own, so it does not answer the presenter
  // view of this one.
  DF.token = DF.ss.get(DF.deckKey("df-token")) || DF.randomToken();
  DF.ss.set(DF.deckKey("df-token"), DF.token);
  // A source player can read its position from the address (the claude.ai player reads #n). After
  // the reload at the end the address still holds the last slide, which would override the chosen
  // start slide, so the hash goes before the player runs.
  function dropHash() {
    if (!location.hash) return;
    try { history.replaceState(null, "", location.pathname + location.search); } catch (e) { }
  }
  dropHash();
  var cfg = DF.settings();
  var phase = "start"; // start, loading, show
  var rWin = null;
  var last = DF.ss.get(DF.deckKey("df-last"));
  var sel = last && slides.length ? Math.max(0, Math.min(slides.length - 1, last.slide)) : 0;
  var send = DF.channel(function () { return rWin; }, function () { return DF.token; }, onMsg);
  document.documentElement.classList.add("df-start-mode");

  // ---------- start screen ----------
  var frame = null, strip = null, hintEl, fsBox, preBox = null, autoBox = null, intervalIn = null, loopBox = null, startBtn;
  var showFs = !(cfg.show && cfg.show.fullscreen === false);
  function check(attr, label, checked, onchange) {
    var box = DF.el("input", { type: "checkbox", checked: checked });
    box.setAttribute(attr, "");
    box.addEventListener("change", onchange);
    return { box: box, label: DF.el("label", { class: "df-check" }, [box, " " + label]) };
  }
  var fs = check("data-df-fs", DF.t("fullscreen"), showFs, function () {
    cfg.show = cfg.show || {}; cfg.show.fullscreen = fs.box.checked; DF.saveSettings(cfg);
  });
  fsBox = fs.box;
  var opts = [];
  if (!generic) {
    var pre = check("data-df-presenter", DF.t("presenterView"), false, function () { if (pre.box.checked) openR(); else closeR(); });
    preBox = pre.box;
    opts.push(DF.el("div", { class: "df-opt" }, [pre.label]));
  }
  opts.push(DF.el("div", { class: "df-opt" }, [fs.label]));
  if (!generic) {
    var au = check("data-df-auto", DF.t("auto"), cfg.auto.enabled, saveAuto);
    autoBox = au.box;
    intervalIn = DF.el("input", { type: "number", "data-df-interval": "", min: "1", max: "3600", step: "0.5", value: String(cfg.auto.intervalSeconds) });
    intervalIn.addEventListener("change", saveAuto);
    var lp = check("data-df-loop", DF.t("autoLoop"), cfg.auto.loop, saveAuto);
    loopBox = lp.box;
    opts.push(DF.el("div", { class: "df-opt df-opt-row" }, [au.label,
      DF.el("label", { class: "df-check" }, [DF.t("autoInterval") + " ", intervalIn]), lp.label]));
  }
  // interval and loop can only be set while auto mode is checked
  function autoFields() {
    intervalIn.disabled = loopBox.disabled = !autoBox.checked;
    intervalIn.parentNode.classList.toggle("df-disabled", !autoBox.checked);
    loopBox.parentNode.classList.toggle("df-disabled", !autoBox.checked);
  }
  function saveAuto() {
    autoFields();
    var v = parseFloat(intervalIn.value);
    cfg.auto.enabled = autoBox.checked;
    cfg.auto.intervalSeconds = isFinite(v) ? Math.max(1, Math.min(3600, v)) : 8;
    cfg.auto.loop = loopBox.checked;
    DF.saveSettings(cfg);
  }
  hintEl = DF.el("p", { class: "df-start-hint", "data-df-hint": "" });
  startBtn = DF.el("button", { type: "button", class: "df-primary df-start-btn", "data-df-start": "", text: DF.t("start"), onclick: begin });
  // the preview has the shape of the slide (--df-ar, width by height) and takes the height the
  // other rows leave: its frame shrinks, and the stage keeps its shape inside it
  var stage = DF.el("div", { class: "df-start-stage" });
  var view = DF.el("div", { class: "df-start-view" }, [stage]);
  view.style.setProperty("--df-ar", String((meta.width || 1920) / (meta.height || 1080)));
  if (!generic) strip = DF.slideStrip({ pick: function (i) { setSel(i); }, step: function (d) { setSel(sel + d); } });
  // search and outline: only the slide text here, because this window may already be on the
  // projector; the presenter view searches the notes too
  var finder = generic ? null : DF.finder({ notes: false, pick: function (i) { setSel(i); }, current: function () { return sel; } });
  // the key help and the version of deckfold are behind the button at the start of the footer
  var help = DF.helpOverlay(keyGroups());
  var root = DF.el("div", { "data-df-ui": "", class: "df-start" }, [
    DF.el("div", { class: "df-start-box" }, [
      DF.el("h1", { text: meta.title || document.title || "deckfold" }),
      generic ? DF.el("p", { class: "df-sub", text: DF.t("generic") }) : null,
      generic ? null : view,
      generic ? null : strip.el,
      // the settings with "Start" beside them, and below the settings the footer, everything at
      // the left: the key help, the search, the number of slides. Nothing stands below "Start".
      DF.el("div", { class: "df-start-row" }, [DF.el("div", { class: "df-opts" }, opts), startBtn, hintEl,
        DF.el("div", { class: "df-start-foot" }, [help.button, finder ? finder.button : null,
          generic ? null : DF.el("span", { class: "df-count", "data-df-count": String(slides.length), text: slides.length + " " + DF.t("slides") })])
      ])
    ])
  ]);
  document.body.appendChild(root);

  // the complete key help, built from the keys as configured, so that it always matches them:
  // start screen, slides, display, show, presenter view
  function keyGroups() {
    var k = cfg.keys || {};
    var row = function (action, label) { return { action: action, keys: k[action] || [], label: label }; };
    var start = { name: "start", title: DF.t("keysStart"), rows: [] };
    // choosing the start slide takes every key of back and next; the first of each stands for them
    if (!generic) start.rows.push({ action: "choose", keys: (k.prev || []).concat(k.next || []), label: DF.t("chooseSlide"),
      caps: [(k.prev || [])[0], (k.next || [])[0]].filter(Boolean).map(DF.keyCap) });
    start.rows.push({ action: "start", caps: ["Enter"], label: DF.t("start") });
    var display = { name: "display", title: DF.t("keysDisplay"), rows: [row("fullscreen", DF.t("fullscreen")), row("black", DF.t("black")), row("white", DF.t("white"))] };
    if (generic) return [start, display, { name: "run", title: DF.t("keysRun"), rows: [row("end", DF.t("endShow"))] }];
    return [start,
      // a click moves on like the keys of "next": it stands in their row, after them
      { name: "slides", title: DF.t("keysSlides"), rows: [{ action: "next", keys: k.next || [], label: DF.t("following"), caps: DF.keyCaps(k.next || []).concat([DF.t("click")]) },
        row("prev", DF.t("previous")), row("first", DF.t("firstSlide")), row("last", DF.t("lastSlide")),
        { action: "goto", caps: [DF.t("keyDigits"), "Enter"], label: DF.t("gotoSlide") }].concat(finder ? [row("find", DF.t("search"))] : []) },
      display,
      { name: "run", title: DF.t("keysRun"), rows: [row("auto", DF.t("auto")), row("end", DF.t("endShow"))] },
      { name: "presenter", title: DF.t("presenterView"), rows: [row("timerReset", DF.t("timerReset"))] }];
  }
  try { performance.mark("deckfold:start-screen"); } catch (e) { }
  if (!generic) {
    // the preview runs the source player in an iframe on this window's thread; it is created after
    // the start screen has been drawn, so that the screen can be used at once
    requestAnimationFrame(function () {
      setTimeout(function () { frame = DF.previewFrame(stage, snapshotHTML); frame.show(finalState(sel)); }, 0);
    });
    autoFields();
    setSel(sel);
  }
  try { startBtn.focus({ preventScroll: true }); } catch (e) { }

  // the large preview shows the chosen slide as it looks after all its steps (effects included)
  function finalState(i) { return DF.model.end(i); }
  function setSel(i) {
    if (generic || !slides.length) return;
    sel = Math.max(0, Math.min(slides.length - 1, i));
    // the choice survives a reload; the show writes the same key, so the later of the two counts
    DF.ss.set(DF.deckKey("df-last"), { slide: sel, step: 0 });
    if (frame) frame.show(finalState(sel));
    strip.mark(sel);
    strip.nav(sel > 0, sel < slides.length - 1);
    beat();
  }

  // ---------- presenter view window ----------
  function openR() {
    if (rWin && !rWin.closed) return;
    rWin = window.open(location.href.split("#")[0], "df-presenter:" + DF.token, "popup,width=1200,height=760");
    if (!rWin) { preBox.checked = false; hint("popupBlocked"); return; }
    hint(null);
  }
  var closedWin = null; // the window the switch closed: its last messages must not bring it back
  function closeR() {
    if (rWin && !rWin.closed) { try { rWin.close(); } catch (e) { } }
    closedWin = rWin;
    rWin = null;
  }
  function onMsg(d, source) {
    if (!rWin && source) {
      if (source === closedWin || source.closed) return;
      rWin = source; if (preBox) preBox.checked = true; // after a reload of this window
    }
    if (d.type === "hello") { send(status("init")); return; }
    if (d.type === "hb") return;
    if (d.type === "cmd") {
      if (phase === "start") return selectBy(d.cmd, d.arg);
      // from "Start" on: to the presentation; while it is still loading, kept for it
      if (DF.audience) return DF.audience.act(d.cmd, d.arg);
      early.push([d.cmd, d.arg]);
    }
  }
  // early: commands and keys between "Start" and the presentation being able to take them (the
  // presenter view already shows the show then); carried out in order once it can
  var early = [];
  function runEarly() {
    var q = early; early = [];
    q.forEach(function (c) { DF.audience.act(c[0], c[1]); });
  }
  function selectBy(cmd, arg) {
    switch (cmd) {
      case "next": return setSel(sel + 1);
      case "prev": return setSel(sel - 1);
      case "first": return setSel(0);
      case "last": return setSel(slides.length - 1);
      case "goto": return setSel(arg);
    }
  }
  function status(type) {
    var m = { type: type, phase: phase === "show" ? "show" : "start", sel: sel };
    if (phase === "show" && DF.audience) { var i = DF.audience.info(); for (var k in i) m[k] = i[k]; }
    return m;
  }
  function beat() { if (rWin) send(status("hb")); }
  setInterval(function () {
    if (rWin && rWin.closed) { rWin = null; if (preBox) preBox.checked = false; }
    beat();
  }, 1000);
  function hint(k) { hintEl.textContent = k ? DF.t(k) : ""; hintEl.classList.toggle("df-on", !!k); }

  // ---------- keys on the start screen ----------
  DF.installKeys(function (a, e) {
    if (phase === "loading") {
      // keys pressed while the presentation loads (until it takes them over)
      if (a !== "digits" && a !== "start") early.push([a, a === "goto" ? e.slide : undefined]);
      return true;
    }
    if (phase !== "start") return true;
    if (a === "find") { if (finder) finder.open(); return true; }
    // Enter starts here even where a configuration binds it to an action
    if (a === "start" || (e && e.key === "Enter" && a !== "goto")) { begin(); return true; }
    switch (a) {
      case "digits": return true;
      case "goto": setSel(e.slide); return true;
      case "next": case "prev": case "first": case "last": selectBy(a); return true;
      default: return true;
    }
  }, { swallowAll: true });

  // ---------- start and end ----------
  function begin(e) {
    if (phase !== "start") return;
    phase = "loading";
    // the click on "Start" must not reach the source player: it registers its listeners while the
    // scripts start and would take the click as "next"
    if (e && e.stopPropagation) e.stopPropagation();
    // full screen must be requested inside the click or key press, before anything async
    var wantFs = fsBox.checked;
    if (wantFs) DF.requestFs().catch(function () { });
    startBtn.disabled = true;
    startBtn.textContent = DF.t("loading");
    if (finder) finder.remove(); // no search in the presentation itself
    help.remove();
    var start = generic ? null : DF.model.goto(sel);
    document.documentElement.classList.remove("df-start-mode");
    dropHash();
    // the scripts start after the click or key event has ended
    new Promise(function (r) { setTimeout(r, 0); }).then(DF.activateScripts).then(function () {
      return DF.startAudience({
        start: start, fullscreen: wantFs,
        send: function (m) { m.sel = sel; return send(m); },
        onEnd: end
      });
    }).then(function () {
      phase = "show";
      root.parentNode.removeChild(root);
      runEarly();
      beat();
    }, function (err) { console.error("deckfold:", err); });
  }
  // end: back to the start screen with the last slide chosen, by reloading this window; the source
  // player stops with the reload, the presenter view stays open and reconnects
  function end(lastState) {
    if (lastState) DF.ss.set(DF.deckKey("df-last"), lastState);
    send({ type: "ended" });
    phase = "start";
    location.reload();
  }
};

// --- 40-audience.js ---
// The show window during the presentation (after "Start"): owns the state and drives the source
// player; auto mode, black/white screen, full-screen fallback, wake lock and mouse cursor.
//
// opts: start (state to begin with), fullscreen (the start screen asked for full screen),
// send(msg) (to the presenter view), onEnd(lastState). Returns a promise that resolves once the first slide is shown.

DF.startAudience = function (opts) {
  var send = opts.send;
  var generic = DF.format() === "generic";
  var adapter = DF.adapter();
  var st = { slide: 0, step: 0 }, lastShown = null;
  var driving = Promise.resolve();
  var blank = null;
  var auto = { on: false, due: 0, timer: 0, waiting: false };
  var cfg = DF.settings();
  document.documentElement.classList.add("df-audience");

  // ---------- Overlays ----------
  var blankEl = DF.el("div", { "data-df-ui": "", class: "df-blank" });
  var fsEl = DF.el("div", { "data-df-ui": "", class: "df-fs-overlay" }, [DF.el("button", { type: "button", "data-df-fs-overlay": "", text: DF.t("clickFs") })]);
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
  // The start screen requested full screen in the click on "Start". If that failed, one click on
  // the overlay is enough. While the presenter view is on the same screen, no full screen.
  var fsWanted = !!opts.fullscreen;
  function needFs() {
    return !document.fullscreenElement && !(window.innerHeight >= screen.height - 2 && window.innerWidth >= screen.width - 2);
  }
  function updateFsOverlay() { fsEl.classList.toggle("df-on", fsWanted && needFs()); }
  fsEl.addEventListener("click", function () { DF.requestFs().then(updateFsOverlay, updateFsOverlay); });
  document.addEventListener("fullscreenchange", updateFsOverlay);
  window.addEventListener("resize", updateFsOverlay);
  setInterval(updateFsOverlay, 1000);
  setTimeout(updateFsOverlay, 300);

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
  function wake() {
    if (!navigator.wakeLock || document.visibilityState !== "visible") return;
    navigator.wakeLock.request("screen").then(function () { }, function () { });
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
  function info() { return { state: st, blank: blank, auto: { on: auto.on, due: auto.due, waiting: auto.waiting }, ended: !!st.ended }; }
  function report() {
    var m = info();
    m.type = "state"; m.phase = "show";
    send(m);
    if (lastShown) DF.ss.set(DF.deckKey("df-last"), lastShown);
  }
  function go(target, o) {
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
    if (target.slide !== st.slide) before = st.slide;
    var to = st = { slide: target.slide, step: target.step };
    lastShown = st;
    drive(function () { return (o && o.jump) ? adapter.jump(to, from) : DF.drive(from, to); });
    report();
    scheduleAuto();
  }
  // before: the slide shown before the current one, for a link to "the last slide viewed"
  var before = null;
  // follow: a link to another slide on the current one (the content model's "jump"). A link goes
  // to the start of its slide; "next" and "prev" mean slides here, not steps.
  function follow(to) {
    if (st.ended) return;
    var n = DF.slides().length;
    switch (to) {
      case "next": return st.slide + 1 < n ? go(DF.model.goto(st.slide + 1)) : go(null);
      case "prev": return st.slide > 0 ? go(DF.model.goto(st.slide - 1)) : undefined;
      case "first": return go(DF.model.goto(0));
      case "last": return go(DF.model.goto(n - 1));
      case "back": return before != null ? go(DF.model.goto(before)) : undefined;
      case "end": return finish();
    }
    if (/^\d+$/.test(to) && +to < n) return go(DF.model.goto(+to));
  }
  // drive: one move of the player after the ones before it; a move that fails does not stop the
  // later ones
  var moving = 0, quietUntil = 0;
  function drive(move) {
    moving++;
    var done = function () { moving--; quietUntil = Date.now() + QUIET; };
    driving = driving.then(move).then(done, function (err) { done(); console.error("deckfold:", err); });
    return driving;
  }

  // ---------- player and deckfold kept together (decision E15) ----------
  // After the moves have settled, and once a second, the slide the player shows is compared with
  // deckfold's state; a player that went elsewhere (an input that reached it past deckfold, its
  // own timer) is brought back with a jump. At most two jumps for the same state, so that a
  // player that cannot go there is not jumped at forever (and its media not restarted).
  // QUIET: the slowest player took 223 ms from a jump to showing the slide (Keynote, measured).
  var QUIET = 1000, retry = { key: "", n: 0 };
  function check() {
    if (generic || !started || moving || Date.now() < quietUntil || !adapter.shown) return;
    var at = adapter.shown();
    if (at == null || at === st.slide) return;
    var key = st.slide + ":" + st.step;
    if (retry.key !== key) retry = { key: key, n: 0 };
    if (retry.n >= 2) {
      if (retry.n === 2) console.error("deckfold: the player stays on slide " + (at + 1) + " instead of " + (st.slide + 1));
      retry.n = 3;
      return;
    }
    retry.n++;
    var to = { slide: st.slide, step: st.step };
    drive(function () { return adapter.jump(to, null); });
  }
  setInterval(check, 1000);
  function finish() {
    stopAuto();
    opts.onEnd(lastShown || st);
  }
  function act(a, arg) {
    switch (a) {
      // the end slide stays until the presentation is ended (Escape, "End show")
      case "next": return st.ended ? undefined : go(DF.model.next(st));
      // on the first slide there is nothing before it: no end slide
      case "prev": if (st.ended) return go(st); var pv = DF.model.prev(st); return pv ? go(pv) : undefined;
      case "first": return go(DF.model.first());
      case "last": return go(DF.model.last());
      case "goto": return go(DF.model.goto(arg));
      case "state": return go(arg, { jump: true });
      case "follow": return follow(String(arg));
      case "black": return setBlank("black");
      case "white": return setBlank("white");
      case "auto": return auto.on ? stopAuto(true) : startAuto();
      case "fullscreen":
        if (document.fullscreenElement) return DF.toggleFs();
        fsWanted = true;
        return DF.requestFs().then(updateFsOverlay, updateFsOverlay);
      case "end": return finish();
    }
  }

  // ---------- auto mode (E7): the timer runs here, in the visible window ----------
  function sourceDelay() {
    var sl = DF.slides()[st.slide];
    // a negative delay means: no timing from the source for this step
    if (sl && sl.delay && sl.delay[st.step] != null && sl.delay[st.step] >= 0) return sl.delay[st.step] * 1000;
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
  // take: keys, clicks and commands of the presenter view. Until the player is ready and on the
  // start slide they are kept and then carried out in order: a step counted from the state before
  // the start slide would be lost.
  var started = false, held = [];
  function take(a, arg) {
    if (!started) { held.push([a, arg]); return; }
    return act(a, arg);
  }
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    take(a, e && e.slide);
    return true; // the player never gets a bound key; Escape (in full screen through Keyboard Lock) ends the show
  }, generic ? { passthrough: true, forward: function () { } } : { swallowAll: true });
  // a click on the slide moves on; the wheel and swipes do nothing (generic decks keep their own)
  if (!generic) DF.installPointer({ next: function () { take("next"); }, interactive: adapter.interactive, guardClicks: !!adapter.guardClicks,
    link: adapter.link ? function (t, e) { return adapter.link(t, e, st); } : null, follow: function (to) { take("follow", to); } });

  DF.audience = {
    act: take,
    state: function () { return st; },
    info: info,
    settings: function (s) { cfg = DF.merge(cfg, s || {}); if (auto.on) scheduleAuto(); }
  };

  // ---------- start ----------
  return adapter.ready().then(function () {
    if (!generic) {
      go(opts.start || DF.model.first(), { jump: true });
      if (cfg.auto.enabled) startAuto();
    }
    started = true;
    var q = held; held = [];
    q.forEach(function (c) { act(c[0], c[1]); });
    return driving;
  });
};

// --- 50-presenter.js ---
// Presenter view (role "presenter", E5): a window the show window opens. Current and next slide,
// each after all its steps, as muted srcdoc previews, notes, time, slide overview; its settings are changed here while
// it runs (elements, notes size, target duration, size of the panes). It sends commands to the
// show window (window.opener), which owns the state, and gets the state and heartbeat back.

DF.startPresenter = function (snapshotHTML) {
  var slides = DF.slides();
  var meta = DF.meta.deck || {};
  var show = null;
  try { show = window.opener; } catch (e) { show = null; }
  var send = DF.channel(function () { return show; }, function () { return DF.token; }, onMsg);
  var phase = "start", sel = 0, st = DF.model.first();
  var remote = { blank: null, auto: { on: false, due: 0 }, ended: false };
  var lastBeat = 0, cfg = DF.settings();
  var timer = { start: 0, accum: 0, paused: false };
  document.documentElement.classList.add("df-presenter-mode");

  // ---------- layout ----------
  var ui = DF.el("div", { "data-df-ui": "", class: "df-presenter" });
  var bannerEl = DF.el("div", { class: "df-banner", "data-df-banner": "" });
  var counterEl = DF.el("span", { class: "df-counter" });
  var clockEl = DF.el("span", { class: "df-clock", title: DF.t("clock") });
  var elapsedEl = DF.el("span", { class: "df-elapsed", title: DF.t("elapsed") });
  var remainEl = DF.el("span", { class: "df-remain", title: DF.t("remaining") });
  var pauseBtn = DF.el("button", { type: "button", onclick: togglePause });
  // target duration as minutes:seconds; the input is completed ("20" -> 20:00, "1:75" -> 02:15),
  // input that cannot be read brings back the value before
  var targetIn = DF.el("input", { type: "text", inputmode: "numeric", "data-df-target": "", placeholder: "mm:ss", size: "6", title: DF.t("target") });
  function targetSecs() {
    var p = cfg.presenter;
    return p.targetSeconds || (p.targetMinutes ? p.targetMinutes * 60 : 0); // targetMinutes: older settings
  }
  function showTarget() { var t = targetSecs(); targetIn.value = t ? DF.fmtMinSec(t) : ""; }
  function takeTarget() {
    var t = DF.parseMinSec(targetIn.value);
    if (t !== null) {
      cfg.presenter.targetSeconds = t;
      delete cfg.presenter.targetMinutes;
      DF.saveSettings(cfg);
    }
    showTarget(); tickClock();
  }
  targetIn.addEventListener("change", takeTarget);
  targetIn.addEventListener("keydown", function (e) { if (e.key === "Enter") { e.preventDefault(); takeTarget(); targetIn.blur(); } });
  showTarget();
  // the full-screen button names what a click does: "Full screen" in a window, "Window" in full screen
  var fsBtn = DF.el("button", { type: "button", "data-df-fs": "", onclick: function () { DF.toggleFs(); } });
  function fsLabel() { fsBtn.textContent = DF.t(document.fullscreenElement || document.webkitFullscreenElement ? "window" : "fullscreen"); }
  document.addEventListener("fullscreenchange", fsLabel);
  document.addEventListener("webkitfullscreenchange", fsLabel);
  fsLabel();
  // search and outline, with the notes: only the presenter sees this window. An entry goes to its
  // slide like a tile of the overview does.
  var finder = DF.finder({ notes: true, pick: function (i) { command("goto", i); }, current: function () { return shown().slide; } });
  var autoBtn = DF.el("button", { type: "button", "data-df-autobtn": "", onclick: function () { command("auto"); } });
  var autoInfo = DF.el("span", { class: "df-autoinfo" });
  var endBtn = DF.el("button", { type: "button", class: "df-danger", "data-df-end": "", text: DF.t("endShow"), onclick: function () { command("end"); } });
  var toggles = DF.el("span", { class: "df-toggles" }, ["current", "audience", "next", "notes", "time", "overview"].map(function (k) {
    return DF.el("button", { type: "button", "data-df-el": k, text: DF.t(k), onclick: function () { toggleElement(k); } });
  }));
  var bar = DF.el("header", { class: "df-bar" }, [
    DF.el("strong", { class: "df-title", text: meta.title || document.title || "" }), counterEl,
    DF.el("span", { class: "df-time-group", "data-k": "time" }, [clockEl, elapsedEl, remainEl, pauseBtn,
      DF.el("button", { type: "button", text: "\u21ba", title: DF.t("reset"), onclick: resetTimer }),
      DF.el("label", { class: "df-target" }, [DF.t("target") + " ", targetIn])]),
    DF.el("span", { class: "df-auto-group" }, [autoBtn, autoInfo]),
    DF.el("button", { type: "button", "data-df-font": "-", text: "A\u2212", title: DF.t("smaller"), onclick: function () { fontBy(-2); } }),
    DF.el("button", { type: "button", "data-df-font": "+", text: "A+", title: DF.t("larger"), onclick: function () { fontBy(2); } }),
    finder ? finder.button : null,
    fsBtn,
    endBtn
  ]);
  var cur = pane("current", DF.t("current"));
  // audience: what the audience sees right now: the slide at its step, black, white, the end slide;
  // empty only before the start
  var aud = pane("audience", DF.t("audience"));
  aud.querySelector(".df-stage").appendChild(DF.el("div", { class: "df-audcover" }, [DF.el("span", { text: DF.t("endOfShow") })]));
  var nxt = pane("next", DF.t("next"));
  var notesEl = DF.el("div", { class: "df-notes-text" });
  var notes = DF.el("section", { class: "df-pane df-notes", "data-k": "notes" }, [DF.el("h3", { text: DF.t("notes") }), notesEl]);
  var splitV = DF.el("div", { class: "df-split df-split-v", "data-df-split": "main" });
  var splitH = DF.el("div", { class: "df-split df-split-h", "data-df-split": "side" });
  var splitA = DF.el("div", { class: "df-split df-split-h", "data-df-split": "audience" });
  var left = DF.el("div", { class: "df-left" }, [cur, splitA, aud]);
  var side = DF.el("div", { class: "df-side" }, [nxt, splitH, notes]);
  var grid = DF.el("main", { class: "df-grid" }, [left, splitV, side]);
  // overview: a band of all slides above the status bar; a click jumps there (before the start it
  // chooses the start slide); the arrows choose the start slide before the start and only scroll
  // the band during the show, so that a click never moves the audience on by accident
  var strip = DF.slideStrip({
    pick: function (i) { command("goto", i); },
    step: function (d) { if (phase === "show") strip.scroll(d); else command(d < 0 ? "prev" : "next"); }
  });
  var overviewEl = DF.el("div", { class: "df-overview", "data-k": "overview" }, [strip.el]);
  var status = DF.el("footer", { class: "df-status" }, [bannerEl, toggles]);
  ui.appendChild(bar); ui.appendChild(grid); ui.appendChild(overviewEl); ui.appendChild(status);
  ui.style.setProperty("--df-ratio", (meta.width || 1920) + " / " + (meta.height || 1080));
  document.body.appendChild(ui);
  var frames = { current: DF.previewFrame(cur.querySelector(".df-stage"), snapshotHTML), audience: DF.previewFrame(aud.querySelector(".df-stage"), snapshotHTML),
    next: DF.previewFrame(nxt.querySelector(".df-stage"), snapshotHTML) };
  // narrow window: the panes stand one below the other and the middle scrolls (class df-narrow)
  function narrow() { ui.classList.toggle("df-narrow", ui.clientWidth < 760); }
  if ("ResizeObserver" in window) new ResizeObserver(narrow).observe(ui); else window.addEventListener("resize", narrow);
  narrow();
  function pane(k, title) {
    return DF.el("section", { class: "df-pane df-" + k, "data-k": k }, [DF.el("h3", { text: title }),
      DF.el("div", { class: "df-stage" }), DF.el("div", { class: "df-endlabel", text: DF.t("end") })]);
  }

  // ---------- settings while running ----------
  function toggleElement(k) {
    var els = cfg.presenter.elements.slice(), i = els.indexOf(k);
    if (i >= 0) els.splice(i, 1); else els.push(k);
    cfg.presenter.elements = els;
    DF.saveSettings(cfg); render();
  }
  function fontBy(d) {
    cfg.presenter.notesFontSize = Math.max(8, Math.min(96, cfg.presenter.notesFontSize + d));
    DF.saveSettings(cfg); render();
  }
  // panes: the dividers between the left column and the side column, between the current slide and
  // the audience, and between the next slide and the notes can be dragged; the shares are stored
  // in presenter.layout
  function layout() {
    var l = cfg.presenter.layout || {};
    return { main: l.main || 0.6, side: l.side || 0.45, audience: l.audience || 0.6 };
  }
  function drag(el, key) {
    el.addEventListener("pointerdown", function (e) {
      e.preventDefault();
      el.setPointerCapture(e.pointerId);
      var move = function (ev) {
        var r = (key === "main" ? grid : key === "audience" ? left : side).getBoundingClientRect();
        var f = key === "main" ? (ev.clientX - r.left) / r.width : (ev.clientY - r.top) / r.height;
        var l = layout();
        l[key] = Math.max(0.2, Math.min(0.8, f));
        cfg.presenter.layout = l;
        applyLayout();
      };
      var up = function () {
        el.removeEventListener("pointermove", move); el.removeEventListener("pointerup", up);
        DF.saveSettings(cfg);
      };
      el.addEventListener("pointermove", move); el.addEventListener("pointerup", up);
    });
  }
  drag(splitV, "main"); drag(splitH, "side"); drag(splitA, "audience");
  // the shares go into CSS variables; the class df-narrow lays the panes out one below the other
  // and ignores them (an inline grid template would override the narrow layout)
  function applyLayout() {
    var l = layout(), els = cfg.presenter.elements;
    var showCur = els.indexOf("current") >= 0, showAud = els.indexOf("audience") >= 0;
    var showNext = els.indexOf("next") >= 0, showNotes = els.indexOf("notes") >= 0 && hasNotes;
    var showLeft = showCur || showAud, showSide = showNext || showNotes;
    left.classList.toggle("df-hidden", !showLeft);
    side.classList.toggle("df-hidden", !showSide);
    splitV.classList.toggle("df-hidden", !(showLeft && showSide));
    splitA.classList.toggle("df-hidden", !(showCur && showAud));
    splitH.classList.toggle("df-hidden", !(showNext && showNotes));
    grid.style.setProperty("--df-cols", showLeft && showSide ? "minmax(0," + l.main + "fr) 8px minmax(0," + (1 - l.main) + "fr)" : "minmax(0,1fr)");
    cur.style.setProperty("--df-flex", showCur && showAud ? "0 0 calc(" + (l.audience * 100) + "% - 4px)" : "1 1 auto");
    nxt.style.setProperty("--df-flex", showNext && showNotes ? "0 0 calc(" + (l.side * 100) + "% - 4px)" : "1 1 auto");
  }
  var hasNotes = slides.some(function (s) { return s.notes; });

  // ---------- rendering ----------
  function shown() { return phase === "show" ? st : DF.model.goto(sel); }
  function render() {
    cfg = DF.settings();
    var els = cfg.presenter.elements;
    [].slice.call(ui.querySelectorAll("[data-k]")).forEach(function (n) { n.classList.toggle("df-hidden", els.indexOf(n.getAttribute("data-k")) < 0); });
    [].slice.call(toggles.querySelectorAll("[data-df-el]")).forEach(function (b) {
      var on = els.indexOf(b.getAttribute("data-df-el")) >= 0;
      b.classList.toggle("df-active", on); b.setAttribute("aria-pressed", on ? "true" : "false");
    });
    if (!hasNotes) notes.classList.add("df-hidden");
    applyLayout();
    var s = shown(), n = slides.length;
    counterEl.textContent = (s.slide + 1) + " / " + n +
      (phase === "show" && slides[s.slide] && slides[s.slide].steps > 1 ? "  \u00b7  " + DF.t("step") + " " + (s.step + 1) + "/" + slides[s.slide].steps : "");
    notesEl.textContent = (slides[s.slide] && slides[s.slide].notes) || DF.t("noNotes");
    notesEl.style.fontSize = cfg.presenter.notesFontSize + "px";
    // both previews show a whole slide after all its steps: the slide the audience sees and the
    // one after it
    frames.current.show(DF.model.end(s.slide));
    var as = audienceView();
    aud.setAttribute("data-df-audience", as);
    if (as === "slide") frames.audience.show({ slide: st.slide, step: st.step });
    var nx = s.slide + 1 < n ? DF.model.end(s.slide + 1) : null;
    nxt.classList.toggle("df-at-end", !nx);
    if (nx) frames.next.show(nx);
    autoBtn.textContent = remote.auto.on ? DF.t("autoOn") : DF.t("autoOff");
    autoBtn.classList.toggle("df-active", !!remote.auto.on);
    ui.classList.toggle("df-before", phase !== "show");
    if (s.slide !== marked) { marked = s.slide; strip.mark(s.slide); }
    strip.nav(phase === "show" || s.slide > 0, phase === "show" || s.slide < n - 1);
    banners();
  }
  // what the audience pane shows: empty before the start, then the end slide, black, white, or the
  // slide at the step the audience sees (also when that is the whole slide shown in "Current": an
  // empty pane during the show looked like a fault)
  function audienceView() {
    if (phase !== "show") return "empty";
    if (remote.ended) return "end";
    if (remote.blank === "black" || remote.blank === "white") return remote.blank;
    return slides[st.slide] ? "slide" : "empty";
  }
  function tickClock() {
    var d = new Date();
    clockEl.textContent = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
    var el = elapsed();
    elapsedEl.textContent = DF.fmtTime(el);
    pauseBtn.textContent = timer.paused ? "\u25b6" : "\u275a\u275a";
    pauseBtn.title = timer.paused ? DF.t("resume") : DF.t("pause");
    var target = targetSecs() * 1000;
    remainEl.style.display = target ? "" : "none";
    if (target) {
      var left = target - el;
      remainEl.textContent = DF.fmtTime(left);
      remainEl.className = "df-remain" + (left < 0 ? " df-over" : left < 120000 ? " df-warn" : "");
    }
    autoInfo.textContent = remote.auto.on ? (remote.auto.waiting ? "\u25b6 " + DF.t("media") : remote.auto.due ? Math.max(0, Math.ceil((remote.auto.due - Date.now()) / 1000)) + " s" : "") : "";
  }
  function elapsed() {
    if (!timer.start) return 0;
    return timer.accum + (timer.paused ? 0 : Date.now() - timer.start);
  }
  function togglePause() {
    if (!timer.start) return;
    if (timer.paused) { timer.start = Date.now(); timer.paused = false; }
    else { timer.accum = elapsed(); timer.paused = true; }
    tickClock();
  }
  function resetTimer() { timer = { start: phase === "show" ? Date.now() : 0, accum: 0, paused: false }; tickClock(); }

  // ---------- banner: lost connection, start hint ----------
  var flash = null, flashUntil = 0;
  function banners() {
    var key = null;
    if (Date.now() - lastBeat > 4000) key = "showLost";
    else if (flash && Date.now() < flashUntil) key = flash;
    bannerEl.textContent = key ? DF.t(key) : "";
    bannerEl.setAttribute("data-df-banner", key || "");
    bannerEl.classList.toggle("df-on", !!key);
  }
  function flashBanner(k) { flash = k; flashUntil = Date.now() + 4000; banners(); }

  var marked = -1;

  // ---------- commands and messages ----------
  var pending = [];
  function command(cmd, arg) {
    if (Date.now() - lastBeat > 1500) { pending.push([cmd, arg]); return; } // the show window reloads
    send({ type: "cmd", cmd: cmd, arg: arg });
  }
  function onMsg(d) {
    lastBeat = Date.now();
    if (pending.length) { var q = pending; pending = []; setTimeout(function () { q.forEach(function (c) { command(c[0], c[1]); }); }, 0); }
    if (d.type === "ended") { phase = "start"; render(); return; }
    var was = phase, before = JSON.stringify([phase, sel, st, remote.auto.on]);
    if (d.phase) phase = d.phase;
    if (typeof d.sel === "number") sel = d.sel;
    if (d.state) {
      st = { slide: d.state.slide, step: d.state.step };
      remote = { blank: d.blank, auto: d.auto || { on: false }, ended: d.ended };
    }
    if (was !== "show" && phase === "show") timer = { start: Date.now(), accum: 0, paused: false };
    if (phase !== "show") remote.auto = { on: false, due: 0 };
    if (d.type === "state" || d.type === "init" || JSON.stringify([phase, sel, st, remote.auto.on]) !== before) render();
  }
  function beat() { send({ type: "hb" }); banners(); }
  setInterval(beat, 1000);
  // the window of the start screen and the presentation is closed: this window goes with it. Once
  // that window is closed, window.opener is null in Chrome and WebKit (measured), not a closed
  // window; a reload of that window keeps it. A presenter view that had no opener from the start
  // stays.
  var hadShow = !!show;
  setInterval(function () {
    var o = null;
    try { o = window.opener; } catch (e) { o = null; }
    if (hadShow && (!o || o.closed)) window.close();
  }, 1000);
  setInterval(tickClock, 500);

  // ---------- keys ----------
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    if (a === "find") { if (finder) finder.open(); return true; }
    if (a === "fullscreen") { DF.toggleFs(); return true; }
    if (a === "timerReset") { resetTimer(); return true; }
    // Enter cannot start from here (full screen needs a click in the presentation window); it is
    // consumed, so that it does not press a focused button either
    if (a === "start" || (phase !== "show" && e && e.key === "Enter" && a !== "goto")) { if (phase !== "show") flashBanner("startInShow"); return true; }
    if (phase !== "show") {
      // before the start: choose the slide
      if (a === "end") return true;
    }
    if (a === "goto") { command("goto", e.slide); return true; }
    command(a);
    return true;
  }, null);

  // keep the keyboard focus out of the previews (the clicker would stop working)
  var refocus = function () {
    var a = document.activeElement;
    if (a && a.tagName === "IFRAME" && a.hasAttribute("data-df-preview")) { a.blur(); if (document.body.focus) document.body.focus(); }
  };
  window.addEventListener("blur", function () { setTimeout(refocus, 0); });
  setInterval(refocus, 500);

  render();
  tickClock();
  send({ type: "hello" });
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
  // A preview shows the end state of its slide without playing it: Web Animations (the claude.ai
  // player shows its builds with element.animate) are finished as soon as they are created, and
  // whatever still runs after a jump is finished too. finish() fires the usual finish events, so a
  // player that waits for them carries on. CSS animations and transitions get a duration of 0
  // (class df-still) in the claude.ai and Keynote players, which use no end events of their own.
  var origAnimate = Element.prototype.animate;
  if (origAnimate) Element.prototype.animate = function () {
    var an = origAnimate.apply(this, arguments);
    try { an.finish(); } catch (e) { } // an endless animation cannot finish and keeps running
    return an;
  };
  function finishAll() {
    if (!document.getAnimations) return;
    document.getAnimations().forEach(function (an) { try { an.finish(); } catch (e) { } });
  }
  if (DF.format() !== "pptx") document.documentElement.classList.add("df-still");
  var cur = { slide: 0, step: 0 }, queue = Promise.resolve(), ready;
  DF.previewReady = new Promise(function (res) { ready = res; });
  // setPreviewState returns a promise that settles once the state is shown
  DF.setPreviewState = function (s) {
    queue = queue.then(function () {
      if (DF.model.same(cur, s)) return;
      var from = cur;
      cur = { slide: s.slide, step: s.step };
      return DF.drive(from, cur).then(function () { finishAll(); return DF.wait(30); }).then(finishAll);
    }).then(null, function (err) { console.error("deckfold:", err); });
    return queue;
  };
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    document.documentElement.classList.add("df-preview");
    return DF.adapter().ready();
  }).then(function () { ready(); });
}

function bootShow() {
  // the start screen needs only styles and fonts; the source player runs after "Start"
  var snap = DF.snapshot();
  DF.activate({ cssOnly: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startShow(snap);
  });
}

function bootPresenter() {
  var snap = DF.snapshot();
  DF.activate({ runScripts: false }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startPresenter(snap);
  });
}

function boot() {
  try {
    if (DF.role === "preview") bootPreview();
    else if (DF.role === "presenter") bootPresenter();
    else bootShow();
  } catch (e) {
    console.error("deckfold:", e);
    document.documentElement.classList.remove("df-loading");
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
else boot();

})();
