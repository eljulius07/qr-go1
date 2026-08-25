package payload

import (
	"strings"
	"testing"
)

func g(m map[string]string) Getter {
	return func(name string) string { return m[name] }
}

func TestURL(t *testing.T) {
	v, err := Build("url", g(map[string]string{"value": "pass2fun.com/eventos"}))
	if err != nil {
		t.Fatal(err)
	}
	if v != "https://pass2fun.com/eventos" {
		t.Fatalf("got %q", v)
	}
	if _, err := Build("url", g(map[string]string{"value": "https://"})); err == nil {
		t.Fatal("expected error for host-less url")
	}
}

func TestWiFi(t *testing.T) {
	v, err := Build("wifi", g(map[string]string{
		"ssid": "Cafe;Centro", "password": `p@ss:word,1`, "hidden": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `WIFI:T:WPA;S:Cafe\;Centro;P:p@ss\:word\,1;H:true;;`
	if v != want {
		t.Fatalf("got %q want %q", v, want)
	}

	v, err = Build("wifi", g(map[string]string{"ssid": "Libre", "security": "nopass"}))
	if err != nil {
		t.Fatal(err)
	}
	if v != "WIFI:T:NOPASS;S:Libre;;" {
		t.Fatalf("got %q", v)
	}

	if _, err := Build("wifi", g(map[string]string{"ssid": "x"})); err == nil {
		t.Fatal("expected error: WPA without password")
	}
}

func TestVCard(t *testing.T) {
	v, err := Build("vcard", g(map[string]string{
		"first_name": "Julio", "last_name": "González",
		"org": "Go1; Tours", "phone": "+52 155 1234 5678", "email": "julio@go1.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"BEGIN:VCARD", "VERSION:3.0",
		"N:González;Julio;;;", "FN:Julio González",
		`ORG:Go1\; Tours`, "TEL;TYPE=CELL:+52 155 1234 5678",
		"EMAIL:julio@go1.com", "END:VCARD",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q in:\n%s", want, v)
		}
	}
}

func TestWhatsApp(t *testing.T) {
	v, err := Build("whatsapp", g(map[string]string{
		"phone": "+52 (155) 1234-5678", "message": "Hola, ¿info de tours?",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v, "https://wa.me/5215512345678?text=") {
		t.Fatalf("got %q", v)
	}
	if _, err := Build("whatsapp", g(map[string]string{"phone": "123"})); err == nil {
		t.Fatal("expected error for short phone")
	}
}

func TestGeo(t *testing.T) {
	v, err := Build("geo", g(map[string]string{"lat": "20.6296", "lng": "-87.0739"}))
	if err != nil {
		t.Fatal(err)
	}
	if v != "geo:20.6296,-87.0739" {
		t.Fatalf("got %q", v)
	}
	if _, err := Build("geo", g(map[string]string{"lat": "99", "lng": "0"})); err == nil {
		t.Fatal("expected error for out-of-range lat")
	}
}

func TestEmailTelSMS(t *testing.T) {
	v, err := Build("email", g(map[string]string{"to": "hola@go1.com", "subject": "Reserva QR"}))
	if err != nil {
		t.Fatal(err)
	}
	if v != "mailto:hola@go1.com?subject=Reserva%20QR" {
		t.Fatalf("got %q", v)
	}
	if v, _ := Build("tel", g(map[string]string{"phone": "+52 55 1234 5678"})); v != "tel:+525512345678" {
		t.Fatalf("got %q", v)
	}
	if v, _ := Build("sms", g(map[string]string{"phone": "5215512345678", "message": "Hola"})); v != "SMSTO:+5215512345678:Hola" {
		t.Fatalf("got %q", v)
	}
}

func TestUnknownType(t *testing.T) {
	if _, err := Build("nave-espacial", g(nil)); err == nil {
		t.Fatal("expected error")
	}
}
