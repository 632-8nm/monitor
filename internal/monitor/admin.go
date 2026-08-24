package monitor

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	adminCookieName = "monitor_admin"
	adminSessionTTL = 7 * 24 * time.Hour
)

// adminAuth is a small in-memory session store guarding the admin-only
// features (currently the web terminal). Credentials come from
// MONITOR_ADMIN_USER / MONITOR_ADMIN_PASS and fall back to the built-in
// defaults (admin/123456) when unset; sessions are lost on restart, which
// is acceptable for this scale.
type adminAuth struct {
	user     string
	pass     string
	mu       sync.Mutex
	sessions map[string]time.Time // token -> expiry
}

func newAdminAuth() *adminAuth {
	user := strings.TrimSpace(os.Getenv("MONITOR_ADMIN_USER"))
	pass := os.Getenv("MONITOR_ADMIN_PASS")
	if user == "" {
		user = "admin"
	}
	if pass == "" {
		pass = "123456"
	}
	return &adminAuth{
		user:     user,
		pass:     pass,
		sessions: make(map[string]time.Time),
	}
}

func (a *adminAuth) configured() bool {
	return a.user != "" && a.pass != ""
}

// login validates credentials with constant-time comparisons and mints a
// random session token.
func (a *adminAuth) login(user, pass string) (string, bool) {
	if !a.configured() {
		return "", false
	}
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user))
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(a.pass))
	if userOK&passOK != 1 {
		return "", false
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false
	}
	token := hex.EncodeToString(buf)
	a.mu.Lock()
	a.sessions[token] = time.Now().Add(adminSessionTTL)
	a.mu.Unlock()
	return token, true
}

func (a *adminAuth) logout(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

// valid checks the token and lazily prunes expired sessions.
func (a *adminAuth) valid(token string) bool {
	if token == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for t, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, t)
		}
	}
	exp, ok := a.sessions[token]
	return ok && now.Before(exp)
}

func (a *adminAuth) tokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(adminCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// AdminSessionHandler reports whether the current browser holds an admin
// session; the frontend uses it to decide between login form and terminal.
func (s *Server) AdminSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"authed": s.admin.valid(s.admin.tokenFromRequest(r))})
}

// AdminLoginHandler exchanges credentials for an HttpOnly session cookie.
func (s *Server) AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	token, ok := s.admin.login(body.User, body.Pass)
	if !ok {
		time.Sleep(500 * time.Millisecond) // blunt brute-force attempts
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(adminSessionTTL.Seconds()),
	})
	w.WriteHeader(http.StatusNoContent)
}

// AdminLogoutHandler drops the current session and clears the cookie.
func (s *Server) AdminLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(adminCookieName); err == nil {
		s.admin.logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
