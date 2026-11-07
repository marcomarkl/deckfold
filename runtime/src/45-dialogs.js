// Dialogs: settings (E4) and screen choice (E3).

function dialog(title, body, buttons) {
  var box = DF.el("div", { "data-df-ui": "", class: "df-dialog-backdrop" });
  var dlg = DF.el("div", { class: "df-dialog", role: "dialog", "aria-modal": "true", "aria-label": title }, [
    DF.el("h2", { text: title }), body,
    DF.el("div", { class: "df-dialog-buttons" }, buttons)
  ]);
  box.appendChild(dlg);
  document.body.appendChild(box);
  var first = dlg.querySelector("input,select,button");
  if (first) first.focus();
  return { close: function () { if (box.parentNode) box.parentNode.removeChild(box); }, root: dlg };
}

DF.openSettings = function (onSave) {
  var s = DF.settings();
  var generic = DF.format() === "generic";
  var checks = DF.el("fieldset", { class: "df-checks" }, [DF.el("legend", { text: DF.t("elements") })]);
  ["current", "next", "notes", "time", "overview"].forEach(function (k) {
    var id = "df-el-" + k;
    checks.appendChild(DF.el("label", { for: id }, [
      DF.el("input", { type: "checkbox", id: id, "data-k": k, checked: s.presenter.elements.indexOf(k) >= 0 }), " " + DF.t(k)
    ]));
  });
  function num(id, label, val, min, max, step) {
    return DF.el("label", { class: "df-row", for: id }, [DF.el("span", { text: label }),
      DF.el("input", { type: "number", id: id, value: String(val), min: String(min), max: String(max), step: String(step || 1) })]);
  }
  function chk(id, label, val) {
    return DF.el("label", { class: "df-row", for: id }, [DF.el("span", { text: label }), DF.el("input", { type: "checkbox", id: id, checked: !!val })]);
  }
  var body = DF.el("div", { class: "df-settings" }, generic ? [DF.el("p", { text: DF.t("generic") })] : [
    checks,
    num("df-notes-size", DF.t("notesSize"), s.presenter.notesFontSize, 8, 96),
    num("df-target", DF.t("target"), s.presenter.targetMinutes, 0, 600),
    num("df-interval", DF.t("autoInterval"), s.auto.intervalSeconds, 1, 3600, 0.5),
    chk("df-autostart", DF.t("autoStart"), s.auto.enabled),
    chk("df-loop", DF.t("autoLoop"), s.auto.loop),
    chk("df-remember", DF.t("remember") + " (" + DF.t("screens") + ")", s.screens.rememberChoice)
  ]);
  var d;
  function clamp(v, lo, hi, dflt) { v = parseFloat(v); return isFinite(v) ? Math.max(lo, Math.min(hi, v)) : dflt; }
  var save = DF.el("button", { type: "button", class: "df-primary", text: DF.t("save"), onclick: function () {
    if (!generic) {
      var r = d.root;
      s.presenter.elements = [].slice.call(r.querySelectorAll(".df-checks input")).filter(function (i) { return i.checked; }).map(function (i) { return i.getAttribute("data-k"); });
      s.presenter.notesFontSize = Math.round(clamp(r.querySelector("#df-notes-size").value, 8, 96, 22));
      s.presenter.targetMinutes = Math.round(clamp(r.querySelector("#df-target").value, 0, 600, 0));
      s.auto.intervalSeconds = clamp(r.querySelector("#df-interval").value, 1, 3600, 8);
      s.auto.enabled = r.querySelector("#df-autostart").checked;
      s.auto.loop = r.querySelector("#df-loop").checked;
      s.screens.rememberChoice = r.querySelector("#df-remember").checked;
      if (!s.screens.rememberChoice) DF.ls.del("deckfold:screens");
    }
    DF.saveSettings(s);
    d.close();
    if (onSave) onSave(DF.settings());
  } });
  var reset = DF.el("button", { type: "button", text: DF.t("reset"), onclick: function () {
    DF.saveSettings(DF.clone(DF.cfg.defaults));
    d.close();
    if (onSave) onSave(DF.settings());
  } });
  var cancel = DF.el("button", { type: "button", text: DF.t("cancel"), onclick: function () { d.close(); } });
  d = dialog(DF.t("settings"), body, [reset, cancel, save]);
  return d;
};

