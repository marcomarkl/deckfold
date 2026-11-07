// Core: configuration, role, texts, storage, small helpers.
var DF = window.__deckfold = window.__deckfold || {};

function readJSON(id) {
  var el = document.getElementById(id);
  if (!el) return null;
  try { return JSON.parse(el.textContent); } catch (e) { console.error("deckfold: " + id + " nicht lesbar", e); return null; }
}

DF.cfg = readJSON("df-config") || { defaults: {} };
DF.version = DF.cfg.version;

// Role: "preview" (srcdoc iframe of the presenter view), "audience" (audience window),
// "start" (the window the file was opened in; it becomes the presenter view, or the solo
// audience view after a reload). The solo role is also kept in sessionStorage because WebKit
// drops window.name when a file:// page reloads.
function soloFlag() { try { return sessionStorage.getItem("df-solo"); } catch (e) { return null; } }
DF.role = (function () {
  try {
    if (window.frameElement && window.frameElement.hasAttribute("data-df-preview")) return "preview";
  } catch (e) { /* cross-origin */ }
  var n = window.name || "";
  if (n.indexOf("df-audience:") === 0 || n.indexOf("df-solo:") === 0 || soloFlag()) return "audience";
  return "start";
})();
DF.solo = DF.role === "audience" && ((window.name || "").indexOf("df-solo:") === 0 || (!!soloFlag() && (window.name || "").indexOf("df-audience:") !== 0));
DF.token = DF.role === "audience" ? ((window.name || "").split(":")[1] || soloFlag() || "") : "";
DF.setSolo = function (token) {
  try { if (token) sessionStorage.setItem("df-solo", token); else sessionStorage.removeItem("df-solo"); } catch (e) { }
  window.name = token ? "df-solo:" + token : "";
};

var LANG = String(navigator.language || "en").toLowerCase().indexOf("de") === 0 ? "de" : "en";
var TEXT = {
  de: {
    present: "Präsentieren", audienceOnly: "Nur Publikumsansicht", settings: "Einstellungen",
    slides: "Folien", generic: "Unbekanntes Format: ohne Referentenansicht, Übersicht und Auto-Modus.",
    screens: "Bildschirme", audienceScreen: "Publikum", presenterScreen: "Referent", presenterFs: "Referentenansicht im Vollbild",
    remember: "Auswahl merken", start: "Start", cancel: "Abbrechen", close: "Schließen", save: "Speichern",
    reset: "Zurücksetzen", screensHint: "Dieser Browser kann Fenster nicht selbst auf einen Bildschirm legen. Zieh das Publikumsfenster auf den Beamer und klick dort auf „Vollbild“.",
    screensDenied: "Kein Zugriff auf die Bildschirme. Das Publikumsfenster öffnet sich am Standardplatz; zieh es auf den Beamer.",
    presenterStays: "Die Referentenansicht bleibt, wo dieses Fenster liegt.",
    clickFs: "Klicken für Vollbild", fullscreen: "Vollbild", popupBlocked: "Das Publikumsfenster wurde blockiert.",
    openAudience: "Publikumsfenster öffnen", audienceLost: "Publikumsfenster nicht erreichbar.", reopen: "Neu öffnen",
    searching: "Suche Publikumsfenster …", screensChanged: "Die Bildschirme haben sich geändert.", chooseAgain: "Neu wählen",
    current: "Aktuell", next: "Als Nächstes", notes: "Notizen", time: "Zeit", overview: "Übersicht",
    end: "Ende", endShow: "Beenden", noNotes: "Keine Notizen", elapsed: "Verstrichen", remaining: "Rest",
    clock: "Uhr", pause: "Pause", resume: "Weiter", auto: "Auto", autoOn: "Auto an", autoOff: "Auto aus",
    elements: "Elemente der Referentenansicht", notesSize: "Schriftgröße Notizen (px)", target: "Zieldauer (Minuten, 0 = keine)",
    autoInterval: "Auto-Intervall (Sekunden)", autoLoop: "Auto-Modus in Schleife", autoStart: "Im Auto-Modus starten",
    endOfShow: "Ende der Präsentation", step: "Schritt", noPresenter: "Für dieses Format gibt es keine Referentenansicht. Tasten werden an das Publikumsfenster weitergegeben.",
    offline: "Live-Inhalt – ohne Netz nicht verfügbar", media: "Medien laufen",
    keysHelp: "Weiter: → ↓ Bild↓ Leertaste · Zurück: ← ↑ Bild↑ · Folie: Ziffern + Enter · B schwarz · W weiß · A Auto · F Vollbild"
  },
  en: {
    present: "Present", audienceOnly: "Audience view only", settings: "Settings",
    slides: "slides", generic: "Unknown format: no presenter view, overview or auto mode.",
    screens: "Screens", audienceScreen: "Audience", presenterScreen: "Presenter", presenterFs: "Presenter view in full screen",
    remember: "Remember choice", start: "Start", cancel: "Cancel", close: "Close", save: "Save",
    reset: "Reset", screensHint: "This browser cannot place windows on a screen. Drag the audience window to the projector and click “Full screen” there.",
    screensDenied: "No access to the screens. The audience window opens at the default position; drag it to the projector.",
    presenterStays: "The presenter view stays where this window is.",
    clickFs: "Click for full screen", fullscreen: "Full screen", popupBlocked: "The audience window was blocked.",
    openAudience: "Open audience window", audienceLost: "Audience window not reachable.", reopen: "Reopen",
    searching: "Looking for the audience window …", screensChanged: "The screen setup changed.", chooseAgain: "Choose again",
    current: "Current", next: "Next", notes: "Notes", time: "Time", overview: "Overview",
    end: "End", endShow: "End show", noNotes: "No notes", elapsed: "Elapsed", remaining: "Left",
    clock: "Clock", pause: "Pause", resume: "Resume", auto: "Auto", autoOn: "Auto on", autoOff: "Auto off",
    elements: "Presenter view elements", notesSize: "Notes font size (px)", target: "Target duration (minutes, 0 = none)",
    autoInterval: "Auto interval (seconds)", autoLoop: "Loop auto mode", autoStart: "Start in auto mode",
    endOfShow: "End of presentation", step: "Step", noPresenter: "This format has no presenter view. Keys are passed to the audience window.",
    offline: "Live content – not available offline", media: "media playing",
    keysHelp: "Next: → ↓ PgDn Space · Back: ← ↑ PgUp · Slide: digits + Enter · B black · W white · A auto · F full screen"
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
