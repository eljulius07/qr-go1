package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// linkJSON is the public representation of a Link. The edit token is only
// included in the create response.
type linkJSON struct {
	Code      string           `json:"code"`
	ShortURL  string           `json:"short_url"`
	Target    string           `json:"target"`
	EditToken string           `json:"edit_token,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Scans     int64            `json:"scans"`
	LastScan  *time.Time       `json:"last_scan,omitempty"`
	Daily     map[string]int64 `json:"daily,omitempty"`
}

func (a *api) linkView(l *Link, includeToken bool) linkJSON {
	v := linkJSON{
		Code:      l.Code,
		ShortURL:  a.shortURL(l.Code),
		Target:    l.Target,
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
		Scans:     l.Scans,
		LastScan:  l.LastScan,
		Daily:     l.Daily,
	}
	if includeToken {
		v.EditToken = l.Token
	}
	return v
}

func (a *api) shortURL(code string) string {
	return strings.TrimRight(a.baseURL, "/") + "/r/" + code
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func linkErrStatus(err error) int {
	switch {
	case errors.Is(err, ErrLinkNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrBadToken):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}

// jsonOrForm reads a parameter from a JSON body (already decoded into m) or
// falls back to form/query values.
func param(r *http.Request, m map[string]string, name string) string {
	if v, ok := m[name]; ok && v != "" {
		return v
	}
	return field(r, name)
}

func decodeBody(r *http.Request) map[string]string {
	m := map[string]string{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&m)
	} else {
		r.ParseForm()
	}
	return m
}

// POST /api/links {target, code?}
func (a *api) createLink(w http.ResponseWriter, r *http.Request) {
	m := decodeBody(r)
	target := param(r, m, "target")
	if target == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required parameter: target"))
		return
	}
	link, err := a.store.Create(target, param(r, m, "code"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.linkView(link, true))
}

// GET /api/links/{code}?token=... — stats
func (a *api) linkStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	token := tokenFrom(r, nil)
	link, err := a.store.Get(code, token)
	if err != nil {
		writeErr(w, linkErrStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, a.linkView(link, false))
}

// PATCH/POST /api/links/{code} {target, token}
func (a *api) updateLink(w http.ResponseWriter, r *http.Request) {
	m := decodeBody(r)
	target := param(r, m, "target")
	if target == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing required parameter: target"))
		return
	}
	link, err := a.store.Update(r.PathValue("code"), tokenFrom(r, m), target)
	if err != nil {
		writeErr(w, linkErrStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, a.linkView(link, false))
}

// DELETE /api/links/{code}
func (a *api) deleteLink(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Delete(r.PathValue("code"), tokenFrom(r, nil)); err != nil {
		writeErr(w, linkErrStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /r/{code} — the redirect scanned phones follow.
func (a *api) redirect(w http.ResponseWriter, r *http.Request) {
	target, err := a.store.RecordScan(r.PathValue("code"))
	if err != nil {
		writeErr(w, linkErrStatus(err), err)
		return
	}
	// 302 (not 301) so phones re-resolve after the target is edited.
	http.Redirect(w, r, target, http.StatusFound)
}

func tokenFrom(r *http.Request, m map[string]string) string {
	if t := r.Header.Get("X-Edit-Token"); t != "" {
		return t
	}
	if m != nil {
		if t := param(r, m, "token"); t != "" {
			return t
		}
	}
	return field(r, "token")
}
