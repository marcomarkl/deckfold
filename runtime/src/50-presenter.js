// Start screen and presenter view (E5): current slide and preview as muted srcdoc iframes,
// notes, time, slide overview; window management with heartbeat and recovery.

DF.startPresenter = function (snapshotHTML) {
  var generic = DF.format() === "generic";
  var slides = DF.slides();
  var meta = DF.meta.deck || {};
  var session = DF.ss.get("df-session");
  var popup = null, token = session ? session.token : "";
  var st = session && session.state ? session.state : DF.model.first();
  var remote = { blank: null, auto: { on: false, due: 0 }, ended: false };
  var lastBeat = 0, active = !!(session && session.active);
  var cfg = DF.settings();
  var timer = (session && session.timer) || { start: 0, accum: 0, paused: false };
  var send = DF.channel(function () { return popup; }, function () { return token; }, onMsg);

  function saveSession() { DF.ss.set("df-session", { token: token, state: st, active: active, timer: timer }); }

  // ---------- start screen ----------
  var start = DF.el("div", { "data-df-ui": "", class: "df-start" }, [
    DF.el("div", { class: "df-start-box" }, [
      DF.el("h1", { text: meta.title || document.title || "deckfold" }),
      DF.el("p", { class: "df-sub", text: generic ? DF.t("generic") : (slides.length + " " + DF.t("slides")) }),
      DF.el("div", { class: "df-start-buttons" }, [
        DF.el("button", { type: "button", class: "df-primary", text: DF.t("present"), onclick: present }),
        DF.el("button", { type: "button", text: DF.t("audienceOnly"), onclick: soloAudience }),
        DF.el("button", { type: "button", text: DF.t("settings"), onclick: function () { DF.openSettings(applySettings); } })
      ]),
      DF.el("p", { class: "df-keys", text: DF.t("keysHelp") }),
      DF.el("p", { class: "df-ver", text: "deckfold " + (DF.version || "") })
    ])
  ]);
  document.body.appendChild(start);

  function soloAudience() {
    DF.setSolo(DF.randomToken());
    DF.ss.del("df-audience-state");
    location.reload();
  }

  // ---------- present ----------
  function present() {
    DF.chooseScreens(function (choice) { launch(choice); });
  }
  function launch(choice) {
    if (!token) token = DF.randomToken();
    var url = location.href.split("#")[0];
    var feat = "popup";
    if (choice && choice.presenterFs) DF.requestFs(document.documentElement, choice.presenter).catch(function () { });
    if (choice && choice.audience) {
      var a = choice.audience;
      feat += ",left=" + a.availLeft + ",top=" + a.availTop + ",width=" + a.availWidth + ",height=" + a.availHeight;
      if (choice.granted) feat += ",fullscreen";
    } else {
      feat += ",width=1280,height=720";
    }
    DF.ss.del("df-audience-state");
    popup = window.open(url, "df-audience:" + token, feat);
    active = true;
    if (!timer.start) timer = { start: Date.now(), accum: 0, paused: false };
    saveSession();
    enterPresenter();
    if (!popup) banner("popupBlocked", "openAudience", function () { reopen(); });
    else if (!choice && !("getScreenDetails" in window)) banner("screensHint");
    lastBeat = Date.now();
  }
  function reopen() {
    var url = location.href.split("#")[0];
    DF.ss.del("df-audience-state");
    popup = window.open(url, "df-audience:" + token, "popup,width=1280,height=720");
    lastBeat = Date.now();
    hideBanner();
    if (!popup) banner("popupBlocked", "openAudience", reopen);
  }

  // ---------- presenter view ----------
  var ui = null, frames = {}, notesEl, counterEl, clockEl, elapsedEl, remainEl, autoBtn, autoInfo, pauseBtn, bannerEl, overviewEl;
  function enterPresenter() {
    start.style.display = "none";
    document.documentElement.classList.add("df-presenter-mode");
    if (ui) return render();
    ui = DF.el("div", { "data-df-ui": "", class: "df-presenter" });
    bannerEl = DF.el("div", { class: "df-banner" });
    counterEl = DF.el("span", { class: "df-counter" });
    clockEl = DF.el("span", { class: "df-clock", title: DF.t("clock") });
    elapsedEl = DF.el("span", { class: "df-elapsed", title: DF.t("elapsed") });
    remainEl = DF.el("span", { class: "df-remain", title: DF.t("remaining") });
    pauseBtn = DF.el("button", { type: "button", onclick: togglePause });
    autoBtn = DF.el("button", { type: "button", onclick: function () { command("auto"); } });
    autoInfo = DF.el("span", { class: "df-autoinfo" });
    var bar = DF.el("header", { class: "df-bar" }, [
      DF.el("strong", { class: "df-title", text: meta.title || document.title || "" }), counterEl,
      DF.el("span", { class: "df-time-group", "data-k": "time" }, [clockEl, elapsedEl, remainEl, pauseBtn,
        DF.el("button", { type: "button", text: "↺", title: DF.t("reset"), onclick: resetTimer })]),
      generic ? null : DF.el("span", { class: "df-auto-group" }, [autoBtn, autoInfo]),
      generic ? null : DF.el("button", { type: "button", "data-k": "overview", text: DF.t("overview"), onclick: toggleOverview }),
      DF.el("button", { type: "button", text: DF.t("settings"), onclick: function () { DF.openSettings(applySettings); } }),
      "getScreenDetails" in window ? DF.el("button", { type: "button", text: DF.t("screens"), onclick: function () { DF.chooseScreens(function (c) { relaunch(c); }); } }) : null,
      DF.el("button", { type: "button", class: "df-danger", text: DF.t("endShow"), onclick: endShow })
    ]);
    ui.appendChild(bar);
    ui.appendChild(bannerEl);
    if (generic) {
      ui.appendChild(DF.el("main", { class: "df-generic" }, [DF.el("p", { text: DF.t("noPresenter") })]));
    } else {
      var cur = pane("current", DF.t("current"));
      var nxt = pane("next", DF.t("next"));
      notesEl = DF.el("div", { class: "df-notes-text" });
      var notes = DF.el("section", { class: "df-pane df-notes", "data-k": "notes" }, [DF.el("h3", { text: DF.t("notes") }), notesEl]);
      ui.appendChild(DF.el("main", { class: "df-grid" }, [cur, DF.el("div", { class: "df-side" }, [nxt, notes])]));
      frames.current = makeFrame(cur.querySelector(".df-stage"));
      frames.next = makeFrame(nxt.querySelector(".df-stage"));
      overviewEl = DF.el("div", { class: "df-overview" });
      ui.appendChild(overviewEl);
    }
    document.body.appendChild(ui);
    if (DF.screenDetails) DF.screenDetails.addEventListener("screenschange", function () { banner("screensChanged", "chooseAgain", function () { DF.chooseScreens(relaunch); }); });
    render();
    setInterval(tickClock, 500);
    setInterval(watch, 1000);
    // take the focus back if it still ends up in a preview
    var refocus = function () {
      var a = document.activeElement;
      if (a && a.tagName === "IFRAME" && a.hasAttribute("data-df-preview")) { a.blur(); document.body.focus && document.body.focus(); }
    };
    window.addEventListener("blur", function () { setTimeout(refocus, 0); });
    setInterval(refocus, 500);
  }
  function pane(k, title) {
    return DF.el("section", { class: "df-pane df-" + k, "data-k": k }, [DF.el("h3", { text: title }),
      DF.el("div", { class: "df-stage" }), DF.el("div", { class: "df-endlabel", text: DF.t("end") })]);
  }
  function makeFrame(host) {
    var f = DF.el("iframe", { "data-df-preview": "", tabindex: "-1", inert: true, title: "preview", "aria-hidden": "true" });
    f.srcdoc = snapshotHTML;
    var shield = DF.el("div", { class: "df-shield" });
    host.appendChild(f);
    host.appendChild(shield);
    var rec = { el: f, want: null, ready: false };
    f.addEventListener("load", function () {
      var w = f.contentWindow;
      var waitReady = function () {
        if (w.__deckfold && w.__deckfold.previewReady) {
          w.__deckfold.previewReady.then(function () { rec.ready = true; push(rec); });
        } else setTimeout(waitReady, 50);
      };
      waitReady();
    });
    return rec;
  }
  function push(rec) {
    if (!rec.ready || !rec.want) return;
    try { rec.el.contentWindow.__deckfold.setPreviewState(rec.want); } catch (e) { }
  }
  function render() {
    if (!ui) return;
    cfg = DF.settings();
    var els = cfg.presenter.elements;
    [].slice.call(ui.querySelectorAll("[data-k]")).forEach(function (n) { n.classList.toggle("df-hidden", els.indexOf(n.getAttribute("data-k")) < 0); });
    ui.classList.toggle("df-no-next", els.indexOf("next") < 0);
    if (generic) return;
    var n = slides.length;
    counterEl.textContent = (st.slide + 1) + " / " + n + (slides[st.slide] && slides[st.slide].steps > 1 ? "  ·  " + DF.t("step") + " " + (st.step + 1) + "/" + slides[st.slide].steps : "");
    var hasNotes = slides.some(function (s) { return s.notes; });
    var notesPane = ui.querySelector(".df-notes");
    if (!hasNotes) notesPane.classList.add("df-hidden");
    var note = slides[st.slide] && slides[st.slide].notes;
    notesEl.textContent = note || DF.t("noNotes");
    notesEl.style.fontSize = cfg.presenter.notesFontSize + "px";
    frames.current.want = st; push(frames.current);
    var nx = DF.model.next(st);
    ui.querySelector(".df-next").classList.toggle("df-at-end", !nx);
    if (nx) { frames.next.want = nx; push(frames.next); }
    autoBtn.textContent = remote.auto.on ? DF.t("autoOn") : DF.t("autoOff");
    autoBtn.classList.toggle("df-active", !!remote.auto.on);
    if (overviewEl && overviewEl.classList.contains("df-on")) markOverview();
    saveSession();
  }
  function tickClock() {
    var d = new Date();
    clockEl.textContent = ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2);
    var el = elapsed();
    elapsedEl.textContent = DF.fmtTime(el);
    pauseBtn.textContent = timer.paused ? "▶" : "❚❚";
    pauseBtn.title = timer.paused ? DF.t("resume") : DF.t("pause");
    var target = cfg.presenter.targetMinutes * 60000;
    remainEl.style.display = target ? "" : "none";
    if (target) {
      var left = target - el;
      remainEl.textContent = DF.fmtTime(left);
      remainEl.className = "df-remain" + (left < 0 ? " df-over" : left < 120000 ? " df-warn" : "");
    }
    if (autoInfo) autoInfo.textContent = remote.auto.on ? (remote.auto.waiting ? "▶ " + DF.t("media") : remote.auto.due ? Math.max(0, Math.ceil((remote.auto.due - Date.now()) / 1000)) + " s" : "") : "";
  }
  function elapsed() {
    if (!timer.start) return 0;
    return timer.accum + (timer.paused ? 0 : Date.now() - timer.start);
  }
  function togglePause() {
    if (timer.paused) { timer.start = Date.now(); timer.paused = false; }
    else { timer.accum = elapsed(); timer.paused = true; }
    saveSession(); tickClock();
  }
  function resetTimer() { timer = { start: Date.now(), accum: 0, paused: false }; saveSession(); tickClock(); }

  // ---------- overview ----------
  function toggleOverview() {
    if (!overviewEl) return;
    var on = !overviewEl.classList.contains("df-on");
    overviewEl.classList.toggle("df-on", on);
    if (on && !overviewEl.firstChild) buildOverview();
    if (on) markOverview();
  }
  function buildOverview() {
    var srcSlides = document.querySelectorAll(meta.slideSelector || ".deck-slide");
    var w = meta.width || 1920, h = meta.height || 1080;
    slides.forEach(function (s, i) {
      var tile = DF.el("button", { type: "button", class: "df-tile", "data-i": String(i), onclick: function () { command("goto", i); toggleOverview(); } });
      var box = DF.el("div", { class: "df-thumb" });
      box.style.aspectRatio = w + " / " + h;
      if (s.thumb) {
        box.appendChild(DF.el("img", { src: DF.assets.url(s.thumb), alt: "" }));
      } else if (srcSlides[i]) {
        var canvas = srcSlides[i].querySelector(".deck-scaler > *") || srcSlides[i];
        var c = canvas.cloneNode(true);
        [].slice.call(c.querySelectorAll("script,iframe,video,audio")).forEach(function (x) { x.parentNode.removeChild(x); });
        c.removeAttribute("id");
        var holder = DF.el("div", { class: "df-clone" });
        holder.style.width = w + "px"; holder.style.height = h + "px";
        holder.appendChild(c);
        box.appendChild(holder);
      }
      tile.appendChild(box);
      tile.appendChild(DF.el("span", { text: (i + 1) + (s.label ? " · " + s.label : "") }));
      overviewEl.appendChild(tile);
    });
    scaleClones();
    window.addEventListener("resize", scaleClones);
  }
  function scaleClones() {
    var w = meta.width || 1920;
    [].slice.call(overviewEl.querySelectorAll(".df-clone")).forEach(function (c) {
      var bw = c.parentNode.clientWidth;
      c.style.transform = "scale(" + (bw / w) + ")";
    });
  }
  function markOverview() {
    [].slice.call(overviewEl.querySelectorAll(".df-tile")).forEach(function (t) { t.classList.toggle("df-cur", +t.getAttribute("data-i") === st.slide); });
  }

  // ---------- Banner ----------
  function banner(textKey, btnKey, fn) {
    bannerEl.innerHTML = "";
    bannerEl.appendChild(DF.el("span", { text: DF.t(textKey) }));
    if (btnKey) bannerEl.appendChild(DF.el("button", { type: "button", text: DF.t(btnKey), onclick: fn }));
    bannerEl.classList.add("df-on");
  }
  function hideBanner() { bannerEl.classList.remove("df-on"); }

  // ---------- commands ----------
  var pending = [], bootAt = Date.now();
  function command(cmd, arg) {
    if (popup && !popup.closed && send({ type: "cmd", cmd: cmd, arg: arg })) return;
    if (active && !popup && Date.now() - bootAt < 5000) { pending.push([cmd, arg]); return; } // wait for the reconnect
    // without an audience window: advance locally (rehearsal)
    if (generic) return;
    var nx = null;
    switch (cmd) {
      case "next": nx = DF.model.next(st); break;
      case "prev": nx = DF.model.prev(st); break;
      case "first": nx = DF.model.first(); break;
      case "last": nx = DF.model.last(); break;
      case "goto": nx = DF.model.goto(arg); break;
    }
    if (nx) { st = nx; render(); }
  }
  function onMsg(d, source) {
    if (!popup && source) {
      popup = source; // after a reload of the presenter view
      var q = pending; pending = [];
      setTimeout(function () { q.forEach(function (c) { command(c[0], c[1]); }); }, 0);
    }
    lastBeat = Date.now();
    hideBannerIfLost();
    if (d.type === "hello") {
      // new or reloaded audience window: send the session state and the auto settings
      send({ type: "init", state: generic ? null : st, settings: { auto: cfg.auto } });
      return;
    }
    if (d.type === "state" || d.type === "hb") {
      if (d.state && !generic) {
        var changed = !DF.model.same(d.state, st) || remote.auto.on !== (d.auto && d.auto.on);
        st = { slide: d.state.slide, step: d.state.step };
        remote = { blank: d.blank, auto: d.auto || { on: false }, ended: d.ended };
        if (changed || d.type === "state") render();
      }
      return;
    }
    if (d.type === "action" && d.action === "timerReset") resetTimer();
    if (d.type === "ended" && !d.reload) { popup = null; leave(); }
  }
  function hideBannerIfLost() { if (bannerEl && bannerEl.getAttribute("data-lost")) { bannerEl.removeAttribute("data-lost"); hideBanner(); } }
  function watch() {
    if (!active) return;
    var lost = (popup && popup.closed) || (!popup && Date.now() - lastBeat > 3000) || (popup && Date.now() - lastBeat > 4000);
    if (lost && bannerEl && !bannerEl.getAttribute("data-lost")) {
      banner(popup ? "audienceLost" : "searching", "reopen", reopen);
      bannerEl.setAttribute("data-lost", "1");
    }
  }
  function relaunch(choice) {
    if (popup && !popup.closed) { try { popup.close(); } catch (e) { } }
    popup = null;
    launch(choice);
  }
  function endShow() {
    send({ type: "end" });
    if (popup && !popup.closed) { try { popup.close(); } catch (e) { } }
    leave();
  }
  function leave() {
    active = false; popup = null;
    DF.ss.del("df-session"); DF.ss.del("df-audience-state");
    token = ""; timer = { start: 0, accum: 0, paused: false };
    if (document.fullscreenElement) document.exitFullscreen().catch(function () { });
    document.documentElement.classList.remove("df-presenter-mode");
    if (ui) ui.style.display = "none";
    start.style.display = "";
    location.reload();
  }
  function applySettings(s) {
    cfg = s;
    send({ type: "settings", settings: { auto: s.auto } });
    render();
  }

  // ---------- keys in the presenter view ----------
  DF.installKeys(function (a, e) {
    if (!active) {
      if (a === "fullscreen") { DF.toggleFs(); return true; }
      return false;
    }
    switch (a) {
      case "digits": return true;
      case "timerReset": resetTimer(); return true;
      case "fullscreen": DF.toggleFs(); return true;
      case "end": if (!document.fullscreenElement) endShow(); return true;
      case "goto": command("goto", e.slide); return true;
      default: command(a); return true;
    }
  }, generic ? { passthrough: true, forward: function (e) {
    if (active && popup && !popup.closed) {
      send({ type: "key", key: e.key, code: e.code, keyCode: e.keyCode, shift: e.shiftKey });
      e.preventDefault();
    }
  } } : null);

  // recovery after a reload of the presenter view
  if (active) {
    enterPresenter();
    lastBeat = Date.now();
  }
};
