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
