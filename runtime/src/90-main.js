// Boot per role.
document.documentElement.classList.add("df-loading");

function bootPreview() {
  // Preview: muted, without its own keys; driven by the presenter view.
  // The preview must never take the keyboard focus (the clicker would stop working).
  HTMLElement.prototype.focus = function () { };
  window.focus = function () { };
  var origPlay = HTMLMediaElement.prototype.play;
  HTMLMediaElement.prototype.play = function () { this.muted = true; this.volume = 0; return origPlay.apply(this, arguments); };
  // A preview shows the end state of its slide without playing it: Web Animations (the claude.ai
  // player shows its builds with element.animate) are finished as soon as they are created, and
  // whatever still runs after a jump is finished too. finish() fires the usual finish events, so a
  // player that waits for them carries on. CSS animations and transitions get a duration of 0
  // (class df-still) in the claude.ai and Keynote players, which use no end events of their own.
  var origAnimate = Element.prototype.animate;
  if (origAnimate) Element.prototype.animate = function () {
    var an = origAnimate.apply(this, arguments);
    try { an.finish(); } catch (e) { } // an endless animation cannot finish and keeps running
    return an;
  };
  function finishAll() {
    if (!document.getAnimations) return;
    document.getAnimations().forEach(function (an) { try { an.finish(); } catch (e) { } });
  }
  if (DF.format() !== "pptx") document.documentElement.classList.add("df-still");
  var cur = { slide: 0, step: 0 }, queue = Promise.resolve(), ready;
  DF.previewReady = new Promise(function (res) { ready = res; });
  // setPreviewState returns a promise that settles once the state is shown
  DF.setPreviewState = function (s) {
    queue = queue.then(function () {
      if (DF.model.same(cur, s)) return;
      var from = cur;
      cur = { slide: s.slide, step: s.step };
      return DF.drive(from, cur).then(function () { finishAll(); return DF.wait(30); }).then(finishAll);
    }).then(null, function (err) { console.error("deckfold:", err); });
    return queue;
  };
  DF.activate({ runScripts: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    document.documentElement.classList.add("df-preview");
    return DF.adapter().ready();
  }).then(function () { ready(); });
}

function bootShow() {
  // the start screen needs only styles and fonts; the source player runs after "Start"
  var snap = DF.snapshot();
  DF.activate({ cssOnly: true }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startShow(snap);
  });
}

function bootPresenter() {
  var snap = DF.snapshot();
  DF.activate({ runScripts: false }).then(function () {
    document.documentElement.classList.remove("df-loading");
    DF.startPresenter(snap);
  });
}

function boot() {
  try {
    if (DF.role === "preview") bootPreview();
    else if (DF.role === "presenter") bootPresenter();
    else bootShow();
  } catch (e) {
    console.error("deckfold:", e);
    document.documentElement.classList.remove("df-loading");
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
else boot();
