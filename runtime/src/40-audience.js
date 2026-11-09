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
    if (lastShown) DF.ss.set("df-last", lastShown);
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
    st = { slide: target.slide, step: target.step };
    lastShown = st;
    driving = driving.then(function () { return (o && o.jump) ? adapter.jump(st, from) : DF.drive(from, st); });
    report();
    scheduleAuto();
  }
  function finish() {
    stopAuto();
    opts.onEnd(lastShown || st);
  }
  function act(a, arg) {
    switch (a) {
      // after the end slide, one more step returns to the start screen (as in PowerPoint)
      case "next": return st.ended ? finish() : go(DF.model.next(st));
      // on the first slide there is nothing before it: no end slide
      case "prev": if (st.ended) return go(st); var pv = DF.model.prev(st); return pv ? go(pv) : undefined;
      case "first": return go(DF.model.first());
      case "last": return go(DF.model.last());
      case "goto": return go(DF.model.goto(arg));
      case "state": return go(arg, { jump: true });
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
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    act(a, e && e.slide);
    return true; // the player never gets a bound key; Escape (in full screen through Keyboard Lock) ends the show
  }, generic ? { passthrough: true, forward: function () { } } : { swallowAll: true });

  DF.audience = {
    act: act,
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
    return driving;
  });
};
