// Core: configuration, role, texts, storage, small helpers.
var DF = window.__deckfold = window.__deckfold || {};

function readJSON(id) {
  var el = document.getElementById(id);
  if (!el) return null;
  try { return JSON.parse(el.textContent); } catch (e) { console.error("deckfold: " + id + " nicht lesbar", e); return null; }
}

DF.cfg = readJSON("df-config") || { defaults: {} };
DF.version = DF.cfg.version;

// Roles:
// "show": the window the file was opened in. It shows the start screen first and runs the source
//   player only after "Start"; it owns the state.
// "presenter": the presenter view, a popup the show window opens (window.name "df-presenter:<token>",
//   kept in sessionStorage too, because WebKit drops window.name when a file:// page reloads).
// "preview": srcdoc iframes in the start screen and the presenter view.
DF.role = (function () {
  try {
    if (window.frameElement && window.frameElement.hasAttribute("data-df-preview")) return "preview";
  } catch (e) { /* cross-origin */ }
  var n = window.name || "", saved = null;
  try { saved = sessionStorage.getItem("df-presenter"); } catch (e) { }
  if (n.indexOf("df-presenter:") === 0 || saved) return "presenter";
  return "show";
})();
// Token of the session; both windows use the same one. The show window keeps it across reloads.
DF.token = (function () {
  if (DF.role === "preview") return "";
  try {
    if (DF.role === "presenter") {
      var t = (window.name || "").indexOf("df-presenter:") === 0 ? window.name.split(":")[1] : sessionStorage.getItem("df-presenter");
      sessionStorage.setItem("df-presenter", t);
      return t;
    }
    var have = sessionStorage.getItem("df-token");
    if (have) return have;
  } catch (e) { }
  return null; // set by the show window once randomToken exists
})();

var LANG = String(navigator.language || "en").toLowerCase().indexOf("de") === 0 ? "de" : "en";
var TEXT = {
  de: {
    slides: "Folien", generic: "Unbekanntes Format: ohne Referentenansicht, Übersicht und Auto-Modus.",
    start: "Start", presenterView: "Referentenansicht",
    fullscreen: "Vollbild", window: "Fenster", auto: "Auto-Modus", autoInterval: "Intervall (s)", autoLoop: "Schleife",
    slide: "Folie", previous: "Zurück", following: "Weiter",
    popupBlocked: "Der Browser hat das Fenster der Referentenansicht blockiert. Erlaube Pop-ups für diese Datei und schalte die Referentenansicht noch einmal ein.",
    startInShow: "Gestartet wird im Präsentationsfenster: Klick dort auf „Start“.", showLost: "Das Präsentationsfenster ist nicht erreichbar.",
    loading: "Wird geladen …",
    clickFs: "Klicken für Vollbild",
    current: "Aktuell", audience: "Publikum", next: "Als Nächstes", notes: "Notizen", time: "Zeit", overview: "Übersicht",
    end: "Ende", endShow: "Beenden", noNotes: "Keine Notizen", elapsed: "Verstrichen", remaining: "Rest",
    clock: "Uhr", pause: "Pause", resume: "Weiter", autoOn: "Auto an", autoOff: "Auto aus", reset: "Zurücksetzen",
    target: "Ziel", smaller: "Notizen kleiner", larger: "Notizen größer",
    endOfShow: "Ende der Präsentation", step: "Schritt",
    offline: "Live-Inhalt – ohne Netz nicht verfügbar", media: "Medien laufen",
    keysStart: "Startbildschirm", keysSlides: "Folien", keysDisplay: "Anzeige", keysRun: "Ablauf",
    chooseSlide: "Folie wählen", firstSlide: "Erste Folie", lastSlide: "Letzte Folie", gotoSlide: "Folie n",
    black: "Schwarz", white: "Weiß", timerReset: "Zeit zurücksetzen", click: "Klick",
    keySpace: "Leertaste", keyPgDn: "Bild ↓", keyPgUp: "Bild ↑", keyHome: "Pos1", keyEnd: "Ende", keyBackspace: "Rückschritt", keyDigits: "1 2 3 …",
  },
  en: {
    slides: "slides", generic: "Unknown format: no presenter view, overview or auto mode.",
    start: "Start", presenterView: "Presenter view",
    fullscreen: "Full screen", window: "Window", auto: "Auto mode", autoInterval: "Interval (s)", autoLoop: "Loop",
    slide: "Slide", previous: "Back", following: "Forward",
    popupBlocked: "The browser blocked the presenter view window. Allow pop-ups for this file and switch the presenter view on again.",
    startInShow: "The presentation starts in the presentation window: click “Start” there.", showLost: "The presentation window is not reachable.",
    loading: "Loading …",
    clickFs: "Click for full screen",
    current: "Current", audience: "Audience", next: "Next", notes: "Notes", time: "Time", overview: "Overview",
    end: "End", endShow: "End show", noNotes: "No notes", elapsed: "Elapsed", remaining: "Left",
    clock: "Clock", pause: "Pause", resume: "Resume", autoOn: "Auto on", autoOff: "Auto off", reset: "Reset",
    target: "Target", smaller: "Smaller notes", larger: "Larger notes",
    endOfShow: "End of presentation", step: "Step",
    offline: "Live content – not available offline", media: "media playing",
    keysStart: "Start screen", keysSlides: "Slides", keysDisplay: "Display", keysRun: "Show",
    chooseSlide: "Choose slide", firstSlide: "First slide", lastSlide: "Last slide", gotoSlide: "Slide n",
    black: "Black", white: "White", timerReset: "Reset time", click: "Click",
    keySpace: "Space", keyPgDn: "Page Down", keyPgUp: "Page Up", keyHome: "Home", keyEnd: "End", keyBackspace: "Backspace", keyDigits: "1 2 3 …",
  }
};
function t(k) { return (TEXT[LANG] && TEXT[LANG][k]) || TEXT.en[k] || k; }
DF.t = t;

