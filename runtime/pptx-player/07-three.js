// 3D shapes drawn with three.js (window.__dfThree, embedded only in decks with 3D content).
//
// The renderer writes every 3D shape as an SVG approximation and marks its path with
// data-df-pptx-3d (bevel, extrusion, contour, material, camera, light; see
// internal/pptx/render/threed.go). When WebGL is available, the player builds the shape as an
// extruded, bevelled mesh, renders it once into a picture and lays the picture over the shape;
// the path keeps only its outline. Previews and browsers without WebGL keep the approximation.

// hasWebGL tells whether the browser can create a WebGL context. It is asked before three.js
// creates a renderer, because three.js reports a failed context as console errors.
var hasWebGL = (function () {
  var known = null;
  return function () {
    if (known !== null) return known;
    known = false;
    try {
      var c = document.createElement("canvas");
      var gl = c.getContext("webgl2") || c.getContext("webgl");
      if (gl) {
        known = true;
        var lose = gl.getExtension("WEBGL_lose_context");
        if (lose) lose.loseContext();
      }
    } catch (e) {}
    return known;
  };
})();

var Shapes3D = (function () {
  var svgNS = "http://www.w3.org/2000/svg";
  var T = null, renderer = null, failed = false, probe = null;

  function ready() {
    if (failed) return false;
    T = window.__dfThree;
    if (!T || !hasWebGL()) { failed = true; return false; }
    if (renderer) return true;
    try {
      var c = document.createElement("canvas");
      renderer = new T.WebGLRenderer({ canvas: c, alpha: true, antialias: true, preserveDrawingBuffer: true });
      renderer.outputColorSpace = T.SRGBColorSpace;
      renderer.setClearColor(0x000000, 0);
    } catch (e) { failed = true; return false; }
    return true;
  }

  // probePath returns a detached path element (in an invisible svg) to measure path data.
  function probePath(d) {
    if (!probe) {
      var svg = document.createElementNS(svgNS, "svg");
      svg.setAttribute("style", "position:absolute;width:0;height:0;overflow:hidden");
      probe = document.createElementNS(svgNS, "path");
      svg.appendChild(probe);
      (document.body || document.documentElement).appendChild(svg);
    }
    probe.setAttribute("d", d);
    return probe;
  }

  // subpaths turns absolute path data (M L H V C Q A Z, as the renderer writes it) into one
  // polygon per subpath (slide px). Corners stay exact; curves are sampled every 1.5 px.
  function subpaths(d) {
    var tok = String(d).match(/[MLHVCQAZ]|-?\d*\.?\d+(?:e[-+]?\d+)?/gi) || [];
    var out = [], cur = null, x = 0, y = 0, i = 0, cmd = "";
    var num = function () { return parseFloat(tok[i++]); };
    var add = function (px, py) { if (cur) cur.push({ x: px, y: py }); x = px; y = py; };
    var curve = function (fn, len) {
      var n = Math.max(4, Math.min(400, Math.ceil(len / 1.5)));
      for (var k = 1; k <= n; k++) { var q = fn(k / n); add(q.x, q.y); }
    };
    while (i < tok.length) {
      if (/^[A-Z]$/i.test(tok[i])) cmd = tok[i++].toUpperCase();
      var x0 = x, y0 = y;
      switch (cmd) {
        case "M":
          if (cur && cur.length >= 3) out.push(cur);
          cur = []; add(num(), num()); cmd = "L"; break;
        case "L": add(num(), num()); break;
        case "H": add(num(), y); break;
        case "V": add(x, num()); break;
        case "C":
          var c1x = num(), c1y = num(), c2x = num(), c2y = num(), ex = num(), ey = num();
          curve(function (t) {
            var u = 1 - t;
            return { x: u * u * u * x0 + 3 * u * u * t * c1x + 3 * u * t * t * c2x + t * t * t * ex,
              y: u * u * u * y0 + 3 * u * u * t * c1y + 3 * u * t * t * c2y + t * t * t * ey };
          }, Math.hypot(c1x - x0, c1y - y0) + Math.hypot(c2x - c1x, c2y - c1y) + Math.hypot(ex - c2x, ey - c2y));
          break;
        case "Q":
          var qx = num(), qy = num(), qx2 = num(), qy2 = num();
          curve(function (t) {
            var u = 1 - t;
            return { x: u * u * x0 + 2 * u * t * qx + t * t * qx2, y: u * u * y0 + 2 * u * t * qy + t * t * qy2 };
          }, Math.hypot(qx - x0, qy - y0) + Math.hypot(qx2 - qx, qy2 - qy));
          break;
        case "A":
          var rx = num(), ry = num(), rot = num(), large = num(), sweep = num(), ax = num(), ay = num();
          var el = probePath("M" + x0 + " " + y0 + "A" + rx + " " + ry + " " + rot + " " + large + " " + sweep + " " + ax + " " + ay), len = 0;
          try { len = el.getTotalLength(); } catch (e) { }
          if (len > 0) curve(function (t) { var q = el.getPointAtLength(len * t); return { x: q.x, y: q.y }; }, len);
          add(ax, ay);
          break;
        case "Z":
          if (cur && cur.length >= 3) out.push(cur);
          if (cur && cur.length) { x = cur[0].x; y = cur[0].y; }
          cur = null; break;
        default: i++;
      }
    }
    if (cur && cur.length >= 3) out.push(cur);
    // drop repeated points (the closing point of a curve onto the start)
    return out.map(function (p) {
      var q = p.filter(function (a, k) { var b = p[(k + 1) % p.length]; return Math.abs(a.x - b.x) > 1e-3 || Math.abs(a.y - b.y) > 1e-3; });
      return q;
    }).filter(function (p) { return p.length >= 3; });
  }

  function inside(pt, poly) {
    var c = false;
    for (var i = 0, j = poly.length - 1; i < poly.length; j = i++) {
      var a = poly[i], b = poly[j];
      if ((a.y > pt.y) !== (b.y > pt.y) && pt.x < (b.x - a.x) * (pt.y - a.y) / (b.y - a.y) + a.x) c = !c;
    }
    return c;
  }

  // shapesOf turns polygons into three.js shapes: a polygon inside an odd number of others is a
  // hole of the smallest one around it. y is flipped (SVG y points down).
  function shapesOf(polys) {
    var depth = polys.map(function (p, i) {
      var n = 0;
      polys.forEach(function (q, j) { if (i !== j && inside(p[0], q)) n++; });
      return n;
    });
    var toV = function (p) { return p.map(function (q) { return new T.Vector2(q.x, -q.y); }); };
    var shapes = [], owner = [];
    polys.forEach(function (p, i) {
      if (depth[i] % 2 === 0) { owner[i] = shapes.length; shapes.push(new T.Shape(toV(p))); }
    });
    polys.forEach(function (p, i) {
      if (depth[i] % 2 === 0) return;
      var best = -1;
      polys.forEach(function (q, j) {
        if (depth[j] === depth[i] - 1 && inside(p[0], q) && (best < 0 || polys[j].length < polys[best].length)) best = j;
      });
      if (best >= 0) shapes[owner[best]].holes.push(new T.Path(toV(p)));
    });
    return shapes;
  }

  // colorOf returns the fill color of a path: its own color, or the mean of a gradient's stops.
  function colorOf(path) {
    var f = path.getAttribute("fill") || "";
    if (/^#[0-9a-f]{6}$/i.test(f)) return f;
    var m = /^url\(#([^)]+)\)$/.exec(f);
    var g = m && document.getElementById(m[1]);
    if (!g) return null;
    var stops = g.querySelectorAll("stop"), r = 0, gg = 0, b = 0, n = 0;
    [].forEach.call(stops, function (s) {
      var c = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(s.getAttribute("stop-color") || "");
      if (!c) return;
      r += parseInt(c[1], 16); gg += parseInt(c[2], 16); b += parseInt(c[3], 16); n++;
    });
    if (!n) return null;
    var h = function (v) { return ("0" + Math.round(v / n).toString(16)).slice(-2); };
    return "#" + h(r) + h(gg) + h(b);
  }

  // camera presets as rotations (latitude, longitude, revolution) and projection
  var presets = {
    orthographicFront: [0, 0, 0],
    // isometric presets: the shape turns 45 degrees in its plane, then tilts by 54.74 degrees,
    // so that its face becomes the top (bottom, left or right) of a block (measured against the
    // PowerPoint reference of isometricTopUp; the others follow by symmetry)
    isometricTopUp: { x: -54.74, z: 45 }, isometricTopDown: { x: -54.74, z: -45 },
    isometricBottomUp: { x: 54.74, z: 45 }, isometricBottomDown: { x: 54.74, z: -45 },
    isometricLeftUp: { y: 54.74, z: 45 }, isometricLeftDown: { y: 54.74, z: -45 },
    isometricRightUp: { y: -54.74, z: 45 }, isometricRightDown: { y: -54.74, z: -45 },
    isometricOffAxis1Left: [-20, 60, 0], isometricOffAxis1Right: [-20, -60, 0], isometricOffAxis1Top: [-60, 20, 0],
    isometricOffAxis2Left: [-20, 30, 0], isometricOffAxis2Right: [-20, -30, 0], isometricOffAxis2Top: [-60, -20, 0],
    isometricOffAxis3Left: [20, 60, 0], isometricOffAxis3Right: [20, -60, 0], isometricOffAxis3Bottom: [60, 20, 0],
    isometricOffAxis4Left: [20, 30, 0], isometricOffAxis4Right: [20, -30, 0], isometricOffAxis4Bottom: [60, -20, 0],
    obliqueTopLeft: [-15, 15, 0], obliqueTop: [-15, 0, 0], obliqueTopRight: [-15, -15, 0], obliqueLeft: [0, 15, 0],
    obliqueRight: [0, -15, 0], obliqueBottomLeft: [15, 15, 0], obliqueBottom: [15, 0, 0], obliqueBottomRight: [15, -15, 0],
    perspectiveFront: [0, 0, 0], perspectiveLeft: [0, 20, 0], perspectiveRight: [0, -20, 0],
    perspectiveAbove: [-20, 0, 0], perspectiveBelow: [20, 0, 0],
    perspectiveAboveLeftFacing: [-15, 25, 0], perspectiveAboveRightFacing: [-15, -25, 0],
    perspectiveContrastingLeftFacing: [-10, 40, 0], perspectiveContrastingRightFacing: [-10, -40, 0],
    perspectiveHeroicLeftFacing: [10, 35, 0], perspectiveHeroicRightFacing: [10, -35, 0],
    perspectiveHeroicExtremeLeftFacing: [15, 50, 0], perspectiveHeroicExtremeRightFacing: [15, -50, 0],
    perspectiveRelaxed: [-30, 0, 0], perspectiveRelaxedModerately: [-20, 0, 0]
  };

  function material(name, color, map) {
    var c = new T.Color(map ? "#ffffff" : color), o = { color: c };
    if (map) o.map = map;
    var ph = function (spec, shin) { o.specular = new T.Color(spec); o.shininess = shin; return new T.MeshPhongMaterial(o); };
    switch (name) {
      case "metal": return ph(0x999999, 60);
      case "softmetal": return ph(0x666666, 30);
      case "plastic": case "clear": case "translucentPowder": return ph(0x555555, 40);
      case "flat": return new T.MeshBasicMaterial(o);
      case "matte": case "powder": case "dkEdge": return new T.MeshLambertMaterial(o);
      default: return ph(0x222222, 15);
    }
  }

  // lights: a key light from the rig's direction and, except for the flat and single-light
  // rigs, a weaker fill light from the front right, as in the three-point rigs. The shares of
  // ambient, key and fill light add up to pi on a face turned to the viewer (three.js's Lambert
  // term divides by pi), so that such a face keeps its fill color.
  function lights(scene, d) {
    var dirs = { t: [0, 1], tl: [-1, 1], tr: [1, 1], l: [-1, 0], r: [1, 0], b: [0, -1], bl: [-1, -1], br: [1, -1] };
    var v = dirs[d.lightDir] || dirs.t, el = { harsh: 35, contrasting: 35, sunrise: 35, sunset: 35, soft: 55, balanced: 55, brightRoom: 55, glow: 55 }[d.light] || 45;
    var a = Math.atan2(v[1], v[0]) + (d.lightRev || 0) * Math.PI / 180, z = Math.sin(el * Math.PI / 180), xy = Math.cos(el * Math.PI / 180);
    if (d.light === "flat") { scene.add(new T.AmbientLight(0xffffff, Math.PI)); return; }
    var fillShare = /^(harsh|contrasting|flood)$/.test(d.light) ? 0.15 : 0.25, amb = 0.3, keyShare = 1 - amb - fillShare;
    var key = new T.DirectionalLight(0xffffff, Math.PI * keyShare / z);
    key.position.set(Math.cos(a) * xy, Math.sin(a) * xy, z);
    var f = new T.Vector3(0.7, 0.1, 0.7).normalize();
    var fill = new T.DirectionalLight(0xffffff, Math.PI * fillShare / f.z);
    fill.position.copy(f);
    scene.add(new T.AmbientLight(0xffffff, Math.PI * amb), key, fill);
  }

  // source returns what a 3D element is made of: a shape path (its fill color) or a picture
  // group (the clip path and the picture).
  function source(el) {
    if (el.tagName.toLowerCase() === "path") {
      var color = colorOf(el);
      return color ? { d: el.getAttribute("d"), color: color, transform: el.getAttribute("transform") } : null;
    }
    var clipped = el.querySelector("g[clip-path]"), image = el.querySelector("image");
    var m = clipped && /url\(#([^)]+)\)/.exec(clipped.getAttribute("clip-path"));
    var cp = m && document.getElementById(m[1]), cpath = cp && cp.querySelector("path");
    if (!cpath || !image || !image.getAttribute("href")) return null;
    var num = function (a) { return parseFloat(image.getAttribute(a)) || 0; };
    return { d: cpath.getAttribute("d"), href: image.getAttribute("href"), img: { x: num("x"), y: num("y"), w: num("width"), h: num("height") } };
  }

  // render draws one 3D element and lays the picture of it over the element. scale is the device
  // pixel size of one slide px; tex is the loaded picture of a picture element.
  function render(el, src, scale, tex) {
    var d;
    try { d = JSON.parse(el.getAttribute("data-df-pptx-3d")); } catch (e) { return; }
    var polys = subpaths(src.d);
    if (!polys.length) return;
    var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    polys.forEach(function (p) { p.forEach(function (q) { minX = Math.min(minX, q.x); minY = Math.min(minY, q.y); maxX = Math.max(maxX, q.x); maxY = Math.max(maxY, q.y); }); });
    var bt = d.bevelT || d.bevelB, depth = d.depth || 0;
    var bw = bt ? Math.min(bt.w, (maxX - minX) / 2 - 0.5, (maxY - minY) / 2 - 0.5) : 0;
    var opts = {
      depth: Math.max(depth, 0.01), steps: 1, curveSegments: 1,
      bevelEnabled: !!bt && bw > 0, bevelThickness: bt ? bt.h : 0, bevelSize: Math.max(0, bw), bevelOffset: -Math.max(0, bw),
      bevelSegments: bt && /^(angle|coolSlant|slope|hardEdge)$/.test(bt.prst) ? 1 : 8
    };
    if (tex) {
      // the picture is projected from the front onto the face and the bevel
      var im = src.img;
      var uv = function (v, k) { return new T.Vector2((v[k * 3] - im.x) / im.w, (v[k * 3 + 1] + im.y + im.h) / im.h); };
      opts.UVGenerator = {
        generateTopUV: function (g, v, a, b, c) { return [uv(v, a), uv(v, b), uv(v, c)]; },
        generateSideWallUV: function (g, v, a, b, c, e) { return [uv(v, a), uv(v, b), uv(v, c), uv(v, e)]; }
      };
    }
    var geo = new T.ExtrudeGeometry(shapesOf(polys), opts);
    var scene = new T.Scene(), group = new T.Group();
    // caps take the fill (or the picture); sides, that is the bevel and the extrusion, the
    // extrusion color when there is one
    var side = depth > 0 && d.depthColor ? material(d.material, d.depthColor) : material(d.material, src.color, tex);
    var front = new T.Mesh(geo, [material(d.material, src.color, tex), side]);
    group.add(front);
    // turn the shape about its center as the camera preset (or explicit rotation) says
    // PowerPoint turns a shape about the center of its front face; the extrusion lies behind
    var cx = (minX + maxX) / 2, cy = -(minY + maxY) / 2, cz = Math.max(depth, 0.01) + (opts.bevelEnabled ? opts.bevelThickness : 0);
    front.position.set(-cx, -cy, -cz);
    var rot = d.rot || presets[d.camera] || [0, 0, 0], deg = Math.PI / 180;
    if (rot.length) {
      // a camera rotation (latitude, longitude, revolution): the shape turns with the latitude
      // and against longitude and revolution (signs checked against the PowerPoint reference)
      group.rotation.order = "ZYX";
      group.rotation.set(rot[0] * deg, -rot[1] * deg, -rot[2] * deg);
    } else {
      // a turn of the shape itself: first in its plane (z), then about x and y
      group.rotation.order = "XYZ";
      group.rotation.set((rot.x || 0) * deg, (rot.y || 0) * deg, (rot.z || 0) * deg);
    }
    scene.add(group);
    lights(scene, d);
    // fit the camera around the turned mesh
    var box = new T.Box3().setFromObject(group), m = 2;
    var w = box.max.x - box.min.x + 2 * m, h = box.max.y - box.min.y + 2 * m;
    var cam, mx = (box.min.x + box.max.x) / 2, my = (box.min.y + box.max.y) / 2;
    if (/^perspective/.test(d.camera)) {
      // the picture covers the field of view at the distance of the shape's center, so that the
      // shape keeps its size on the slide
      var fov = d.fov || 45, dist = Math.max(w, h) * 1.6, vis = 2 * dist * Math.tan(fov * Math.PI / 360);
      cam = new T.PerspectiveCamera(fov, w / h, 1, dist * 4);
      cam.position.set(mx, my, dist);
      h = Math.max(vis, h); w = h * (box.max.x - box.min.x + 2 * m) / (box.max.y - box.min.y + 2 * m);
      cam.aspect = w / h; cam.fov = 2 * Math.atan(h / 2 / dist) * 180 / Math.PI; cam.updateProjectionMatrix();
      box.min.x = mx - w / 2 + m; box.max.x = mx + w / 2 - m; box.min.y = my - h / 2 + m; box.max.y = my + h / 2 - m;
    } else {
      cam = new T.OrthographicCamera(box.min.x - m, box.max.x + m, box.max.y + m, box.min.y - m, -1e5, 1e5);
      cam.position.set(0, 0, 1e4);
    }
    var pw = Math.max(1, Math.min(4096, Math.round(w * scale))), ph = Math.max(1, Math.min(4096, Math.round(h * scale)));
    renderer.setSize(pw, ph, false);
    renderer.render(scene, cam);
    var url = renderer.domElement.toDataURL("image/png");
    geo.dispose();
    if (tex) tex.dispose();
    // the picture covers the area of the turned mesh, centered on the shape
    var img = document.createElementNS(svgNS, "image");
    img.setAttribute("x", String((box.min.x - m) + cx));
    img.setAttribute("y", String(-(box.max.y + m) - cy));
    img.setAttribute("width", String(w));
    img.setAttribute("height", String(h));
    img.setAttribute("preserveAspectRatio", "none");
    img.setAttribute("data-df-pptx-3d-image", "");
    if (src.transform) img.setAttribute("transform", src.transform);
    img.setAttribute("href", url);
    el.parentNode.insertBefore(img, el);
    if (src.href) {
      el.setAttribute("visibility", "hidden"); // the picture group
    } else {
      // the path keeps its outline
      el.setAttribute("fill", "none");
      el.removeAttribute("filter");
    }
    // the approximation of the extrusion behind it is no longer needed
    for (var s = img.previousSibling; s && s.getAttribute; s = s.previousSibling) {
      if (s.hasAttribute("data-df-pptx-3d-base")) s.setAttribute("visibility", "hidden");
    }
    el.setAttribute("data-df-pptx-3d-done", "");
  }

  // chartMeshes builds the plot of a 3D chart (data from chart3DAttr in render/chart2.go) in
  // plot coordinates: x to the right, y up from the bottom of the plot, z toward the viewer.
  function chartMeshes(d, group) {
    var W = d.plot[2], H = d.plot[3], ns = d.series.length, m = 0;
    d.series.forEach(function (v) { m = Math.max(m, v.length); });
    var mat = function (c) { return new T.MeshPhongMaterial({ color: new T.Color(c || "#4472c4"), specular: new T.Color(0x111111), shininess: 10 }); };
    var val = function (v) { return (v - d.lo) / ((d.hi - d.lo) || 1) * H; };
    if (d.type === "pie") {
      var vals = d.series[0] || [], total = 0;
      vals.forEach(function (v) { if (v > 0) total += v; });
      var rad = Math.min(W, H) / 2 * 0.9, a = Math.PI / 2;
      vals.forEach(function (v, i) {
        if (!(v > 0) || !total) return;
        var sw = v / total * 2 * Math.PI, sh = new T.Shape();
        sh.moveTo(0, 0);
        sh.absarc(0, 0, rad, a, a - sw, true);
        sh.lineTo(0, 0);
        var geo = new T.ExtrudeGeometry(sh, { depth: rad * 0.25, bevelEnabled: false, curveSegments: 48 });
        var mesh = new T.Mesh(geo, mat(d.colors[i]));
        mesh.position.set(W / 2, H / 2, 0);
        group.add(mesh);
        a -= sw;
      });
      return;
    }
    if (d.type === "surface") {
      var rows = ns, cols = m;
      var geo2 = new T.PlaneGeometry(W, W * 0.6, cols - 1, rows - 1);
      var pos = geo2.attributes.position, colors = [];
      for (var k = 0; k < pos.count; k++) {
        var ci = k % cols, ri = Math.floor(k / cols), v = (d.series[rows - 1 - ri] || [])[ci] || 0;
        pos.setZ(k, val(v));
        var band = Math.max(0, Math.floor((v - d.lo) / ((d.hi - d.lo) || 1) * 6));
        var c = new T.Color(d.colors[band % d.colors.length] || "#4472c4");
        colors.push(c.r, c.g, c.b);
      }
      geo2.setAttribute("color", new T.Float32BufferAttribute(colors, 3));
      geo2.computeVertexNormals();
      var surf = new T.Mesh(geo2, new T.MeshLambertMaterial({ vertexColors: true, side: T.DoubleSide }));
      surf.rotation.x = -Math.PI / 2;
      surf.position.set(W / 2, 0, -W * 0.3);
      group.add(surf);
      return;
    }
    var band = W / Math.max(1, m), depth = d.type === "bar" ? band * 0.5 : band * 0.4;
    if (d.type === "bar") {
      var bw = band / (d.stacked ? 1.6 : ns + 1.5);
      for (var i = 0; i < m; i++) {
        var base = 0;
        for (var si = 0; si < ns; si++) {
          var v1 = (d.series[si] || [])[i] || 0, h = Math.abs(val(v1) - val(0));
          if (h <= 0) continue;
          var box = new T.Mesh(new T.BoxGeometry(bw, h, depth), mat(d.colors[si]));
          var x = i * band + (d.stacked ? (band - bw) / 2 : bw * 0.75 + si * bw) + bw / 2;
          var y0 = d.stacked ? base : val(0);
          box.position.set(x, y0 + (v1 >= 0 ? h / 2 : -h / 2), 0);
          if (d.stacked) base += h;
          group.add(box);
        }
      }
      return;
    }
    // line and area: one extruded band per series, the later series further back
    d.series.forEach(function (vals2, si) {
      var sh = new T.Shape(), n = vals2.length;
      if (n < 2) return;
      var xs = function (i) { return (i + 0.5) * band; };
      if (d.type === "area") {
        sh.moveTo(xs(0), val(0));
        vals2.forEach(function (v, i) { sh.lineTo(xs(i), val(v)); });
        sh.lineTo(xs(n - 1), val(0));
      } else {
        var t = Math.max(3, H * 0.02);
        vals2.forEach(function (v, i) { if (i === 0) sh.moveTo(xs(i), val(v) + t); else sh.lineTo(xs(i), val(v) + t); });
        for (var j = n - 1; j >= 0; j--) sh.lineTo(xs(j), val(vals2[j]) - t);
      }
      var g = new T.ExtrudeGeometry(sh, { depth: depth, bevelEnabled: false });
      var mesh = new T.Mesh(g, mat(d.colors[si]));
      mesh.position.z = -(ns - 1 - si) * depth * 1.5;
      group.add(mesh);
    });
  }

  // renderChart draws the plot of a 3D chart and lays the picture over the flat drawing.
  function renderChart(el, scale) {
    var d;
    try { d = JSON.parse(el.getAttribute("data-df-pptx-chart3d")); } catch (e) { return; }
    var scene = new T.Scene(), group = new T.Group(), inner = new T.Group();
    chartMeshes(d, inner);
    var W = d.plot[2], H = d.plot[3];
    inner.position.set(-W / 2, -H / 2, 0);
    group.add(inner);
    group.rotation.order = "XYZ";
    group.rotation.set((d.rotX || 0) * Math.PI / 180, -(d.rotY || 0) * Math.PI / 180, 0);
    scene.add(group);
    lights(scene, { light: "threePt", lightDir: "t" });
    var box = new T.Box3().setFromObject(group), m = 4;
    var w = box.max.x - box.min.x + 2 * m, h = box.max.y - box.min.y + 2 * m;
    // fit into the plot area: the picture keeps its aspect ratio
    var k = Math.min(W / w, H / h, 1.2);
    var cam = new T.OrthographicCamera(box.min.x - m, box.max.x + m, box.max.y + m, box.min.y - m, -1e5, 1e5);
    cam.position.set(0, 0, 1e4);
    renderer.setSize(Math.max(1, Math.min(4096, Math.round(w * k * scale))), Math.max(1, Math.min(4096, Math.round(h * k * scale))), false);
    renderer.render(scene, cam);
    var url = renderer.domElement.toDataURL("image/png");
    scene.traverse(function (o) { if (o.geometry) o.geometry.dispose(); });
    var img = document.createElementNS(svgNS, "image");
    img.setAttribute("x", String(d.plot[0] + (W - w * k) / 2));
    img.setAttribute("y", String(d.plot[1] + (H - h * k) / 2));
    img.setAttribute("width", String(w * k));
    img.setAttribute("height", String(h * k));
    img.setAttribute("preserveAspectRatio", "none");
    img.setAttribute("data-df-pptx-3d-image", "");
    img.setAttribute("href", url);
    el.parentNode.insertBefore(img, el);
    el.setAttribute("visibility", "hidden");
    el.setAttribute("data-df-pptx-3d-done", "");
  }

  // slide draws the 3D elements of a slide that are not drawn yet; pictures once they have
  // loaded.
  function slide(section, scale) {
    var charts = section.querySelectorAll("[data-df-pptx-chart3d]:not([data-df-pptx-3d-done])");
    if (charts.length && ready()) {
      [].forEach.call(charts, function (el) { try { renderChart(el, scale); } catch (e) { el.setAttribute("data-df-pptx-3d-done", "failed"); } });
    }
    var els = section.querySelectorAll("[data-df-pptx-3d]:not([data-df-pptx-3d-done])");
    if (!els.length || !ready()) return;
    [].forEach.call(els, function (el) {
      var src = source(el);
      if (!src) { el.setAttribute("data-df-pptx-3d-done", "skipped"); return; }
      var draw = function (tex) {
        try { render(el, src, scale, tex); } catch (e) { el.setAttribute("data-df-pptx-3d-done", "failed"); }
      };
      if (!src.href) { draw(null); return; }
      el.setAttribute("data-df-pptx-3d-done", "loading");
      var im = new Image();
      im.onload = function () {
        var tex = new T.Texture(im);
        tex.colorSpace = T.SRGBColorSpace;
        tex.needsUpdate = true;
        draw(tex);
      };
      im.onerror = function () { el.setAttribute("data-df-pptx-3d-done", "failed"); };
      im.src = src.href;
    });
  }

  return { slide: slide, available: function () { return ready(); } };
})();
