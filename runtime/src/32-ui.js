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
// keys (as configured, e.g. "ArrowRight", " ", "f"), label, caps (fixed key caps instead of keys) }] }].
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
DF.keyHelp = function (groups) {
  return DF.el("div", { class: "df-keyhelp", "data-df-keyhelp": "" }, groups.map(function (g) {
    return DF.el("div", { class: "df-keygroup", "data-df-keygroup": g.name }, [DF.el("h4", { text: g.title })].concat(g.rows.map(function (r) {
      var caps = r.caps || [];
      if (!r.caps) (r.keys || []).forEach(function (k) { var c = keyCap(k); if (caps.indexOf(c) < 0) caps.push(c); });
      return DF.el("div", { class: "df-keyrow", "data-df-action": r.action, "data-df-keys": JSON.stringify(r.keys || []) }, [
        DF.el("span", { class: "df-keylabel", text: r.label }),
        DF.el("span", { class: "df-caps" }, caps.map(function (c) { return DF.el("kbd", { text: c }); }))]);
    })));
  }));
};
