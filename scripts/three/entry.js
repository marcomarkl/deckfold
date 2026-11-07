// The parts of three.js the PowerPoint player uses (3D shapes, particle transitions, 3D charts).
// scripts/three bundles them into runtime/dist/three.js as a classic script that sets
// window.__dfThree.
export {
  WebGLRenderer, Scene, Group, Mesh, Points, Object3D,
  OrthographicCamera, PerspectiveCamera,
  AmbientLight, DirectionalLight, HemisphereLight, PointLight,
  MeshStandardMaterial, MeshPhongMaterial, MeshLambertMaterial, MeshBasicMaterial, PointsMaterial, ShaderMaterial,
  ExtrudeGeometry, ShapeGeometry, BoxGeometry, CylinderGeometry, PlaneGeometry, BufferGeometry,
  BufferAttribute, Float32BufferAttribute,
  Shape, Path, ShapeUtils, Vector2, Vector3, Euler, Quaternion, Matrix4, Box3, Color, MathUtils,
  CanvasTexture, Texture,
  SRGBColorSpace, LinearSRGBColorSpace, DoubleSide, FrontSide, NoToneMapping, AdditiveBlending, NormalBlending
} from "three";