function screenKey(s) { return { label: s.label || "", w: s.width, h: s.height, l: s.left, t: s.top }; }
function matchScreen(list, k) {
  if (!k) return -1;
  for (var i = 0; i < list.length; i++) if (list[i].label === k.label && list[i].width === k.w && list[i].height === k.h) return i;
  for (var j = 0; j < list.length; j++) if (list[j].width === k.w && list[j].height === k.h && list[j].left === k.l) return j;
  return -1;
}

// chooseScreens(onStart): Chromium with the Window Management API shows a dialog; otherwise onStart(null).
// onStart(choice) must run synchronously inside the click (user gesture): choice = {audience, presenter, presenterFs, granted}.
DF.chooseScreens = function (onStart) {
  if (!("getScreenDetails" in window)) { onStart(null); return; }
  window.getScreenDetails().then(function (det) {
    DF.screenDetails = det;
    var list = det.screens;
    var saved = DF.settings().screens.rememberChoice ? DF.ls.get("deckfold:screens") : null;
    var ai = matchScreen(list, saved && saved.aud), pi = matchScreen(list, saved && saved.pre);
    if (ai < 0) ai = list.length > 1 ? list.indexOf(list.filter(function (s) { return s !== det.currentScreen; })[0]) : 0;
    if (pi < 0) pi = Math.max(0, list.indexOf(det.currentScreen));
    function sel(id, idx) {
      var s = DF.el("select", { id: id });
      list.forEach(function (sc, i) {
        var label = (sc.label || ("Screen " + (i + 1))) + " – " + sc.width + "×" + sc.height + (sc.isPrimary ? " ★" : "");
        s.appendChild(DF.el("option", { value: String(i), text: label, selected: i === idx }));
      });
      return s;
    }
    var fs = DF.el("input", { type: "checkbox", id: "df-prefs", checked: saved ? !!saved.fs : true });
    var hint = DF.el("p", { class: "df-hint", text: DF.t("presenterStays") });
    var body = DF.el("div", { class: "df-settings" }, [
      DF.el("label", { class: "df-row", for: "df-aud" }, [DF.el("span", { text: DF.t("audienceScreen") }), sel("df-aud", ai)]),
      DF.el("label", { class: "df-row", for: "df-pre" }, [DF.el("span", { text: DF.t("presenterScreen") }), sel("df-pre", pi)]),
      DF.el("label", { class: "df-row", for: "df-prefs" }, [DF.el("span", { text: DF.t("presenterFs") }), fs]),
      hint
    ]);
    var d;
    function upd() { hint.style.display = fs.checked ? "none" : ""; }
    fs.addEventListener("change", upd); upd();
    var startBtn = DF.el("button", { type: "button", class: "df-primary", text: DF.t("start"), onclick: function () {
      var a = list[parseInt(d.root.querySelector("#df-aud").value, 10)], p = list[parseInt(d.root.querySelector("#df-pre").value, 10)];
      if (DF.settings().screens.rememberChoice) DF.ls.set("deckfold:screens", { aud: screenKey(a), pre: screenKey(p), fs: fs.checked });
      d.close();
      onStart({ audience: a, presenter: p, presenterFs: fs.checked, granted: true });
    } });
    d = dialog(DF.t("screens"), body, [DF.el("button", { type: "button", text: DF.t("cancel"), onclick: function () { d.close(); } }), startBtn]);
  }, function () {
    // permission denied: default position
    var d = dialog(DF.t("screens"), DF.el("p", { text: DF.t("screensDenied") }), [
      DF.el("button", { type: "button", class: "df-primary", text: DF.t("start"), onclick: function () { d.close(); onStart(null); } })
    ]);
  });
};
