// Package payload builds QR content strings for structured types
// (WiFi, vCard, WhatsApp, geo, email, tel, sms) from named parameters.
package payload

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Getter returns the value of a named parameter ("" if absent).
type Getter func(name string) string

// Types lists the supported payload types.
var Types = []string{"url", "text", "wifi", "vcard", "whatsapp", "geo", "email", "tel", "sms"}

// Build assembles the QR content for typ using parameters from get.
// An empty typ behaves like "text": the raw value is passed through.
func Build(typ string, get Getter) (string, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "", "text":
		return require(get, "value")
	case "url":
		return buildURL(get)
	case "wifi":
		return buildWiFi(get)
	case "vcard":
		return buildVCard(get)
	case "whatsapp":
		return buildWhatsApp(get)
	case "geo":
		return buildGeo(get)
	case "email":
		return buildEmail(get)
	case "tel":
		p, err := phoneDigits(get("phone"))
		if err != nil {
			return "", err
		}
		return "tel:+" + p, nil
	case "sms":
		p, err := phoneDigits(get("phone"))
		if err != nil {
			return "", err
		}
		return "SMSTO:+" + p + ":" + get("message"), nil
	}
	return "", fmt.Errorf("unknown type %q (use %s)", typ, strings.Join(Types, ", "))
}

func require(get Getter, name string) (string, error) {
	v := strings.TrimSpace(get(name))
	if v == "" {
		return "", fmt.Errorf("missing required parameter: %s", name)
	}
	return v, nil
}

func buildURL(get Getter) (string, error) {
	v, err := require(get, "value")
	if err != nil {
		return "", err
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid url %q", v)
	}
	return v, nil
}

// wifiEscape escapes the characters that are special in WIFI: payloads.
var wifiEscaper = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)

func buildWiFi(get Getter) (string, error) {
	ssid, err := require(get, "ssid")
	if err != nil {
		return "", err
	}
	sec := strings.ToUpper(strings.TrimSpace(get("security")))
	if sec == "" {
		sec = "WPA"
	}
	switch sec {
	case "WPA", "WEP", "NOPASS":
	default:
		return "", fmt.Errorf("invalid security %q (use WPA, WEP or nopass)", sec)
	}
	pass := get("password")
	if sec != "NOPASS" && pass == "" {
		return "", fmt.Errorf("missing required parameter: password (or use security=nopass)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "WIFI:T:%s;S:%s;", sec, wifiEscaper.Replace(ssid))
	if sec != "NOPASS" {
		fmt.Fprintf(&b, "P:%s;", wifiEscaper.Replace(pass))
	}
	if isTrue(get("hidden")) {
		b.WriteString("H:true;")
	}
	b.WriteString(";")
	return b.String(), nil
}

// vcardEscape escapes per RFC 6350 text values.
var vcardEscaper = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\n", `\n`, "\r", "")

func buildVCard(get Getter) (string, error) {
	first := strings.TrimSpace(get("first_name"))
	last := strings.TrimSpace(get("last_name"))
	if first == "" && last == "" {
		return "", fmt.Errorf("missing required parameter: first_name or last_name")
	}
	full := strings.TrimSpace(strings.TrimSpace(first + " " + last))
	e := vcardEscaper.Replace
	lines := []string{
		"BEGIN:VCARD",
		"VERSION:3.0",
		fmt.Sprintf("N:%s;%s;;;", e(last), e(first)),
		"FN:" + e(full),
	}
	add := func(prop, param string) {
		if v := strings.TrimSpace(get(param)); v != "" {
			lines = append(lines, prop+":"+e(v))
		}
	}
	add("ORG", "org")
	add("TITLE", "title")
	add("TEL;TYPE=CELL", "phone")
	add("TEL;TYPE=WORK", "phone_work")
	add("EMAIL", "email")
	add("URL", "url")
	if street, city := get("street"), get("city"); street != "" || city != "" {
		lines = append(lines, fmt.Sprintf("ADR;TYPE=WORK:;;%s;%s;%s;%s;%s",
			e(street), e(city), e(get("state")), e(get("zip")), e(get("country"))))
	}
	lines = append(lines, "END:VCARD")
	return strings.Join(lines, "\n"), nil
}

var nonDigits = regexp.MustCompile(`\D`)

func phoneDigits(raw string) (string, error) {
	d := nonDigits.ReplaceAllString(raw, "")
	if len(d) < 7 || len(d) > 15 {
		return "", fmt.Errorf("invalid phone %q (use international format, e.g. +5215512345678)", raw)
	}
	return d, nil
}

func buildWhatsApp(get Getter) (string, error) {
	p, err := phoneDigits(get("phone"))
	if err != nil {
		return "", err
	}
	u := "https://wa.me/" + p
	if msg := get("message"); msg != "" {
		u += "?text=" + url.QueryEscape(msg)
	}
	return u, nil
}

func buildGeo(get Getter) (string, error) {
	lat, err := coord(get, "lat", 90)
	if err != nil {
		return "", err
	}
	lng, err := coord(get, "lng", 180)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("geo:%s,%s", lat, lng), nil
}

func coord(get Getter, name string, limit float64) (string, error) {
	s, err := require(get, name)
	if err != nil {
		return "", err
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < -limit || v > limit {
		return "", fmt.Errorf("invalid %s %q", name, s)
	}
	return strconv.FormatFloat(v, 'f', -1, 64), nil
}

func buildEmail(get Getter) (string, error) {
	to, err := require(get, "to")
	if err != nil {
		return "", err
	}
	if !strings.Contains(to, "@") {
		return "", fmt.Errorf("invalid email %q", to)
	}
	q := url.Values{}
	if s := get("subject"); s != "" {
		q.Set("subject", s)
	}
	if b := get("body"); b != "" {
		q.Set("body", b)
	}
	u := "mailto:" + to
	if len(q) > 0 {
		// mailto uses %20, not '+', for spaces.
		u += "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
	}
	return u, nil
}

func isTrue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
