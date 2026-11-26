// The show window during the presentation (after "Start"): owns the state and drives the source
// player; auto mode, black/white screen, full-screen fallback, wake lock and mouse cursor.
//
// opts: start (state to begin with), fullscreen (the start screen asked for full screen),
// send(msg) (to the presenter view), onEnd(lastState), snapshot (the document before activation,
// as text: the slide strip copies its tiles from it). Returns a promise that resolves once the first slide is shown.

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

  // ---------- slide strip (decision E22) ----------
  // A band of slide tiles with "End show" at the bottom of the presentation, for a show without a
  // keyboard: a swipe up or the key of "strip" shows it, a tile goes to its slide (a slide hidden from the show too, E21), a swipe down, a tap or click beside it, Escape and the key again close
  // it. It is built when it is first shown, as the last child of <body>, so that the slides of the
  // deck keep their places among the elements a query finds. While it is open, the keys of "next"
  // and "back" move its mark as on the start screen and Enter goes there; commands of the
  // presenter view still move the slides.
  var strip = { ui: null, band: null, open: false, sel: 0, source: null };
  function stripSource() {
    if (!strip.source && opts.snapshot) {
      try { strip.source = new DOMParser().parseFromString(opts.snapshot, "text/html"); } catch (e) { strip.source = null; }
    }
    return strip.source;
  }
  function stripSelect(i) {
    var n = DF.slides().length;
    strip.sel = Math.max(0, Math.min(n - 1, i));
    strip.band.mark(strip.sel);
    strip.band.nav(strip.sel > 0, strip.sel < n - 1);
  }
  function stripKeys(type, e) {
    e.stopImmediatePropagation();
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    var a = e.key === "Escape" ? "strip" : e.key === "Enter" ? "pick" : DF.actionFor(e);
    if (!a) return;
    e.preventDefault();
    if (type !== "keydown") return;
    switch (a) {
      case "strip": return closeStrip();
      case "pick": closeStrip(); return take("goto", strip.sel);
      case "next": return stripSelect(strip.sel + 1);
      case "prev": return stripSelect(strip.sel - 1);
      case "first": return stripSelect(0);
      case "last": return stripSelect(DF.slides().length - 1);
    }
  }
  function openStrip() {
    if (generic || strip.open) return;
    if (!strip.ui) {
      strip.band = DF.slideStrip({ dim: true, source: stripSource,
        pick: function (i) { closeStrip(); take("goto", i); }, step: function (d) { stripSelect(strip.sel + d); } });
      var endBtn = DF.iconButton("power", DF.t("endShow"), { class: "df-danger", "data-df-end": "", onclick: function () { take("end"); } });
      strip.ui = DF.el("div", { "data-df-ui": "", "data-df-showstrip": "", class: "df-showstrip" }, [strip.band.el, endBtn]);
    }
    document.body.appendChild(strip.ui);
    strip.open = true;
    strip.ui.classList.add("df-on");
    strip.band.hide(DF.hiddenSlides());
    DF.rescaleTiles(strip.band.band);
    stripSelect(st.slide);
    DF.modalKeys = stripKeys;
  }
  function closeStrip() {
    if (!strip.open) return;
    strip.open = false;
    strip.ui.classList.remove("df-on");
    if (DF.modalKeys === stripKeys) DF.modalKeys = null;
    var a = document.activeElement;
    if (a && strip.ui.contains(a) && a.blur) a.blur();
  }

  // ---------- full screen ----------
  // The start screen requested full screen in the click on "Start". If that failed, one click on
  // the overlay is enough. The overlay is for a request that got no full screen.
  // Full screen that was there and is left ends the presentation on a computer (decision E24): one
  // Escape leads back to the start screen in every browser. Safari takes the Escape itself and
  // tells the page only that full screen is gone, so that is the signal. In 20 runs in Safari 27.2
  // it came every time; a keyup of the Escape came in 15 of them and is not used (S2). With
  // Keyboard Lock (DF.escLock) the Escape comes as a key and ends the show by itself, so there the
  // rule is off, and full screen left by holding Escape goes on in the window. The key of
  // "fullscreen" leaves full screen without ending (fsSelf); the show then goes on in the window
  // with nothing over the slide, and the key brings full screen back.
  // On a phone or tablet (E23) the show always runs in full screen, and full screen that is lost
  // was not left on purpose: on an iPad every swipe down leaves it, and no page can take that
  // gesture (measured by the maintainer). There the overlay never shows; the next tap or swipe on
  // the slide asks for full screen again (regainFs) and moves on as usual.
  var mobile = DF.mobile();
  var fsWanted = !!opts.fullscreen, fsHad = !!document.fullscreenElement, fsSelf = false;
  // used by take(), which the full-screen check below can call before the keys are set up
  var started = false, held = [];
  function needFs() {
    return !document.fullscreenElement && !(window.innerHeight >= screen.height - 2 && window.innerWidth >= screen.width - 2);
  }
  function updateFsOverlay() {
    // fsSelf is dropped when full screen comes, not while it is there: Safari takes 0.6 s to leave
    // it (measured), and this check also runs once a second
    if (document.fullscreenElement) { if (!fsHad) fsSelf = false; fsHad = true; }
    else if (fsHad && !mobile) {
      fsHad = false; fsWanted = false;
      if (fsSelf) fsSelf = false;
      else if (!DF.escLock) { take("end"); return; }
    }
    fsEl.classList.toggle("df-on", !mobile && fsWanted && needFs());
  }
  function regainFs() {
    if (mobile && fsWanted && !document.fullscreenElement) DF.requestFs().catch(function () { });
  }
  // the wake lock before full screen: requestFullscreen uses the click up, and Safari grants the
  // wake lock only in one (see "wake lock" below)
  fsEl.addEventListener("click", function () { wake(); DF.requestFs().then(updateFsOverlay, updateFsOverlay); });
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

  // ---------- wake lock (decision E19) and mouse cursor ----------
  // The screen wake lock is held while the presentation runs. The first request is made in the
  // click or key on "Start" (opts.wakeLock), because Safari grants it only shortly after one; it is
  // requested again on every key, click and command while none is held (in take(): the key
  // listeners of 30-keys.js stop every event before a listener added here could see it), when
  // the page becomes visible, when the browser releases it, and every 10 s after a refusal (which
  // helps where no gesture is needed). A request that is still open is never joined by a second
  // one; open for more than 3 s, it counts as "off". wakeState() goes to the presenter view, which
  // shows a hint unless the lock is held.
  var lock = { sentinel: null, asked: 0, refused: 0 };
  function wake() {
    if (!navigator.wakeLock || lock.sentinel || lock.asked || document.visibilityState !== "visible") return;
    var p;
    try { p = navigator.wakeLock.request("screen"); } catch (e) { p = Promise.reject(e); }
    hold(p);
  }
  function hold(p) {
    lock.asked = Date.now();
    p.then(function (s) {
      lock.asked = 0; lock.refused = 0; lock.sentinel = s;
      s.addEventListener("release", function () {
        if (lock.sentinel !== s) return;
        lock.sentinel = null;
        wake();
      });
    }, function () { lock.asked = 0; lock.refused = Date.now(); });
  }
  function wakeState() {
    if (!navigator.wakeLock) return "none";
    if (lock.sentinel) return "held";
    return lock.asked && Date.now() - lock.asked < 3000 ? "pending" : "off";
  }
  document.addEventListener("visibilitychange", function () { if (document.visibilityState === "visible") wake(); });
  setInterval(function () { if (!lock.refused || Date.now() - lock.refused >= 10000) wake(); }, 1000);
  if (opts.wakeLock) hold(opts.wakeLock); else wake();
  var cursorTimer = 0;
  function showCursor() {
    document.documentElement.classList.remove("df-nocursor");
    clearTimeout(cursorTimer);
    cursorTimer = setTimeout(function () { document.documentElement.classList.add("df-nocursor"); }, 2000);
  }
  window.addEventListener("mousemove", showCursor, true);
  showCursor();

  // ---------- state ----------
  function info() { return { state: st, blank: blank, auto: { on: auto.on, due: auto.due, waiting: auto.waiting }, ended: !!st.ended, wake: wakeState() }; }
  function report() {
    var m = info();
    m.type = "state"; m.phase = "show";
    send(m);
    if (strip.open) stripSelect(st.slide);
    if (lastShown) {
      DF.ss.set(DF.deckKey("df-last"), lastShown);
      // also in the state of this history entry, which a reload keeps and which goes to the browser
      // with the navigation (see fromHash in 35-start.js); the address stays as the player wants it
      try { history.replaceState({ dfLast: lastShown.slide }, ""); } catch (e) { }
    }
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
  // to the start of its slide; "next" and "prev" mean slides here, not steps, and like "first" and
  // "last" they skip slides hidden from the show (E21). A link to a slide by its number shows it,
  // hidden or not.
  function follow(to) {
    if (st.ended) return;
    var n = DF.slides().length, j;
    switch (to) {
      case "next": j = DF.model.shownAfter(st.slide); return j >= 0 ? go(DF.model.goto(j)) : go(null);
      case "prev": j = DF.model.shownBefore(st.slide); return j >= 0 ? go(DF.model.goto(j)) : undefined;
      case "first": return go(DF.model.first());
      case "last": return go(DF.model.last());
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
  // once: the reload at the end leaves full screen too, and that must not end the show again
  var ending = false;
  function finish() {
    if (ending) return;
    ending = true;
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
        if (document.fullscreenElement) { fsSelf = true; return DF.toggleFs().catch(function () { fsSelf = false; }); }
        fsWanted = true;
        return DF.requestFs().then(updateFsOverlay, updateFsOverlay);
      case "strip": return strip.open ? closeStrip() : openStrip();
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
  function take(a, arg) {
    // a key or click in this window is the gesture Safari needs to grant the wake lock again (the
    // commands of the presenter view come through here too and give it none)
    wake();
    if (!started) { held.push([a, arg]); return; }
    return act(a, arg);
  }
  DF.installKeys(function (a, e) {
    if (a === "digits") return true;
    take(a, e && e.slide);
    return true; // the player never gets a bound key; Escape ends the show (in full screen it comes through Keyboard Lock, or the browser takes it and full screen ends, see above)
  }, generic ? { passthrough: true, forward: function () { } } : { swallowAll: true });
  // a click on the slide moves on, a swipe to the left too, one to the right goes back, one up
  // shows the slide strip (E22); the wheel does nothing (generic decks keep their own). While the
  // strip is open, a click or tap beside it closes it and moves nothing, and neither does a swipe.
  if (!generic) DF.installPointer({ next: function () { regainFs(); if (strip.open) closeStrip(); else take("next"); },
    interactive: adapter.interactive, guardClicks: !!adapter.guardClicks,
    link: adapter.link ? function (t, e) { return adapter.link(t, e, st); } : null,
    follow: function (to) { regainFs(); if (strip.open) closeStrip(); else take("follow", to); },
    swipe: function (dir) {
      // not on a swipe down, which is how an iPad leaves full screen
      if (dir !== "down") regainFs();
      if (dir === "up") { if (!strip.open) take("strip"); return; }
      if (dir === "down") { if (strip.open) take("strip"); return; }
      if (!strip.open) take(dir === "left" ? "next" : "prev");
    } });

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
