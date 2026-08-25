package server

import "net/http"

func demoPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(demoHTML))
}

const demoHTML = `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>qr-go1 demo</title>
<style>
  body{font-family:system-ui,sans-serif;max-width:900px;margin:2rem auto;padding:0 1rem;background:#fafafa;color:#111}
  form{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:.75rem;background:#fff;padding:1rem;border-radius:12px;border:1px solid #e5e5e5}
  fieldset{grid-column:1/-1;display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:.75rem;border:1px dashed #ddd;border-radius:8px;padding:.75rem}
  legend{font-size:.75rem;color:#666;padding:0 .3rem}
  label{display:flex;flex-direction:column;font-size:.8rem;gap:.25rem}
  label.chk{flex-direction:row;align-items:center;gap:.5rem}
  input,select{padding:.45rem;border:1px solid #ccc;border-radius:8px;font-size:.9rem}
  input[type=checkbox]{width:auto}
  button{grid-column:1/-1;padding:.6rem;border:0;border-radius:8px;background:#111;color:#fff;font-size:1rem;cursor:pointer}
  #out{margin-top:1.5rem;text-align:center}
  #out img,#out object{max-width:420px;width:100%;border:1px solid #e5e5e5;border-radius:12px;background:#fff}
  #meta{font-size:.8rem;color:#666;margin-top:.5rem}
  .dyn{background:#fff8e6;border:1px solid #f0d488;border-radius:8px;padding:.6rem;margin-top:.6rem;font-size:.8rem;text-align:left;word-break:break-all}
  .err{color:#b00020;white-space:pre-wrap}
</style>
</head>
<body>
<h1>qr-go1</h1>
<form id="f">
  <label>Tipo de contenido
    <select id="type" name="type">
      <option value="url">URL</option>
      <option value="text">Texto</option>
      <option value="wifi">WiFi</option>
      <option value="vcard">vCard (contacto)</option>
      <option value="whatsapp">WhatsApp</option>
      <option value="geo">Ubicación</option>
    </select>
  </label>
  <label class="chk" title="Crea una URL corta editable y codifica esa URL">
    <input type="checkbox" name="dynamic" id="dynamic"> QR dinámico (editable + métricas)
  </label>
  <fieldset id="typeFields"><legend>Contenido</legend></fieldset>
  <label>Logo (png/jpg/webp/svg)<input type="file" name="logo" accept="image/*,.svg"></label>
  <label>Estilo
    <select name="style"><option>dots</option><option>rounded</option><option>squares</option></select>
  </label>
  <label>Formato
    <select name="format"><option>png</option><option>svg</option></select>
  </label>
  <label>Tamaño (px)<input name="size" type="number" value="1024" min="128" max="4096"></label>
  <label>Color<input name="fg" type="color" value="#000000"></label>
  <label>Degradado
    <select id="gradient" name="gradient">
      <option value="">sin degradado</option>
      <option value="vertical">vertical</option>
      <option value="horizontal">horizontal</option>
      <option value="diagonal">diagonal</option>
      <option value="radial">radial</option>
    </select>
  </label>
  <label>Color 2 (degradado)<input id="fg2" name="fg2" type="color" value="#5b21b6"></label>
  <label>Fondo<input name="bg" type="color" value="#ffffff"></label>
  <label>Escala logo<input name="logo_scale" type="number" step="0.01" min="0.10" max="0.30" value="0.22"></label>
  <button>Generar</button>
</form>
<div id="out"></div>
<script>
const TYPE_FIELDS = {
  url:      [{n:'value', l:'URL', v:'https://pass2fun.com/'}],
  text:     [{n:'value', l:'Texto', v:'Hola'}],
  wifi:     [{n:'ssid', l:'Red (SSID)'}, {n:'password', l:'Contraseña'},
             {n:'security', l:'Seguridad', opts:['WPA','WEP','nopass']},
             {n:'hidden', l:'Red oculta', chk:true}],
  vcard:    [{n:'first_name', l:'Nombre'}, {n:'last_name', l:'Apellidos'},
             {n:'org', l:'Empresa'}, {n:'title', l:'Puesto'},
             {n:'phone', l:'Teléfono'}, {n:'email', l:'Email'}, {n:'url', l:'Web'}],
  whatsapp: [{n:'phone', l:'Teléfono (con código de país)', v:'+52'},
             {n:'message', l:'Mensaje'}],
  geo:      [{n:'lat', l:'Latitud', v:'20.6296'}, {n:'lng', l:'Longitud', v:'-87.0739'}],
};
const typeSel = document.getElementById('type');
const holder = document.getElementById('typeFields');
function renderTypeFields() {
  holder.innerHTML = '<legend>Contenido</legend>';
  for (const fdef of TYPE_FIELDS[typeSel.value]) {
    const lab = document.createElement('label');
    if (fdef.chk) {
      lab.className = 'chk';
      lab.innerHTML = '<input type="checkbox" name="' + fdef.n + '"> ' + fdef.l;
    } else if (fdef.opts) {
      lab.textContent = fdef.l;
      const sel = document.createElement('select');
      sel.name = fdef.n;
      for (const o of fdef.opts) sel.add(new Option(o, o));
      lab.appendChild(sel);
    } else {
      lab.textContent = fdef.l;
      const inp = document.createElement('input');
      inp.name = fdef.n;
      if (fdef.v) inp.value = fdef.v;
      lab.appendChild(inp);
    }
    holder.appendChild(lab);
  }
}
typeSel.addEventListener('change', renderTypeFields);
renderTypeFields();

const f = document.getElementById('f'), out = document.getElementById('out');
f.addEventListener('submit', async e => {
  e.preventDefault();
  out.innerHTML = 'Generando…';
  const fd = new FormData(f);
  if (fd.get('logo') && fd.get('logo').size === 0) fd.delete('logo');
  if (!fd.get('gradient')) { fd.delete('gradient'); fd.delete('fg2'); }
  try {
    const r = await fetch('/api/qr', {method:'POST', body:fd});
    if (!r.ok) {
      const j = await r.json();
      out.innerHTML = '<div class="err">' + (j.error || r.status) + '</div>';
      return;
    }
    const blob = await r.blob();
    const url = URL.createObjectURL(blob);
    let html = '<img src="' + url + '"><div id="meta">validado: ' +
      r.headers.get('X-QR-Validated') + ' · escala logo: ' +
      r.headers.get('X-QR-Logo-Scale') + ' · <a href="' + url + '" download="qr">descargar</a></div>';
    const short = r.headers.get('X-QR-Short-URL');
    if (short) {
      html += '<div class="dyn"><b>QR dinámico creado</b><br>URL corta: <code>' + short +
        '</code><br>Código: <code>' + r.headers.get('X-QR-Link-Code') +
        '</code><br>Token de edición (guárdalo, no se vuelve a mostrar): <code>' +
        r.headers.get('X-QR-Edit-Token') + '</code></div>';
    }
    out.innerHTML = html;
  } catch (err) {
    out.innerHTML = '<div class="err">' + err + '</div>';
  }
});
</script>
</body>
</html>`