function el(tag, attrs, children) {
  var e = document.createElement(tag);
  if (attrs) for (var k in attrs) {
    if (k === "text") e.textContent = attrs[k];
    else if (k === "class") e.className = attrs[k];
    else if (k.indexOf("on") === 0 && typeof attrs[k] === "function") e.addEventListener(k.slice(2), attrs[k]);
    else if (attrs[k] !== false && attrs[k] != null) e.setAttribute(k, attrs[k] === true ? "" : attrs[k]);
  }
  (children || []).forEach(function (c) { if (c != null) e.appendChild(typeof c === "string" ? document.createTextNode(c) : c); });
  return e;
}
DF.el = el;

// Storage: localStorage for settings (per deck), sessionStorage for the running session.
function store(kind) {
  return {
    get: function (k) { try { var v = window[kind].getItem(k); return v == null ? null : JSON.parse(v); } catch (e) { return null; } },
    set: function (k, v) { try { window[kind].setItem(k, JSON.stringify(v)); } catch (e) { /* private mode or blocked */ } },
    del: function (k) { try { window[kind].removeItem(k); } catch (e) { } }
  };
}
DF.ls = store("localStorage");
DF.ss = store("sessionStorage");

function clone(o) { return JSON.parse(JSON.stringify(o)); }
function isObj(o) { return o && typeof o === "object" && !Array.isArray(o); }
function merge(base, over) {
  var out = clone(base);
  if (!isObj(over)) return out;
  for (var k in over) out[k] = isObj(over[k]) && isObj(out[k]) ? merge(out[k], over[k]) : clone(over[k]);
  return out;
}
// diff returns only the differences from base (for localStorage).
function diff(base, cur) {
  var out = {}, any = false;
  for (var k in cur) {
    if (isObj(cur[k]) && isObj(base[k])) {
      var d = diff(base[k], cur[k]);
      if (d) { out[k] = d; any = true; }
    } else if (JSON.stringify(cur[k]) !== JSON.stringify(base[k])) { out[k] = clone(cur[k]); any = true; }
  }
  return any ? out : null;
}
DF.merge = merge; DF.diff = diff; DF.clone = clone;

function randomToken() {
  var a = new Uint8Array(12);
  (window.crypto || window.msCrypto).getRandomValues(a);
  return Array.prototype.map.call(a, function (b) { return ("0" + b.toString(16)).slice(-2); }).join("");
}
DF.randomToken = randomToken;

function fmtTime(ms) {
  var neg = ms < 0; ms = Math.abs(ms);
  var s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60);
  s = s % 60;
  var p = function (n) { return (n < 10 ? "0" : "") + n; };
  return (neg ? "−" : "") + (h ? h + ":" + p(m) : p(m)) + ":" + p(s);
}
DF.fmtTime = fmtTime;

// Target duration typed as minutes:seconds. parseMinSec returns seconds (0 = no target), or null
// for input it cannot read: "20" is 20:00, "5:3" is 05:03, "1:75" is 02:15, ":30" is 00:30;
// at most 600:00.
DF.parseMinSec = function (v) {
  v = String(v == null ? "" : v).trim();
  if (v === "") return 0;
  var m = /^(\d*)(?::(\d*))?$/.exec(v);
  if (!m || (m[1] === "" && !m[2])) return null;
  return Math.min(parseInt(m[1] || "0", 10) * 60 + parseInt(m[2] || "0", 10), 36000);
};
DF.fmtMinSec = function (t) {
  var m = Math.floor(t / 60), s = t % 60;
  return (m < 10 ? "0" : "") + m + ":" + (s < 10 ? "0" : "") + s;
};

