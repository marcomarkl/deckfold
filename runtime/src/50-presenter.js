// Presenter view (role "presenter", E5 and E20): a window the show window opens. "Live", large,
// what the audience sees, and "Preview", what the next click shows, as muted srcdoc previews, with
// a dot for each state of the slide; notes, time, slide overview, the number of slides; its
// settings are changed here while it runs (panes shown, notes size, target duration, size of the
// panes and of the overview). It sends commands to the
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
  // the time of the show: accum is what ran before since, the moment it last started running (0
  // while it stands still: paused, before the first start, after the end of a show)
  var timer = { since: 0, accum: 0, paused: false };
  document.documentElement.classList.add("df-presenter-mode");

  // ---------- layout ----------
  var ui = DF.el("div", { "data-df-ui": "", class: "df-presenter" });
  var bannerEl = DF.el("div", { class: "df-banner", "data-df-banner": "" });
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
  // the full-screen button shows what a click does: corners pointing out and "Full screen" in a
  // window, corners pointing in and "Window" in full screen
  var fsBtn = DF.iconButton("fsEnter", DF.t("fullscreen"), { "data-df-fs": "", onclick: function () { DF.toggleFs(); } });
  function fsLabel() {
    var full = !!(document.fullscreenElement || document.webkitFullscreenElement), label = DF.t(full ? "window" : "fullscreen");
    fsBtn.replaceChild(DF.icon(full ? "fsExit" : "fsEnter"), fsBtn.firstChild);
    fsBtn.title = label;
    fsBtn.setAttribute("aria-label", label);
  }
  document.addEventListener("fullscreenchange", fsLabel);
  document.addEventListener("webkitfullscreenchange", fsLabel);
  fsLabel();
  // search and outline, with the notes: only the presenter sees this window. An entry goes to its
  // slide like a tile of the overview does.
  var finder = DF.finder({ notes: true, pick: function (i) { command("goto", i); }, current: function () { return shown().slide; } });
  var autoBtn = DF.el("button", { type: "button", "data-df-autobtn": "", onclick: function () { command("auto"); } });
  var autoInfo = DF.el("span", { class: "df-autoinfo" });
  var endBtn = DF.iconButton("power", DF.t("endShow"), { class: "df-danger", "data-df-end": "", onclick: function () { command("end"); } });
  // the panes that can be hidden, with their labels; time and overview are always shown (E20)
  var toggles = DF.el("span", { class: "df-toggles" }, [["audience", "live"], ["next", "upNext"], ["notes", "notes"]].map(function (t) {
    return DF.el("button", { type: "button", "data-df-el": t[0], text: DF.t(t[1]), onclick: function () { toggleElement(t[0]); } });
  }));
  // A- and A+ change the size of the notes; they are off while the notes are not shown
  var fontDown = DF.el("button", { type: "button", "data-df-font": "-", text: "A−", title: DF.t("smaller"), onclick: function () { fontBy(-2); } });
  var fontUp = DF.el("button", { type: "button", "data-df-font": "+", text: "A+", title: DF.t("larger"), onclick: function () { fontBy(2); } });
  var bar = DF.el("header", { class: "df-bar" }, [
    DF.el("strong", { class: "df-title", text: meta.title || document.title || "" }),
    DF.el("span", { class: "df-time-group" }, [clockEl, elapsedEl, remainEl, pauseBtn,
      DF.el("button", { type: "button", text: "↺", title: DF.t("reset"), onclick: resetTimer }),
      DF.el("label", { class: "df-target" }, [DF.t("target") + " ", targetIn])]),
    DF.el("span", { class: "df-auto-group" }, [autoBtn, autoInfo]),
    fontDown, fontUp,
    finder ? finder.button : null,
    fsBtn,
    endBtn
  ]);
  // audience ("Live"): what the audience sees right now: the slide at its step, black, white, the
  // end slide; before the start the slide the show will start on. Its heading has a dot for each
  // state of the slide and an arrow for the change to the next slide.
  var aud = pane("audience", DF.t("live"));
  aud.querySelector(".df-stage").appendChild(DF.el("div", { class: "df-audcover" }, [DF.el("span", { text: DF.t("endOfShow") })]));
  var stepsEl = DF.el("span", { class: "df-steps", "data-df-steps": "", role: "img" });
  aud.querySelector("h3").appendChild(stepsEl);
  // next ("Preview"): the state the next click shows, the next step of this slide or the next
  // slide; the arrow after the dots tells which of the two
  var nxt = pane("next", DF.t("upNext"));
  var notesEl = DF.el("div", { class: "df-notes-text" });
  var notes = DF.el("section", { class: "df-pane df-notes", "data-k": "notes" }, [DF.el("h3", { text: DF.t("notes") }), notesEl]);
  var splitV = DF.el("div", { class: "df-split df-split-v", "data-df-split": "main" });
  var splitH = DF.el("div", { class: "df-split df-split-h", "data-df-split": "side" });
  // the divider above the overview: dragging it up makes the tiles larger
  var splitS = DF.el("div", { class: "df-split df-split-h", "data-df-split": "strip" });
  var left = DF.el("div", { class: "df-left" }, [aud]);
  var side = DF.el("div", { class: "df-side" }, [nxt, splitH, notes]);
  var grid = DF.el("main", { class: "df-grid" }, [left, splitV, side]);
  // overview: a band of all slides above the status bar; a click jumps there (before the start it
  // chooses the start slide); the arrows choose the start slide before the start and only scroll
  // the band during the show, so that a click never moves the audience on by accident
  var strip = DF.slideStrip({
    pick: function (i) { command("goto", i); },
    step: function (d) { if (phase === "show") strip.scroll(d); else command(d < 0 ? "prev" : "next"); }
  });
  var overviewEl = DF.el("div", { class: "df-overview" }, [strip.el]);
  // the status bar: the number of slides at the left, the hints, the switches at the right
  var countEl = DF.el("span", { class: "df-count", "data-df-count": String(slides.length) });
  var status = DF.el("footer", { class: "df-status" }, [countEl, bannerEl, toggles]);
  ui.appendChild(bar); ui.appendChild(grid); ui.appendChild(splitS); ui.appendChild(overviewEl); ui.appendChild(status);
  ui.style.setProperty("--df-ratio", (meta.width || 1920) + " / " + (meta.height || 1080));
  document.body.appendChild(ui);
  var frames = { audience: DF.previewFrame(aud.querySelector(".df-stage"), snapshotHTML),
    next: DF.previewFrame(nxt.querySelector(".df-stage"), snapshotHTML) };
  // narrow window: the panes stand one below the other and the middle scrolls (class df-narrow)
  function narrow() { ui.classList.toggle("df-narrow", ui.clientWidth < 760); }
  // a lower window leaves less room for the overview: its tiles shrink with it (tileWidth)
  function resized() { narrow(); applyLayout(); }
  if ("ResizeObserver" in window) new ResizeObserver(resized).observe(ui); else window.addEventListener("resize", resized);
  narrow();
  function pane(k, title) {
    return DF.el("section", { class: "df-pane df-" + k, "data-k": k }, [DF.el("h3", {}, [DF.el("span", { class: "df-pane-title", text: title })]),
      DF.el("div", { class: "df-stage" }), DF.el("div", { class: "df-endlabel", text: DF.t("end") })]);
  }

  // ---------- settings while running ----------
  function elements() { return DF.presenterElements(cfg.presenter.elements); }
  function toggleElement(k) {
    var els = elements(), i = els.indexOf(k);
    if (i >= 0) els.splice(i, 1); else els.push(k);
    cfg.presenter.elements = els;
    DF.saveSettings(cfg); render();
  }
  function fontBy(d) {
    if (fontUp.disabled) return;
    cfg.presenter.notesFontSize = Math.max(8, Math.min(96, cfg.presenter.notesFontSize + d));
    DF.saveSettings(cfg); render();
  }
  // panes: the dividers between the left column and the side column, between the preview and the
  // notes, and above the overview can be dragged; the shares and the width of a tile of the
  // overview are stored in presenter.layout
  function layout() {
    var l = cfg.presenter.layout || {};
    return { main: l.main || 0.6, side: l.side || 0.45, strip: l.strip || 132 };
  }
  // tileWidth: the width of a tile of the overview, from 96 to 480 px (config.Layout.Strip) and
  // small enough that the overview takes at most 45 % of the window height
  function tileWidth(w) {
    var r = (meta.height || 1080) / (meta.width || 1920);
    var room = 0.45 * ui.clientHeight - (overviewEl.offsetHeight - (tileBox() || 0));
    var most = Math.max(96, Math.min(480, Math.floor(room / r)));
    return Math.max(96, Math.min(most, Math.round(w)));
  }
  // the height of the picture of a tile, 0 when no tile is laid out
  function tileBox() { var b = strip.band.querySelector(".df-thumb"); return b ? b.offsetHeight : 0; }
  function drag(el, key) {
    el.addEventListener("pointerdown", function (e) {
      e.preventDefault();
      el.setPointerCapture(e.pointerId);
      var move = function (ev) {
        var l = layout();
        if (key === "strip") {
          // the bottom of the overview stays where it is; the tile grows with the distance of the
          // pointer from it
          var o = overviewEl.getBoundingClientRect(), r0 = (meta.height || 1080) / (meta.width || 1920);
          var extra = overviewEl.offsetHeight - tileBox();
          l.strip = tileWidth((o.bottom - ev.clientY - extra) / r0);
        } else {
          var r = (key === "main" ? grid : side).getBoundingClientRect();
          var f = key === "main" ? (ev.clientX - r.left) / r.width : (ev.clientY - r.top) / r.height;
          l[key] = Math.max(0.2, Math.min(0.8, f));
        }
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
  drag(splitV, "main"); drag(splitH, "side"); drag(splitS, "strip");
  // the shares go into CSS variables; the class df-narrow lays the panes out one below the other
  // and ignores them (an inline grid template would override the narrow layout)
  function applyLayout() {
    var l = layout(), els = elements();
    var showLeft = els.indexOf("audience") >= 0;
    var showNext = els.indexOf("next") >= 0, showNotes = els.indexOf("notes") >= 0 && hasNotes;
    var showSide = showNext || showNotes;
    left.classList.toggle("df-hidden", !showLeft);
    side.classList.toggle("df-hidden", !showSide);
    splitV.classList.toggle("df-hidden", !(showLeft && showSide));
    splitH.classList.toggle("df-hidden", !(showNext && showNotes));
    grid.style.setProperty("--df-cols", showLeft && showSide ? "minmax(0," + l.main + "fr) 8px minmax(0," + (1 - l.main) + "fr)" : "minmax(0,1fr)");
    nxt.style.setProperty("--df-flex", showNext && showNotes ? "0 0 calc(" + (l.side * 100) + "% - 4px)" : "1 1 auto");
    var tw = tileWidth(l.strip);
    if (overviewEl.style.getPropertyValue("--df-tile") !== tw + "px") {
      overviewEl.style.setProperty("--df-tile", tw + "px");
      DF.rescaleTiles(strip.band);
    }
  }
  var hasNotes = slides.some(function (s) { return s.notes; });

  // ---------- rendering ----------
  // before the start: the slide the show will begin on, which is not a chosen slide hidden from it
  function shown() { return phase === "show" ? st : DF.model.startAt(sel); }
  // the slides hidden from the show (E21): the strip leaves them out, and the count says so. The
  // show window sends the list with every message, so a change on the start screen shows at once.
  var hiddenNow = null;
  function applyHidden(list) {
    var key = JSON.stringify(list || []);
    if (key === hiddenNow) return false;
    hiddenNow = key;
    DF.setHidden(list || []);
    strip.hide(DF.hiddenSlides());
    countEl.textContent = DF.slideCount(slides.length, slides.length - DF.hiddenSlides().length);
    marked = -1;
    return true;
  }
  function render() {
    cfg = DF.settings();
    var els = elements();
    [].slice.call(ui.querySelectorAll("[data-k]")).forEach(function (n) { n.classList.toggle("df-hidden", els.indexOf(n.getAttribute("data-k")) < 0); });
    [].slice.call(toggles.querySelectorAll("[data-df-el]")).forEach(function (b) {
      var on = els.indexOf(b.getAttribute("data-df-el")) >= 0;
      b.classList.toggle("df-active", on); b.setAttribute("aria-pressed", on ? "true" : "false");
    });
    if (!hasNotes) notes.classList.add("df-hidden");
    fontDown.disabled = fontUp.disabled = !(els.indexOf("notes") >= 0 && hasNotes);
    applyLayout();
    var s = shown(), n = slides.length;
    // the states the slide rests in, one dot each, so that auto steps of claude.ai slides count as
    // no click
    var stops = DF.model.stops(s.slide), done = stops.filter(function (p) { return p.step <= s.step; }).length;
    // the slide this window shows, for the tests: they wait for it before they read a preview
    ui.setAttribute("data-df-at", (s.slide + 1) + " / " + n);
    notesEl.textContent = (slides[s.slide] && slides[s.slide].notes) || DF.t("noNotes");
    notesEl.style.fontSize = cfg.presenter.notesFontSize + "px";
    // the left preview shows what the audience sees (before the start the start slide), the right
    // one the state of the next click (E20)
    var as = audienceView();
    aud.setAttribute("data-df-audience", as);
    if (as === "slide") frames.audience.show({ slide: st.slide, step: st.step });
    else if (as === "start") frames.audience.show(s);
    var ended = as === "end";
    var nx = ended ? null : DF.model.next(s);
    var leaves = !ended && (!nx || nx.slide !== s.slide);
    nxt.classList.toggle("df-at-end", !nx);
    if (nx) frames.next.show(nx);
    showSteps(ended ? null : stops, done, leaves);
    autoBtn.textContent = remote.auto.on ? DF.t("autoOn") : DF.t("autoOff");
    autoBtn.classList.toggle("df-active", !!remote.auto.on);
    ui.classList.toggle("df-before", phase !== "show");
    if (s.slide !== marked) { marked = s.slide; strip.mark(s.slide); }
    strip.nav(phase === "show" || DF.model.shownBefore(s.slide) >= 0, phase === "show" || DF.model.shownAfter(s.slide) >= 0);
    banners();
  }
  // dots in the heading of "Live": one for each state the slide rests in, filled up to the one
  // shown, then an arrow for the change to the next slide, marked when the next click makes it. Each
  // mark after the last filled dot is one click. From 13 states on, a bar stands for the dots.
  function showSteps(stops, done, leaves) {
    stepsEl.textContent = "";
    stepsEl.setAttribute("data-df-steps", stops ? done + "/" + stops.length : "");
    if (!stops || !stops.length) { stepsEl.removeAttribute("title"); stepsEl.removeAttribute("aria-label"); return; }
    if (stops.length > 12) {
      var fill = DF.el("span", { class: "df-stepfill" });
      fill.style.width = (100 * done / stops.length) + "%";
      stepsEl.appendChild(DF.el("span", { class: "df-stepbar" }, [fill]));
    } else {
      for (var i = 0; i < stops.length; i++) stepsEl.appendChild(DF.el("span", { class: "df-dot" + (i < done ? " df-done" : "") }));
    }
    stepsEl.appendChild(DF.el("span", { class: "df-arrow" + (leaves ? " df-next-is-slide" : "") }));
    var label = DF.t("stepOf").replace("{x}", done).replace("{y}", stops.length);
    stepsEl.title = label;
    stepsEl.setAttribute("aria-label", label);
  }
  // what the audience pane shows: the start slide before the start, then the end slide, black,
  // white, or the slide at the step the audience sees
  function audienceView() {
    if (phase !== "show") return "start";
    if (remote.ended) return "end";
    if (remote.blank === "black" || remote.blank === "white") return remote.blank;
    return slides[st.slide] ? "slide" : "empty";
  }
  function tickClock() {
    var d = new Date();
    clockEl.textContent = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
    var el = elapsed();
    elapsedEl.textContent = DF.fmtTime(el);
    pauseBtn.textContent = timer.paused ? "▶" : "❚❚";
    pauseBtn.title = timer.paused ? DF.t("resume") : DF.t("pause");
    var target = targetSecs() * 1000;
    remainEl.style.display = target ? "" : "none";
    if (target) {
      var left = target - el;
      remainEl.textContent = DF.fmtTime(left);
      remainEl.className = "df-remain" + (left < 0 ? " df-over" : left < 120000 ? " df-warn" : "");
    }
    autoInfo.textContent = remote.auto.on ? (remote.auto.waiting ? "▶ " + DF.t("media") : remote.auto.due ? Math.max(0, Math.ceil((remote.auto.due - Date.now()) / 1000)) + " s" : "") : "";
  }
  function elapsed() { return timer.accum + (timer.since ? Date.now() - timer.since : 0); }
  function togglePause() {
    if (phase !== "show") return;
    if (timer.paused) { timer.since = Date.now(); timer.paused = false; }
    else { timer.accum = elapsed(); timer.since = 0; timer.paused = true; }
    tickClock();
  }
  function resetTimer() { timer = { since: phase === "show" ? Date.now() : 0, accum: 0, paused: false }; tickClock(); }
  // a start lets the time run on from where the last show ended; the end of a show stops it there
  function runTimer() { timer.since = Date.now(); timer.paused = false; }
  function stopTimer() { timer.accum = elapsed(); timer.since = 0; timer.paused = false; tickClock(); }

  // ---------- banner: lost connection, start hint, no wake lock ----------
  // noWake: the presentation window holds no screen wake lock (refused, released, a request open
  // for more than 3 s, or no API), so the screen saver can start; the audience sees no hint
  var flash = null, flashUntil = 0;
  function banners() {
    var key = null;
    if (Date.now() - lastBeat > 4000) key = "showLost";
    else if (flash && Date.now() < flashUntil) key = flash;
    else if (phase === "show" && (remote.wake === "off" || remote.wake === "none")) key = "noWake";
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
    if (d.type === "ended") { if (phase === "show") stopTimer(); phase = "start"; render(); return; }
    var was = phase, before = JSON.stringify([phase, sel, st, remote.auto.on]);
    if (d.phase) phase = d.phase;
    if (typeof d.sel === "number") sel = d.sel;
    if (d.state) {
      st = { slide: d.state.slide, step: d.state.step };
      remote = { blank: d.blank, auto: d.auto || { on: false }, ended: d.ended, wake: d.wake };
    }
    if (was !== "show" && phase === "show") runTimer();
    if (was === "show" && phase !== "show") stopTimer();
    if (phase !== "show") remote.auto = { on: false, due: 0 };
    var hid = Array.isArray(d.hidden) && applyHidden(d.hidden);
    if (hid || d.type === "state" || d.type === "init" || JSON.stringify([phase, sel, st, remote.auto.on]) !== before) render();
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
      // before the start: choose the slide. "End" has no show to end here; in full screen it leaves
      // full screen, which Keyboard Lock (30-keys.js) keeps Escape from doing by itself
      if (a === "end") { if (document.fullscreenElement) document.exitFullscreen().catch(function () { }); return true; }
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

  // the list as stored until the show window sends its own
  applyHidden(DF.hiddenList());
  render();
  tickClock();
  send({ type: "hello" });
};
