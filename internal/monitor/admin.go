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

	// Brute-force hardening: after 3 consecutive wrong passwords, the next
	// login is refused for 30s, then 1min, 2min, 4min... (doubling per
	// further wrong attempt) capped at adminCooldownMax. The counter is
	// global on purpose: the app normally sits behind a Cloudflare Tunnel,
	// so every request comes from the tunnel itself and per-IP tracking
	// would see a single (useless) address.
	adminMaxFailures  = 3
	adminCooldownBase = 30 * time.Second
	adminCooldownMax  = 24 * time.Hour
)

// adminAuth is a small in-memory session store guarding the admin-only
// features (currently the web terminal). Credentials come from
// MONITOR_ADMIN_USER / MONITOR_ADMIN_PASS and fall back to the built-in
// defaults (admin/123456) when unset; sessions are lost on restart, which
// is acceptable for this scale.
type adminAuth struct {
	user        string
	pass        string
	mu          sync.Mutex
	sessions    map[string]time.Time // token -> expiry
	fails       int                  // consecutive wrong attempts
	lockedUntil time.Time            // login refused before this instant
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
// random session token. While a cooldown is active it returns the remaining
// wait time with ok=false (any lockout attempt is refused regardless of the
// submitted password); on a wrong password it bumps the failure counter and
// activates the next cooldown tier once the threshold is reached. A correct
// login resets the counter.
func (a *adminAuth) login(user, pass string) (time.Duration, string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.configured() {
		return 0, "", false
	}
	now := time.Now()
	if a.lockedUntil.After(now) {
		return a.lockedUntil.Sub(now), "", false
	}
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user))
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(a.pass))
	if userOK&passOK != 1 {
		a.fails++
		if a.fails >= adminMaxFailures {
			cd := adminCooldownBase
			for i, end := 0, a.fails-adminMaxFailures; i < end; i++ {
				cd *= 2
				if cd >= adminCooldownMax {
					cd = adminCooldownMax
					break
				}
			}
			a.lockedUntil = now.Add(cd)
		}
		return 0, "", false
	}
	a.fails = 0
	a.lockedUntil = time.Time{}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return 0, "", false
	}
	token := hex.EncodeToString(buf)
	a.sessions[token] = now.Add(adminSessionTTL)
	return 0, token, true
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
	retryAfter, token, ok := s.admin.login(body.User, body.Pass)
	if !ok {
		if retryAfter > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]int64{"retry_after": int64(retryAfter.Seconds())})
			return
		}
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
