# qr-go1

API en Go para generar códigos QR estilizados (puntos redondeados, esquinas
"finder" redondeadas y logo centrado), con **validación automática**: cada QR
generado se decodifica antes de entregarse; si el logo daña la lectura, se
reduce su escala automáticamente y se reintenta.

Además soporta **contenidos estructurados** (WiFi, vCard, WhatsApp, geo…),
**QRs dinámicos** (URL corta editable con métricas de escaneo), **degradados
de color** y **logos SVG**.

## Ejecutar

```bash
go run ./cmd/server          # escucha en :8080 (o PORT)
```

Abre `http://localhost:8080/` para la página demo. Variables de entorno:
`PORT`, `BASE_URL` (origen público para las URLs cortas, p.ej.
`https://qr.tudominio.com`) y `QR_DATA_FILE` (persistencia de enlaces,
default `data/links.json`).

## API

### `POST /api/qr` (multipart/form-data) · `GET /api/qr` (query params, sin logo)

| Parámetro    | Tipo    | Default | Descripción |
|--------------|---------|---------|-------------|
| `value`      | string  | —       | Texto o URL a codificar (máx 2048 bytes); requerido salvo con `type` estructurado |
| `type`       | string  | text    | `url`, `text`, `wifi`, `vcard`, `whatsapp`, `geo`, `email`, `tel`, `sms` (ver abajo) |
| `dynamic`    | bool    | false   | Crea una URL corta editable y codifica esa URL (requiere destino http/https) |
| `code`       | string  | aleatorio | Código corto personalizado para `dynamic` (3–32 chars) |
| `logo`       | file    | —       | Logo PNG/JPEG/GIF/WebP/**SVG** (solo POST, máx 8 MB) |
| `size`       | int     | 1024    | Tamaño en px (128–4096) |
| `format`     | string  | png     | `png` o `svg` (SVG vectorial con logo embebido) |
| `style`      | string  | dots    | `dots`, `rounded` o `squares` |
| `fg`         | hex     | #000000 | Color de los módulos |
| `fg2`        | hex     | —       | Segundo color: activa el degradado |
| `gradient`   | string  | vertical | `vertical`, `horizontal`, `diagonal` o `radial` (con `fg2`) |
| `bg`         | hex     | #ffffff | Fondo; acepta `transparent` |
| `ec`         | string  | auto    | Corrección de errores `L/M/Q/H` (con logo se fuerza mínimo Q) |
| `logo_scale` | float   | 0.22    | Ancho del logo como fracción de la imagen (0.10–0.30) |
| `margin`     | int     | 3       | Zona de silencio en módulos (1–10) |
| `validate`   | bool    | true    | Decodificar el resultado antes de responder |
| `download`   | bool    | false   | Añade `Content-Disposition: attachment` |

Cabeceras de respuesta: `X-QR-Validated`, `X-QR-Logo-Scale` y, con `dynamic`,
`X-QR-Short-URL`, `X-QR-Link-Code` y `X-QR-Edit-Token` (guárdalo: no se vuelve
a mostrar).

Errores: JSON `{"error": "..."}` con estado 400 (parámetros) o 422 (no se pudo
generar un QR legible).

### Tipos de contenido (`type`)

| `type`     | Parámetros |
|------------|------------|
| `url`      | `value` (añade `https://` si falta y valida) |
| `wifi`     | `ssid`*, `password` (*u `security=nopass`*), `security` (WPA/WEP/nopass), `hidden` |
| `vcard`    | `first_name`/`last_name` (uno requerido), `org`, `title`, `phone`, `phone_work`, `email`, `url`, `street`, `city`, `state`, `zip`, `country` |
| `whatsapp` | `phone`* (con código de país), `message` |
| `geo`      | `lat`*, `lng`* |
| `email`    | `to`*, `subject`, `body` |
| `tel`/`sms`| `phone`* (+ `message` para sms) |

### QRs dinámicos

```bash
# 1. Generar un QR dinámico en un paso (crea la URL corta y la codifica)
curl -s -D - -o qr.png "http://localhost:8080/api/qr?value=https://pass2fun.com/promo&dynamic=1"
# → X-QR-Short-URL: http://localhost:8080/r/Ab3xYz   X-QR-Edit-Token: ...

# 2. Cambiar el destino SIN reimprimir el QR
curl -X PATCH http://localhost:8080/api/links/Ab3xYz \
  -d target=https://pass2fun.com/otra-promo -d token=EL_TOKEN

# 3. Métricas de escaneo (total, último, por día)
curl "http://localhost:8080/api/links/Ab3xYz?token=EL_TOKEN"
```

También: `POST /api/links` (crear sin QR), `DELETE /api/links/{code}`,
y `GET /r/{code}` es la redirección (302) que siguen los teléfonos.
El token puede ir en el header `X-Edit-Token` en lugar del parámetro.

### Ejemplos

```bash
# Con logo, estilo dots (como el ejemplo pass2fun)
curl -o qr.png -F value="https://pass2fun.com/" -F logo=@logo.png \
  http://localhost:8080/api/qr

# WiFi con degradado radial
curl -o wifi.png "http://localhost:8080/api/qr?type=wifi&ssid=CafeGo1&password=secreta123&fg=%23064e3b&fg2=%230d9488&gradient=radial"

# SVG vectorial, degradado diagonal, logo SVG
curl -o qr.svg -F value="https://pass2fun.com/" -F logo=@logo.svg \
  -F format=svg -F fg="#1e3a8a" -F fg2="#7c3aed" -F gradient=diagonal \
  http://localhost:8080/api/qr
```

### CLI

```bash
go run ./cmd/qrgen -value "https://pass2fun.com/" -logo logo.png -out qr.png
```

## Verificación independiente (macOS)

`scripts/decode.swift` decodifica un PNG con el framework Vision de Apple — el
mismo motor de la cámara del iPhone:

```bash
swift scripts/decode.swift qr.png
```

## Tests

```bash
go test ./...
```

Los tests generan QRs con logo en los tres estilos, a tamaños grandes y
pequeños (256 px, simulando escaneo a distancia), con colores personalizados y
fondo transparente, y comprueban que todos decodifican al valor original.

## Diseño

- Codificación: `skip2/go-qrcode` (matriz de módulos). Con logo se usa
  corrección de errores H (~30 % de redundancia).
- Render: supermuestreo 4× + reducción CatmullRom para bordes suaves. Los
  degradados se calculan por píxel en PNG y con `<linearGradient>` /
  `<radialGradient>` en SVG.
- El área del logo se limpia con una "píldora" redondeada y los módulos que
  caen dentro no se dibujan; la validación garantiza que siga siendo legible.
  Los logos SVG se rasterizan con `oksvg` para el PNG y se embeben como
  vector en la salida SVG.
- Validación: `makiuchi-d/gozxing` (port de ZXing). Si falla, el logo se
  reduce 15 % por intento (hasta 4 intentos) antes de devolver error 422.
- Enlaces dinámicos: persistidos en un JSON con escritura atómica
  (`data/links.json`); cada enlace tiene un token de edición propio y
  contadores de escaneo total y por día.

## Docker

```bash
docker build -t qr-go1 .
docker run -p 8080:8080 qr-go1
```
