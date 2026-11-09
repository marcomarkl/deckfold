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
  var autoBtn = DF.el("button", { type: "button", "data-df-autobtn": "", onclick: function () { command("auto"); } });
  var autoInfo = DF.el("span", { class: "df-autoinfo" });
  var endBtn = DF.el("button", { type: "button", class: "df-danger", "data-df-end": "", text: DF.t("endShow"), onclick: function () { command("end"); } });
  var toggles = DF.el("span", { class: "df-toggles" }, ["current", "next", "notes", "time", "overview"].map(function (k) {
    return DF.el("button", { type: "button", "data-df-el": k, text: DF.t(k), onclick: function () { toggleElement(k); } });
  }));
  var bar = DF.el("header", { class: "df-bar" }, [
    DF.el("strong", { class: "df-title", text: meta.title || document.title || "" }), counterEl,
    DF.el("span", { class: "df-time-group", "data-k": "time" }, [clockEl, elapsedEl, remainEl, pauseBtn,
      DF.el("button", { type: "button", text: "↺", title: DF.t("reset"), onclick: resetTimer }),
      DF.el("label", { class: "df-target" }, [DF.t("target") + " ", targetIn])]),
    DF.el("span", { class: "df-auto-group" }, [autoBtn, autoInfo]),
    DF.el("button", { type: "button", "data-df-font": "-", text: "A−", title: DF.t("smaller"), onclick: function () { fontBy(-2); } }),
    DF.el("button", { type: "button", "data-df-font": "+", text: "A+", title: DF.t("larger"), onclick: function () { fontBy(2); } }),
    fsBtn,
    endBtn
  ]);
  var cur = pane("current", DF.t("current"));
  var nxt = pane("next", DF.t("next"));
  var notesEl = DF.el("div", { class: "df-notes-text" });
  var notes = DF.el("section", { class: "df-pane df-notes", "data-k": "notes" }, [DF.el("h3", { text: DF.t("notes") }), notesEl]);
  var splitV = DF.el("div", { class: "df-split df-split-v", "data-df-split": "main" });
  var splitH = DF.el("div", { class: "df-split df-split-h", "data-df-split": "side" });
  var side = DF.el("div", { class: "df-side" }, [nxt, splitH, notes]);
  var grid = DF.el("main", { class: "df-grid" }, [cur, splitV, side]);
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
  document.body.appendChild(ui);
  var frames = { current: DF.previewFrame(cur.querySelector(".df-stage"), snapshotHTML), next: DF.previewFrame(nxt.querySelector(".df-stage"), snapshotHTML) };
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
  // panes: the divider between the current slide and the side column, and the one between the next
  // step and the notes, can be dragged; the shares are stored in presenter.layout
  function layout() { return cfg.presenter.layout || { main: 0.6, side: 0.45 }; }
  function drag(el, key) {
    el.addEventListener("pointerdown", function (e) {
      e.preventDefault();
      el.setPointerCapture(e.pointerId);
      var move = function (ev) {
        var r = (key === "main" ? grid : side).getBoundingClientRect();
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
  drag(splitV, "main"); drag(splitH, "side");
  function applyLayout() {
    var l = layout(), els = cfg.presenter.elements;
    var showCur = els.indexOf("current") >= 0, showNext = els.indexOf("next") >= 0, showNotes = els.indexOf("notes") >= 0 && hasNotes;
    var showSide = showNext || showNotes;
    splitV.classList.toggle("df-hidden", !(showCur && showSide));
    side.classList.toggle("df-hidden", !showSide);
    grid.style.gridTemplateColumns = showCur && showSide ? "minmax(0," + l.main + "fr) 8px minmax(0," + (1 - l.main) + "fr)" : "minmax(0,1fr)";
    splitH.classList.toggle("df-hidden", !(showNext && showNotes));
    nxt.style.flex = showNext && showNotes ? "0 0 calc(" + (l.side * 100) + "% - 4px)" : (showNext ? "1 1 auto" : "");
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
      (phase === "show" && slides[s.slide] && slides[s.slide].steps > 1 ? "  ·  " + DF.t("step") + " " + (s.step + 1) + "/" + slides[s.slide].steps : "");
    notesEl.textContent = (slides[s.slide] && slides[s.slide].notes) || DF.t("noNotes");
    notesEl.style.fontSize = cfg.presenter.notesFontSize + "px";
    // both previews show a whole slide after all its steps: the slide the audience sees and the
    // one after it
    frames.current.show(DF.model.end(s.slide));
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
  setInterval(tickClock, 500);

  // ---------- keys ----------
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    if (a === "fullscreen") { DF.toggleFs(); return true; }
    if (a === "timerReset") { resetTimer(); return true; }
    if (phase !== "show") {
      // before the start: choose the slide; Enter cannot start from here (full screen needs a click there)
      if (e && e.key === "Enter" && a !== "goto") { flashBanner("startInShow"); return true; }
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
