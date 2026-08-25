package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"qr-go1/qrgen"
)

// newTestServer runs the API against a temp link store.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	h, err := NewWithConfig(Config{DataPath: filepath.Join(t.TempDir(), "links.json")})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func testLogoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 120; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestGetPNG(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/qr?value=https://example.com&size=512")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type %q", ct)
	}
	if v := resp.Header.Get("X-QR-Validated"); v != "true" {
		t.Fatalf("X-QR-Validated=%q", v)
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := qrgen.Verify(img, "https://example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestPostMultipartWithLogo(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("value", "https://pass2fun.com/")
	mw.WriteField("size", "768")
	fw, _ := mw.CreateFormFile("logo", "logo.png")
	fw.Write(testLogoPNG(t))
	mw.Close()

	resp, err := http.Post(srv.URL+"/api/qr", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := qrgen.Verify(img, "https://pass2fun.com/"); err != nil {
		t.Fatal(err)
	}
}

func TestGetSVG(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/qr?value=hola&format=svg&size=512")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
		t.Fatalf("content-type %q", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(b), "<svg") {
		t.Fatal("not svg output")
	}
}

func TestErrors(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	cases := []struct {
		url  string
		code int
	}{
		{"/api/qr", http.StatusBadRequest},                                  // missing value
		{"/api/qr?value=x&size=9", http.StatusBadRequest},                   // size out of range
		{"/api/qr?value=x&fg=zzz", http.StatusBadRequest},                   // bad color
		{"/api/qr?value=x&format=bmp", http.StatusBadRequest},               // bad format
		{"/api/qr?value=x&ec=Z", http.StatusBadRequest},                     // bad ec
		{"/api/qr?value=x&logo_scale=0.99", http.StatusUnprocessableEntity}, // scale out of range
	}
	for _, c := range cases {
		resp, err := http.Get(srv.URL + c.url)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.code {
			t.Errorf("%s: got %d, want %d", c.url, resp.StatusCode, c.code)
		}
	}
}
