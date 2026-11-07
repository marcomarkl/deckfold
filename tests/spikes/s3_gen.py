"""Spike S3: writes large HTML files with Base64 blocks (one block vs. many blocks)."""
import base64, os, sys
out = sys.argv[1]
total = int(sys.argv[2]) * 1024 * 1024  # binary size in MB
chunk = int(sys.argv[3]) * 1024 * 1024 if len(sys.argv) > 3 else total
data = os.urandom(total)
probe = """<script>
window.T0 = performance.now();
addEventListener('DOMContentLoaded', () => window.TDCL = performance.now());
addEventListener('load', () => window.TLOAD = performance.now());
window.decode = (id) => { const t=performance.now(); const s=document.getElementById(id).textContent; const t1=performance.now();
  const b=atob(s); const u=new Uint8Array(b.length); for(let i=0;i<b.length;i++) u[i]=b.charCodeAt(i);
  const url=URL.createObjectURL(new Blob([u])); return {read:t1-t, decode:performance.now()-t1, bytes:u.length}; };
</script>"""
with open(out, 'w') as f:
    f.write('<!doctype html><meta charset="utf-8"><title>S3</title>' + probe)
    n = 0
    for i in range(0, total, chunk):
        f.write(f'<script type="application/octet-stream" id="b{n}">')
        f.write(base64.b64encode(data[i:i+chunk]).decode())
        f.write('</script>')
        n += 1
    f.write(f'<p id="done">{n}</p>')
print(out, os.path.getsize(out), n)
