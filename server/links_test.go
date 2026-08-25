package server

import (
	"bytes"
	"encoding/json"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"qr-go1/qrgen"
)

// noRedirect returns a client that surfaces 3xx instead of following them.
func noRedirect() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func TestDynamicLinkLifecycle(t *testing.T) {
	srv := newTestServer(t)

	// Create.
	resp, err := http.Post(srv.URL+"/api/links", "application/x-www-form-urlencoded",
		strings.NewReader(url.Values{"target": {"https://pass2fun.com/verano"}}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	var link linkJSON
	json.NewDecoder(resp.Body).Decode(&link)
	if link.Code == "" || link.EditToken == "" || !strings.Contains(link.ShortURL, "/r/") {
		t.Fatalf("bad link response: %+v", link)
	}

	// Redirect + scan counting (twice).
	for i := 0; i < 2; i++ {
		rr, err := noRedirect().Get(srv.URL + "/r/" + link.Code)
		if err != nil {
			t.Fatal(err)
		}
		rr.Body.Close()
		if rr.StatusCode != http.StatusFound || rr.Header.Get("Location") != "https://pass2fun.com/verano" {
			t.Fatalf("redirect %d -> %q", rr.StatusCode, rr.Header.Get("Location"))
		}
	}

	// Update target with the token.
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/links/"+link.Code,
		strings.NewReader(url.Values{"target": {"https://pass2fun.com/invierno"}, "token": {link.EditToken}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ur, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	ur.Body.Close()
	if ur.StatusCode != http.StatusOK {
		t.Fatalf("update status %d", ur.StatusCode)
	}

	// Redirect now points at the new target — same code, no reprint.
	rr, _ := noRedirect().Get(srv.URL + "/r/" + link.Code)
	rr.Body.Close()
	if rr.Header.Get("Location") != "https://pass2fun.com/invierno" {
		t.Fatalf("after update redirect -> %q", rr.Header.Get("Location"))
	}

	// Stats: 3 scans recorded, token protected.
	sr, _ := http.Get(srv.URL + "/api/links/" + link.Code + "?token=" + link.EditToken)
	var stats linkJSON
	json.NewDecoder(sr.Body).Decode(&stats)
	sr.Body.Close()
	if stats.Scans != 3 || stats.EditToken != "" {
		t.Fatalf("stats: %+v", stats)
	}
	bad, _ := http.Get(srv.URL + "/api/links/" + link.Code + "?token=wrong")
	bad.Body.Close()
	if bad.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong token status %d", bad.StatusCode)
	}

	// Wrong-token update rejected.
	req2, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/links/"+link.Code,
		strings.NewReader(url.Values{"target": {"https://evil.example"}, "token": {"nope"}}.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	fr, _ := http.DefaultClient.Do(req2)
	fr.Body.Close()
	if fr.StatusCode != http.StatusForbidden {
		t.Fatalf("forged update status %d", fr.StatusCode)
	}

	// Delete, then the redirect 404s.
	dreq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/links/"+link.Code+"?token="+link.EditToken, nil)
	dr, _ := http.DefaultClient.Do(dreq)
	dr.Body.Close()
	if dr.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status %d", dr.StatusCode)
	}
	gone, _ := noRedirect().Get(srv.URL + "/r/" + link.Code)
	gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted link status %d", gone.StatusCode)
	}
}

func TestDynamicQRGeneratesShortURL(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/qr?value=https://pass2fun.com/promo&dynamic=1&size=512")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	short := resp.Header.Get("X-QR-Short-URL")
	code := resp.Header.Get("X-QR-Link-Code")
	if short == "" || code == "" || resp.Header.Get("X-QR-Edit-Token") == "" {
		t.Fatal("missing dynamic link headers")
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	// The QR must encode the short URL, not the original target.
	if err := qrgen.Verify(img, short); err != nil {
		t.Fatal(err)
	}
}

func TestDynamicRequiresURL(t *testing.T) {
	srv := newTestServer(t)
	resp, _ := http.Get(srv.URL + "/api/qr?value=hola+mundo&dynamic=1")
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestTypedPayloadWiFi(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/qr?type=wifi&ssid=CafeGo1&password=secreta123&size=512")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := qrgen.Verify(img, "WIFI:T:WPA;S:CafeGo1;P:secreta123;;"); err != nil {
		t.Fatal(err)
	}
}

func TestSVGLogoUpload(t *testing.T) {
	srv := newTestServer(t)

	svgLogo := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 80">` +
		`<rect width="200" height="80" rx="40" fill="#fff" stroke="#000" stroke-width="6"/>` +
		`<circle cx="100" cy="40" r="18" fill="#000"/></svg>`

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("value", "https://pass2fun.com/")
	mw.WriteField("size", "768")
	fw, _ := mw.CreateFormFile("logo", "logo.svg")
	fw.Write([]byte(svgLogo))
	mw.Close()

	resp, err := http.Post(srv.URL+"/api/qr", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	img, err := png.Decode(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := qrgen.Verify(img, "https://pass2fun.com/"); err != nil {
		t.Fatal(err)
	}
}

func TestGradientParam(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/qr?value=https://example.com&fg=%23102060&fg2=%23600a50&gradient=diagonal&size=512")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if resp.Header.Get("X-QR-Validated") != "true" {
		t.Fatal("gradient QR not validated")
	}
}

func TestGradientWithoutFG2Fails(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/qr?value=x&gradient=radial")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
