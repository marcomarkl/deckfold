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
