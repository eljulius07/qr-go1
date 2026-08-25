// Package server implements the HTTP API around qrgen.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"qr-go1/payload"
	"qr-go1/qrgen"
)

const (
	maxUploadBytes = 8 << 20 // 8 MB request body cap
	maxValueLen    = 2048
)

// Config controls server-wide settings.
type Config struct {
	// BaseURL is the public origin used to build short URLs for dynamic QRs
	// (default: env BASE_URL, or http://localhost:8080).
	BaseURL string
	// DataPath is the JSON file where dynamic links are persisted
	// (default: env QR_DATA_FILE, or data/links.json).
	DataPath string
}

type api struct {
	store   *LinkStore
	baseURL string
}

// New returns the API handler with defaults; it panics if the link store
// cannot be loaded. Use NewWithConfig for explicit configuration.
func New() http.Handler {
	h, err := NewWithConfig(Config{})
	if err != nil {
		panic(err)
	}
	return h
}

func NewWithConfig(c Config) (http.Handler, error) {
	if c.BaseURL == "" {
		c.BaseURL = os.Getenv("BASE_URL")
	}
	if c.BaseURL == "" {
		c.BaseURL = "http://localhost:8080"
	}
	if c.DataPath == "" {
		c.DataPath = os.Getenv("QR_DATA_FILE")
	}
	if c.DataPath == "" {
		c.DataPath = "data/links.json"
	}
	store, err := NewLinkStore(c.DataPath)
	if err != nil {
		return nil, err
	}
	a := &api{store: store, baseURL: c.BaseURL}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", demoPage)
	mux.HandleFunc("POST /api/qr", a.handleGenerate)
	mux.HandleFunc("GET /api/qr", a.handleGenerate)
	mux.HandleFunc("POST /api/links", a.createLink)
	mux.HandleFunc("GET /api/links/{code}", a.linkStats)
	mux.HandleFunc("PATCH /api/links/{code}", a.updateLink)
	mux.HandleFunc("POST /api/links/{code}", a.updateLink)
	mux.HandleFunc("DELETE /api/links/{code}", a.deleteLink)
	mux.HandleFunc("GET /r/{code}", a.redirect)
	return withRecovery(mux), nil
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic serving %s: %v", r.URL.Path, rec)
				writeErr(w, http.StatusInternalServerError, errors.New("internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// field reads a parameter from multipart form (POST) or query string (GET).
func field(r *http.Request, name string) string {
	if v := r.FormValue(name); v != "" {
		return v
	}
	return r.URL.Query().Get(name)
}

func isTrue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func (a *api) handleGenerate(w http.ResponseWriter, r *http.Request) {
	opts := qrgen.Options{}

	// Logo upload (POST multipart only). Accepts raster formats and SVG.
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("parsing form: %w", err))
				return
			}
			if f, _, err := r.FormFile("logo"); err == nil {
				defer f.Close()
				data, rerr := io.ReadAll(f)
				if rerr != nil {
					writeErr(w, http.StatusBadRequest, fmt.Errorf("reading logo: %w", rerr))
					return
				}
				img, svgRaw, derr := qrgen.ParseLogo(data)
				if derr != nil {
					writeErr(w, http.StatusBadRequest, derr)
					return
				}
				if b := img.Bounds(); b.Dx() < 16 || b.Dy() < 16 {
					writeErr(w, http.StatusBadRequest, errors.New("logo too small (min 16x16)"))
					return
				}
				opts.Logo, opts.LogoSVG = img, svgRaw
			}
		} else if err := r.ParseForm(); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("parsing form: %w", err))
			return
		}
	}

	// Content: either a raw value, or a structured payload built from type=
	// wifi / vcard / whatsapp / geo / email / tel / sms / url.
	value, err := payload.Build(field(r, "type"), func(name string) string {
		return field(r, name)
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(value) > maxValueLen {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("content too long (max %d bytes)", maxValueLen))
		return
	}

	// Dynamic QR: store the target behind a short redirect URL and encode
	// that instead, so the destination stays editable and scans are counted.
	var dynLink *Link
	if isTrue(field(r, "dynamic")) {
		if err := ValidateTarget(value); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("dynamic QRs require an http(s) target: %w", err))
			return
		}
		dynLink, err = a.store.Create(value, field(r, "code"))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		value = a.shortURL(dynLink.Code)
		w.Header().Set("X-QR-Link-Code", dynLink.Code)
		w.Header().Set("X-QR-Short-URL", value)
		w.Header().Set("X-QR-Edit-Token", dynLink.Token)
	}
	// If generation fails after creating the link, remove it again.
	cleanupLink := func() {
		if dynLink != nil {
			if derr := a.store.Delete(dynLink.Code, dynLink.Token); derr != nil {
				log.Printf("cleaning up link %s: %v", dynLink.Code, derr)
			}
		}
	}

	opts.Value = value
	if opts.Size, err = intField(r, "size", 1024, 128, 4096); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if opts.Margin, err = intField(r, "margin", 3, 1, 10); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if s := field(r, "style"); s != "" {
		opts.Style = qrgen.Style(s)
	}
	if c := field(r, "fg"); c != "" {
		if opts.FG, err = qrgen.ParseColor(c); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("fg: %w", err))
			return
		}
	}
	if c := field(r, "fg2"); c != "" {
		if opts.FG2, err = qrgen.ParseColor(c); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("fg2: %w", err))
			return
		}
	}
	opts.Gradient = field(r, "gradient")
	if c := field(r, "bg"); c != "" {
		if opts.BG, err = qrgen.ParseColor(c); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("bg: %w", err))
			return
		}
	}
	if ec := field(r, "ec"); ec != "" {
		lvl, err := parseEC(ec)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		opts.SetECLevel(lvl)
	}
	if ls := field(r, "logo_scale"); ls != "" {
		v, err := strconv.ParseFloat(ls, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("logo_scale: %w", err))
			return
		}
		opts.LogoScale = v
	}
	validate := true
	if v := field(r, "validate"); v != "" {
		validate, err = strconv.ParseBool(v)
		if err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("validate: %w", err))
			return
		}
	}

	format := strings.ToLower(field(r, "format"))
	if format == "" {
		format = "png"
	}

	switch format {
	case "svg":
		svg, res, err := qrgen.GenerateSVG(opts)
		if err != nil {
			cleanupLink()
			writeErr(w, http.StatusUnprocessableEntity, err)
			return
		}
		setResultHeaders(w, r, res, "qr.svg")
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svg))
	case "png":
		var res *qrgen.Result
		if validate {
			res, err = qrgen.Generate(opts)
		} else {
			res, err = qrgen.GenerateUnvalidated(opts)
		}
		if err != nil {
			cleanupLink()
			writeErr(w, http.StatusUnprocessableEntity, err)
			return
		}
		setResultHeaders(w, r, res, "qr.png")
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, res.Image); err != nil {
			log.Printf("writing png: %v", err)
		}
	default:
		cleanupLink()
		writeErr(w, http.StatusBadRequest, fmt.Errorf("unknown format %q (use png or svg)", format))
	}
}

func setResultHeaders(w http.ResponseWriter, r *http.Request, res *qrgen.Result, filename string) {
	w.Header().Set("X-QR-Validated", strconv.FormatBool(res.Validated))
	w.Header().Set("X-QR-Logo-Scale", fmt.Sprintf("%.3f", res.LogoScale))
	if d := field(r, "download"); isTrue(d) {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	}
}

func intField(r *http.Request, name string, def, min, max int) (int, error) {
	s := field(r, name)
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if v < min || v > max {
		return 0, fmt.Errorf("%s must be between %d and %d", name, min, max)
	}
	return v, nil
}

func parseEC(s string) (l qrgen.RecoveryLevel, err error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "L":
		return qrgen.ECLow, nil
	case "M":
		return qrgen.ECMedium, nil
	case "Q":
		return qrgen.ECQuartile, nil
	case "H":
		return qrgen.ECHigh, nil
	}
	return 0, fmt.Errorf("unknown ec level %q (use L, M, Q or H)", s)
}
