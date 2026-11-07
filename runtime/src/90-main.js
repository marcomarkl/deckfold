// Boot per role.
document.documentElement.classList.add("df-loading");

function bootPreview() {
  // Preview: muted, without its own keys; driven by the presenter view.
  // The preview must never take the keyboard focus (the clicker would stop working).
  HTMLElement.prototype.focus = function () { };
  window.focus = function () { };
  var origPlay = HTMLMediaElement.prototype.play;
  HTMLMediaElement.prototype.play = function () { this.muted = true; this.volume = 0; return origPlay.apply(this, arguments); };
  var cur = { slide: 0, step: 0 }, queue = Promise.resolve(), ready;
  DF.previewReady = new Promise(function (res) { ready = res; });
  DF.setPreviewState = function (s) {
    queue = queue.then(function () {
      if (DF.model.same(cur, s)) return;
      var from = cur;
      cur = { slide: s.slide, step: s.step };
      return DF.drive(from, cur).then(function () { return DF.wait(30); });
    });
  };
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    document.documentElement.classList.add("df-preview");
    return DF.adapter().ready();
  }).then(function () { ready(); });
}

function bootAudience() {
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startAudience();
  });
}

function bootStart() {
  var snap = DF.snapshot();
  DF.activate({ runScripts: false }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startPresenter(snap);
  });
}

function boot() {
  try {
    if (DF.role === "preview") bootPreview();
    else if (DF.role === "audience") bootAudience();
    else bootStart();
  } catch (e) {
    console.error("deckfold:", e);
    document.documentElement.classList.remove("df-loading");
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
else boot();
