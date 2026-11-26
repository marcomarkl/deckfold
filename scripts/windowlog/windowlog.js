// Window log for both windows of a deckfold file: the start screen or presentation ("SHOW") and
// the presenter view ("PRESENTER"). It logs the window position and size, the screen the browser
// reports, full screen, focus, visibility, keys (down and up), mouse clicks with their screen position, the calls
// of requestFullscreen, exitFullscreen, moveTo and Keyboard Lock, and the messages between the
// windows (without the heartbeat). Both windows write into localStorage, so the list in each window
// shows both, in time order, on the clock of the machine. A click on the list copies it. A new tab
// clears the logs of earlier tests; reloads of the show window keep them.
//
// It is put into a copy of a deck by scripts/windowlog (main.go), for faults in how the windows
// behave that only the maintainer can produce, such as a drag to another screen.
(function () {
  if (window.top !== window) return;
  var who = /^df-presenter:/.test(window.name) ? "PRESENTER" : "SHOW";
  var KEY = "dflog:" + who, all = [];
  // a fresh start in a new tab clears the logs of earlier tests; reloads of the show window keep them
  try {
    if (who === "SHOW" && !sessionStorage.getItem("dflog-started")) {
      localStorage.removeItem("dflog:SHOW"); localStorage.removeItem("dflog:PRESENTER");
      sessionStorage.setItem("dflog-started", "1");
    }
    all = JSON.parse(localStorage.getItem(KEY) || "[]");
  } catch (e) { }
  function store() { try { localStorage.setItem(KEY, JSON.stringify(all.slice(-600))); } catch (e) { } }
  function other() { try { return JSON.parse(localStorage.getItem("dflog:" + (who === "SHOW" ? "PRESENTER" : "SHOW")) || "[]"); } catch (e) { return []; } }
  function hms(t) { var d = new Date(t); return ("0" + d.getHours()).slice(-2) + ":" + ("0" + d.getMinutes()).slice(-2) + ":" + ("0" + d.getSeconds()).slice(-2) + "." + ("00" + d.getMilliseconds()).slice(-3); }
  function where() {
    var s = window.screen;
    return "win " + screenX + "," + screenY + " " + outerWidth + "x" + outerHeight +
      " | screen " + s.availLeft + "," + s.availTop + " " + s.width + "x" + s.height +
      " | fs " + (document.fullscreenElement ? "yes" : "no") + " | " + document.visibilityState + (document.hasFocus() ? " focus" : " nofocus");
  }
  var box = document.createElement("pre");
  box.setAttribute("data-df-ui", "");
  box.title = "Click: copy the log of both windows";
  box.style.cssText = (who === "SHOW" ? "position:fixed;left:55%;right:8px;top:8px;max-height:30vh;" : "position:fixed;left:30%;right:8px;bottom:48px;max-height:38vh;") + "overflow:auto;margin:0;padding:8px;background:#000d;color:#9f9;font:11px/1.35 Menlo,monospace;z-index:2147483647;white-space:pre-wrap;-webkit-user-select:text;user-select:text;cursor:copy";
  function text() {
    return all.concat(other()).sort(function (a, b) { return a[0] - b[0]; }).map(function (l) { return hms(l[0]) + " " + l[1] + " " + l[2]; }).join("\n");
  }
  function show() { box.textContent = text(); box.scrollTop = box.scrollHeight; }
  function log(what, at) {
    all.push([at || Date.now(), (who + "    ").slice(0, 9), what + " | " + where()]);
    store(); show();
  }
  window.__dflog = { text: text, log: log };
  window.addEventListener("storage", function (e) { if (e.key && e.key.indexOf("dflog:") === 0) show(); });
  box.addEventListener("click", function () {
    var t = text();
    (navigator.clipboard ? navigator.clipboard.writeText(t) : Promise.reject()).then(function () { log("log copied"); }, function () { log("copy failed: select the text and copy it by hand"); });
  });
  // events
  document.addEventListener("fullscreenchange", function () { log("fullscreenchange"); });
  document.addEventListener("fullscreenerror", function () { log("fullscreenerror"); });
  window.addEventListener("resize", function () { log("resize"); });
  window.addEventListener("focus", function () { log("focus"); });
  window.addEventListener("blur", function () { log("blur"); });
  document.addEventListener("visibilitychange", function () { log("visibilitychange"); });
  window.addEventListener("pagehide", function () { log("pagehide"); });
  // keys: recorded by the script before the runtime block (main.go), which sees them before the
  // runtime stops them; the ones that came before this script ran are logged with their own time
  var early = window.__dfkeys || [];
  window.__dfkeys = { push: function (k) { log(k[1], k[0]); } };
  early.forEach(function (k) { log(k[1], k[0]); });
  window.addEventListener("pointerdown", function (e) { log("pointerdown at screen " + e.screenX + "," + e.screenY + " on " + (e.target && e.target.getAttribute ? (e.target.closest("[data-df-fs]") ? "full-screen button" : e.target.tagName.toLowerCase()) : "?")); }, true);
  if (window.screen && "onchange" in window.screen) window.screen.addEventListener("change", function () { log("screen change"); });
  // position and screen, every 100 ms
  var last = "";
  setInterval(function () {
    var p = screenX + "," + screenY + " " + outerWidth + "x" + outerHeight + " " + window.screen.availLeft + "," + window.screen.availTop;
    if (p !== last) { if (last) log("moved or screen changed"); last = p; }
  }, 100);
  // calls
  var rf = Element.prototype.requestFullscreen;
  Element.prototype.requestFullscreen = function (o) {
    log("requestFullscreen" + (o && o.screen ? " on a screen" : "") + " (activation " + (navigator.userActivation ? navigator.userActivation.isActive : "?") + ")");
    var p = rf.call(this, o);
    p.then(function () { log("requestFullscreen resolved"); }, function (e) { log("requestFullscreen refused: " + (e && e.message)); });
    return p;
  };
  var ex = Document.prototype.exitFullscreen;
  Document.prototype.exitFullscreen = function () { log("exitFullscreen"); return ex.apply(this, arguments); };
  var mv = window.moveTo;
  window.moveTo = function (x, y) { log("moveTo(" + x + "," + y + ")"); return mv.apply(window, arguments); };
  var kb = navigator.keyboard;
  if (kb && kb.lock) {
    var lk = kb.lock.bind(kb), ul = kb.unlock.bind(kb);
    kb.lock = function (k) { log("keyboard.lock " + JSON.stringify(k)); return lk(k); };
    kb.unlock = function () { log("keyboard.unlock"); return ul(); };
  }
  // messages between the windows, without the heartbeat
  window.addEventListener("message", function (e) {
    var d = e.data;
    if (!d || d.df !== "deckfold" || d.type === "hb") return;
    log("message in: " + d.type + (d.cmd ? " " + d.cmd : "") + (d.phase ? " phase " + d.phase : ""));
  }, true);
  function put() { document.body.appendChild(box); log("loaded (" + location.pathname.split("/").pop() + ")"); }
  if (document.body) put(); else document.addEventListener("DOMContentLoaded", put);
})();
