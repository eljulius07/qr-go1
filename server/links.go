package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Link is a dynamic QR target: a short code that redirects to an editable
// URL while counting scans.
type Link struct {
	Code      string           `json:"code"`
	Target    string           `json:"target"`
	Token     string           `json:"token"` // edit secret, only revealed on create
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Scans     int64            `json:"scans"`
	LastScan  *time.Time       `json:"last_scan,omitempty"`
	Daily     map[string]int64 `json:"daily,omitempty"` // "2026-08-24" -> count
}

var (
	ErrLinkNotFound = errors.New("link not found")
	ErrBadToken     = errors.New("invalid edit token")
)

// LinkStore keeps links in memory and persists them to a JSON file with
// atomic writes.
type LinkStore struct {
	mu    sync.Mutex
	path  string
	links map[string]*Link
}

func NewLinkStore(path string) (*LinkStore, error) {
	s := &LinkStore{path: path, links: map[string]*Link{}}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// fresh store
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", path, err)
	default:
		if err := json.Unmarshal(data, &s.links); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	return s, nil
}

// persist writes the store to disk; callers must hold s.mu.
func (s *LinkStore) persist() error {
	data, err := json.MarshalIndent(s.links, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

const codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNPQRSTUVWXYZ123456789"

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = codeAlphabet[int(v)%len(codeAlphabet)]
	}
	return string(out)
}

var codeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

// ValidateTarget checks that a redirect target is an absolute http(s) URL.
func ValidateTarget(target string) error {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("target must be an absolute http(s) URL, got %q", target)
	}
	return nil
}

// Create stores a new link. customCode may be empty for a random code.
func (s *LinkStore) Create(target, customCode string) (*Link, error) {
	if err := ValidateTarget(target); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	code := customCode
	if code != "" {
		if !codeRe.MatchString(code) {
			return nil, fmt.Errorf("invalid code %q (3-32 chars: letters, digits, - _)", code)
		}
		if _, exists := s.links[code]; exists {
			return nil, fmt.Errorf("code %q already in use", code)
		}
	} else {
		for {
			code = randomString(6)
			if _, exists := s.links[code]; !exists {
				break
			}
		}
	}
	now := time.Now().UTC()
	l := &Link{
		Code:      code,
		Target:    strings.TrimSpace(target),
		Token:     randomString(24),
		CreatedAt: now,
		UpdatedAt: now,
		Daily:     map[string]int64{},
	}
	s.links[code] = l
	if err := s.persist(); err != nil {
		delete(s.links, code)
		return nil, fmt.Errorf("persisting link: %w", err)
	}
	cp := *l
	return &cp, nil
}

// Get returns a copy of the link.
func (s *LinkStore) Get(code, token string) (*Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return nil, ErrLinkNotFound
	}
	if l.Token != token {
		return nil, ErrBadToken
	}
	cp := *l
	return &cp, nil
}

// Update changes the target if the token matches.
func (s *LinkStore) Update(code, token, target string) (*Link, error) {
	if err := ValidateTarget(target); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return nil, ErrLinkNotFound
	}
	if l.Token != token {
		return nil, ErrBadToken
	}
	l.Target = strings.TrimSpace(target)
	l.UpdatedAt = time.Now().UTC()
	if err := s.persist(); err != nil {
		return nil, fmt.Errorf("persisting link: %w", err)
	}
	cp := *l
	return &cp, nil
}

// Delete removes the link if the token matches.
func (s *LinkStore) Delete(code, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return ErrLinkNotFound
	}
	if l.Token != token {
		return ErrBadToken
	}
	delete(s.links, code)
	return s.persist()
}

// RecordScan increments counters and returns the redirect target.
func (s *LinkStore) RecordScan(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[code]
	if !ok {
		return "", ErrLinkNotFound
	}
	now := time.Now().UTC()
	l.Scans++
	l.LastScan = &now
	if l.Daily == nil {
		l.Daily = map[string]int64{}
	}
	l.Daily[now.Format("2006-01-02")]++
	if err := s.persist(); err != nil {
		return "", fmt.Errorf("persisting scan: %w", err)
	}
	return l.Target, nil
}
