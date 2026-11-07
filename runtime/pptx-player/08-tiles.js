// Tile transitions drawn with three.js (glitter, honeycomb, vortex, shred, ripple, fracture,
// crush, curtains, wind, prestige, airplane, origami).
//
// Both slides are drawn into textures: each slide's SVG is copied with its pictures and fonts as
// data URLs, so that it can be drawn as a standalone image. The old slide is cut into tiles that
// show the new slide on their back; each transition moves the tiles in its own way in front of
// the new slide. Without WebGL, or when a slide cannot be drawn, the caller fades instead.

var Tiles = (function () {
  var T = null, renderer = null, failed = false;
  var dataURLs = {}; // blob URL -> data URL

  function ready() {
    if (failed) return false;
    T = window.__dfThree;
    if (!T) { failed = true; return false; }
    if (renderer) return true;
    try {
      renderer = new T.WebGLRenderer({ alpha: true, antialias: true });
      renderer.outputColorSpace = T.SRGBColorSpace;
    } catch (e) { failed = true; return false; }
    return true;
  }

  function toDataURL(url) {
    if (!/^blob:/.test(url)) return Promise.resolve(url);
    if (dataURLs[url]) return Promise.resolve(dataURLs[url]);
    return fetch(url).then(function (r) { return r.blob(); }).then(function (b) {
      return new Promise(function (ok, fail) {
        var fr = new FileReader();
        fr.onload = function () { dataURLs[url] = fr.result; ok(fr.result); };
        fr.onerror = fail;
        fr.readAsDataURL(b);
      });
    });
  }

  // fontCSS returns the @font-face rules of the deck with data URLs.
  var fontCSS = null;
  function fonts() {
    if (fontCSS) return fontCSS;
    var rules = [];
    [].forEach.call(document.styleSheets, function (sh) {
      var list;
      try { list = sh.cssRules; } catch (e) { return; }
      [].forEach.call(list || [], function (r) { if (r.type === 5) rules.push(r.cssText); });
    });
    fontCSS = Promise.all(rules.map(function (css) {
      var m = /url\("?([^")]+)"?\)/.exec(css);
      return m ? toDataURL(m[1]).then(function (d) { return css.replace(m[1], d); }, function () { return ""; }) : Promise.resolve(css);
    })).then(function (list) { return list.join("\n"); });
    return fontCSS;
  }

  // snapshot draws the current state of a slide into a canvas.
  function snapshot(section, W, H, scale) {
    var svg = section.querySelector("svg");
    if (!svg) return Promise.reject(new Error("no slide"));
    var copy = svg.cloneNode(true);
    [].forEach.call(copy.querySelectorAll("foreignObject"), function (f) { f.parentNode.removeChild(f); });
    var imgs = [].slice.call(copy.querySelectorAll("image"));
    return Promise.all(imgs.map(function (im) {
      var h = im.getAttribute("href");
      return h ? toDataURL(h).then(function (d) { im.setAttribute("href", d); }, function () { }) : null;
    })).then(fonts).then(function (css) {
      var st = document.createElementNS("http://www.w3.org/2000/svg", "style");
      st.textContent = css;
      copy.insertBefore(st, copy.firstChild);
      copy.setAttribute("width", W); copy.setAttribute("height", H);
      var src = "data:image/svg+xml;charset=utf-8," + encodeURIComponent(new XMLSerializer().serializeToString(copy));
      return new Promise(function (ok, fail) {
        var im = new Image();
        im.onload = function () {
          var c = document.createElement("canvas");
          c.width = Math.round(W * scale); c.height = Math.round(H * scale);
          var g = c.getContext("2d");
          var bg = getComputedStyle(section).backgroundColor;
          g.fillStyle = bg && bg !== "rgba(0, 0, 0, 0)" ? bg : "#fff";
          g.fillRect(0, 0, c.width, c.height);
          g.drawImage(im, 0, 0, c.width, c.height);
          ok(c);
        };
        im.onerror = function () { fail(new Error("slide could not be drawn")); };
        im.src = src;
      });
    });
  }

  function rnd(i) { var x = Math.sin(i * 12.9898 + 78.233) * 43758.5453; return x - Math.floor(x); }
  function clamp(v) { return v < 0 ? 0 : v > 1 ? 1 : v; }
  function ease(v) { v = clamp(v); return v * v * (3 - 2 * v); }

  // grid returns the tile layout of a transition: columns and rows.
  function grid(type) {
    switch (type) {
      case "shred": case "curtains": return [24, 1];
      case "prestige": return [12, 1];
      default: return [16, 9];
    }
  }

  // place sets tile k (center x, y in slide px, size w, h) at progress p of the transition.
  function place(type, dir, m, t, p, W, H) {
    var u = t.cx / W, v = t.cy / H, r = rnd(t.k), q;
    var back = function () { return dir === "l" || dir === "u" ? 1 - u : u; };
    m.position.set(t.cx - W / 2, H / 2 - t.cy, 0);
    m.rotation.set(0, 0, 0);
    m.scale.set(1, 1, 1);
    switch (type) {
      case "glitter":
        q = ease((p - back() * 0.6 - r * 0.15) / 0.25);
        m.rotation.y = Math.PI * q;
        break;
      case "prestige":
        q = ease((p - back() * 0.6) / 0.4);
        m.rotation.x = Math.PI * q;
        m.position.z = Math.sin(Math.PI * q) * 80;
        break;
      case "ripple":
        q = ease((p - Math.hypot(u - 0.5, v - 0.5) * 1.2) / 0.3);
        m.rotation.y = Math.PI * q;
        m.position.z = Math.sin(Math.PI * q) * 60;
        break;
      case "honeycomb":
        q = ease((p - r * 0.5) / 0.5);
        m.position.z = q * 900;
        m.rotation.z = q * (r - 0.5) * 3;
        m.scale.setScalar(1 - 0.5 * q);
        m.visible = q < 0.98;
        break;
      case "vortex":
        q = ease((p - back() * 0.4 - r * 0.2) / 0.4);
        var a = q * Math.PI * 3, rad = q * Math.max(W, H);
        m.position.x += Math.cos(a + r * 6) * rad;
        m.position.y += Math.sin(a + r * 6) * rad;
        m.position.z = q * 300;
        m.rotation.z = a;
        break;
      case "shred":
        q = ease((p - r * 0.3) / 0.7);
        m.position.y -= q * H * 1.3 * (t.k % 2 ? 1 : 1.2);
        m.rotation.x = q * (t.k % 2 ? 1 : -1);
        break;
      case "fracture": case "crush":
        q = ease((p - r * 0.4) / 0.6);
        if (type === "crush") { m.scale.setScalar(1 - q); m.position.x *= 1 - q; m.position.y *= 1 - q; }
        else { m.position.y -= q * q * H * 1.5; m.rotation.z = q * (r - 0.5) * 4; m.rotation.x = q * r * 3; }
        break;
      case "curtains":
        q = ease(p);
        var side = u < 0.5 ? -1 : 1;
        m.position.x += side * q * W * 0.6 * (1 - Math.abs(u - 0.5));
        m.rotation.y = side * q * 1.2;
        m.scale.y = 1 + q * 0.05;
        break;
      case "wind":
        q = ease((p - back() * 0.5 - r * 0.2) / 0.5);
        var s = dir === "l" ? -1 : 1;
        m.position.x += s * q * W * 1.2;
        m.position.y += q * (r - 0.3) * H * 0.4;
        m.rotation.z = s * q * (r + 0.5);
        break;
      default: // airplane, origami
        q = ease((p - back() * 0.5) / 0.5);
        m.rotation.x = q * 1.5;
        m.rotation.y = q * (type === "airplane" ? 0.8 : -0.8);
        m.position.x += q * W * 0.8;
        m.position.y += q * H * 0.8;
        m.scale.setScalar(1 - 0.6 * q);
        break;
    }
  }

  // start plays a tile transition over the stage; returns false when it cannot be drawn, so
  // that the caller fades instead. done runs at the end.
  function start(stage, a, b, tr, W, H, done) {
    if (!ready()) return null;
    var scale = Math.min(2, Math.max(1, window.devicePixelRatio || 1));
    var cancelled = false, raf = 0, canvas = null, cleanup = function () { };
    var finish = function () {
      if (cancelled) return;
      cancelled = true;
      cancelAnimationFrame(raf);
      cleanup();
      done();
    };
    Promise.all([snapshot(a, W, H, scale), snapshot(b, W, H, scale)]).then(function (c) {
      if (cancelled) return;
      var ta = new T.CanvasTexture(c[0]), tb = new T.CanvasTexture(c[1]);
      ta.colorSpace = tb.colorSpace = T.SRGBColorSpace;
      var scene = new T.Scene();
      var cam = new T.PerspectiveCamera(30, W / H, 1, 20000);
      cam.position.set(0, 0, H / 2 / Math.tan(15 * Math.PI / 180));
      // the new slide lies behind all tiles, also tiles that turn away behind its plane
      var full = new T.Mesh(new T.PlaneGeometry(W, H), new T.MeshBasicMaterial({ map: tb, depthWrite: false, depthTest: false }));
      full.renderOrder = -1;
      var flips = /^(glitter|prestige|ripple)$/.test(tr.type);
      if (!flips) scene.add(full);
      var g = grid(tr.type), tw = W / g[0], th = H / g[1], tiles = [];
      var matA = new T.MeshBasicMaterial({ map: ta, side: T.FrontSide });
      var matB = new T.MeshBasicMaterial({ map: tb, side: T.FrontSide });
      for (var j = 0; j < g[1]; j++) for (var i = 0; i < g[0]; i++) {
        var grp = new T.Group();
        var uv = function (geo, flipX) {
          var at = geo.attributes.uv;
          for (var n = 0; n < at.count; n++) {
            var x = at.getX(n), y = at.getY(n);
            if (flipX) x = 1 - x;
            at.setXY(n, (i + x) / g[0], 1 - (j + 1 - y) / g[1]);
          }
        };
        var fa = new T.PlaneGeometry(tw + 0.5, th + 0.5); uv(fa, false);
        grp.add(new T.Mesh(fa, matA));
        if (flips) {
          var fb = new T.PlaneGeometry(tw + 0.5, th + 0.5); uv(fb, true);
          var mb = new T.Mesh(fb, matB);
          mb.rotation.y = Math.PI;
          grp.add(mb);
        }
        scene.add(grp);
        tiles.push({ m: grp, k: j * g[0] + i, cx: (i + 0.5) * tw, cy: (j + 0.5) * th });
      }
      renderer.setPixelRatio(scale);
      renderer.setSize(W, H, false);
      canvas = renderer.domElement;
      canvas.style.cssText = "position:absolute;left:0;top:0;width:" + W + "px;height:" + H + "px;z-index:5";
      stage.appendChild(canvas);
      a.style.visibility = "hidden"; b.style.visibility = "hidden";
      cleanup = function () {
        if (canvas && canvas.parentNode) canvas.parentNode.removeChild(canvas);
        a.style.visibility = ""; b.style.visibility = "";
        ta.dispose(); tb.dispose();
        scene.traverse(function (o) { if (o.geometry) o.geometry.dispose(); });
      };
      var t0 = performance.now(), dir = String(tr.dir || "l").split("-")[0];
      var frame = function (now) {
        if (cancelled) return;
        var p = Math.min(1, (now - t0) / tr.dur);
        tiles.forEach(function (t) {
          t.m.visible = true;
          place(tr.type, dir, t.m, t, p, W, H);
        });
        renderer.render(scene, cam);
        if (p < 1) raf = requestAnimationFrame(frame);
        else finish();
      };
      raf = requestAnimationFrame(frame);
    }, function () {
      // a slide could not be drawn: fade instead
      if (cancelled) return;
      var t0 = performance.now();
      b.style.opacity = "0";
      var fade = function (now) {
        if (cancelled) return;
        var p = Math.min(1, (now - t0) / tr.dur);
        b.style.opacity = String(p);
        if (p < 1) raf = requestAnimationFrame(fade); else { b.style.opacity = ""; finish(); }
      };
      cleanup = function () { b.style.opacity = ""; };
      raf = requestAnimationFrame(fade);
    });
    return { finish: finish };
  }

  return { start: start, available: function () { return ready(); } };
})();
