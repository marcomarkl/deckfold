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
    DF.el("span", { text: (i + 1) + (s.label ? " · " + s.label : "") })]);
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
  var back = DF.el("button", { type: "button", class: "df-strip-nav", "data-df-strip-prev": "", "aria-label": DF.t("previous"), title: DF.t("previous"), text: "‹", onclick: function () { opts.step(-1); } });
  var fwd = DF.el("button", { type: "button", class: "df-strip-nav", "data-df-strip-next": "", "aria-label": DF.t("following"), title: DF.t("following"), text: "›", onclick: function () { opts.step(1); } });
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
var KEYNAMES = { ArrowRight: "→", ArrowLeft: "←", ArrowUp: "↑", ArrowDown: "↓", " ": "keySpace", PageDown: "keyPgDn", PageUp: "keyPgUp",
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
