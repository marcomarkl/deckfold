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
