package middleware

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func useSecret(t *testing.T) []byte {
	t.Helper()
	previous := JWT_SECRET
	JWT_SECRET = []byte("test-secret")
	t.Cleanup(func() { JWT_SECRET = previous })
	return JWT_SECRET
}

func signToken(t *testing.T, method jwt.SigningMethod, key any, claims SessionClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func validClaims(audience ...string) SessionClaims {
	return SessionClaims{
		Scope: []string{"users.read"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			Audience:  jwt.ClaimStrings(audience),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func TestSessionFromJWT(t *testing.T) {
	secret := useSecret(t)

	t.Run("SV-26 a valid token for the site or the API gives its claims", func(t *testing.T) {
		for _, audience := range []string{config.SiteURL, config.APIURL} {
			claims, err := SessionFromJWT(signToken(t, jwt.SigningMethodHS256, secret, validClaims(audience)))
			if err != nil {
				t.Fatalf("audience %s: %v", audience, err)
			}
			if claims.Subject != "user-123" || len(claims.Scope) != 1 || claims.Scope[0] != "users.read" {
				t.Errorf("claims = %+v, want subject user-123 and scope users.read", claims)
			}
		}
	})

	t.Run("SV-27 a token that is forged, expired, malformed or signed another way is refused", func(t *testing.T) {
		expired := validClaims(config.SiteURL)
		expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		cases := map[string]string{
			"another secret": signToken(t, jwt.SigningMethodHS256, []byte("other-secret"), validClaims(config.SiteURL)),
			"expired":        signToken(t, jwt.SigningMethodHS256, secret, expired),
			"another method": signToken(t, jwt.SigningMethodHS384, secret, validClaims(config.SiteURL)),
			"no signature":   signToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims(config.SiteURL)),
			"malformed":      "not.a.token",
			"empty":          "",
		}
		for name, token := range cases {
			t.Run(name, func(t *testing.T) {
				if claims, err := SessionFromJWT(token); err == nil || claims != nil {
					t.Errorf("SessionFromJWT = %+v, %v, want an error and no claims", claims, err)
				}
			})
		}
	})

	t.Run("SV-28 a token with an audience that is not ours is refused and names it", func(t *testing.T) {
		for name, audience := range map[string][]string{
			"only another": {"https://evil.test"},
			"one of two":   {config.SiteURL, "https://evil.test"},
		} {
			t.Run(name, func(t *testing.T) {
				claims, err := SessionFromJWT(signToken(t, jwt.SigningMethodHS256, secret, validClaims(audience...)))
				if err == nil || claims != nil || !strings.Contains(err.Error(), "invalid audience: https://evil.test") {
					t.Errorf("SessionFromJWT = %+v, %v, want the invalid audience error", claims, err)
				}
			})
		}
	})

	t.Run("SV-29 a token without an audience is accepted", func(t *testing.T) {
		claims, err := SessionFromJWT(signToken(t, jwt.SigningMethodHS256, secret, validClaims()))
		if err != nil || claims == nil || claims.Subject != "user-123" {
			t.Errorf("SessionFromJWT = %+v, %v, want the claims", claims, err)
		}
	})
}

func TestLogRequest(t *testing.T) {
	withID := func(r *http.Request, id int) *http.Request {
		return r.WithContext(context.WithValue(r.Context(), RequestIDKey, id))
	}

	t.Run("SV-30 the line holds the request ID, the user of the session, the address and the message", func(t *testing.T) {
		logged := captureLog(t)
		r := withID(httptest.NewRequest(http.MethodGet, "/x", nil), 42)
		r = r.WithContext(context.WithValue(r.Context(), SessionKey, &SessionClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123"}}))
		LogRequest(r, "200", "GET", "/x")
		if !strings.HasSuffix(logged.String(), "42 user-123 192.0.2.1:1234 200 GET /x\n") {
			t.Errorf("log = %q", logged.String())
		}
	})

	t.Run("SV-31 without a session the user is N/A", func(t *testing.T) {
		logged := captureLog(t)
		LogRequest(withID(httptest.NewRequest(http.MethodGet, "/x", nil), 7), "hello")
		var typedNil *SessionClaims
		r := withID(httptest.NewRequest(http.MethodGet, "/x", nil), 8)
		LogRequest(r.WithContext(context.WithValue(r.Context(), SessionKey, typedNil)), "again")
		lines := strings.Split(strings.TrimSpace(logged.String()), "\n")
		if len(lines) != 2 || !strings.HasSuffix(lines[0], "7 N/A 192.0.2.1:1234 hello") || !strings.HasSuffix(lines[1], "8 N/A 192.0.2.1:1234 again") {
			t.Errorf("log = %q", logged.String())
		}
	})
}

func TestCreateStack(t *testing.T) {
	t.Run("SV-32 the first middleware listed is the outermost", func(t *testing.T) {
		var order []string
		mark := func(name string) Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					order = append(order, name+" in")
					next.ServeHTTP(w, r)
					order = append(order, name+" out")
				})
			}
		}
		handler := CreateStack(mark("a"), mark("b"), mark("c"))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			order = append(order, "handler")
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		want := "a in,b in,c in,handler,c out,b out,a out"
		if got := strings.Join(order, ","); got != want {
			t.Errorf("order = %s, want %s", got, want)
		}
	})

	t.Run("SV-33 an empty stack gives the handler itself", func(t *testing.T) {
		rec := httptest.NewRecorder()
		CreateStack()(http.HandlerFunc(okHandler)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	t.Run("SV-34 the four headers are set before the handler runs and the handler is called", func(t *testing.T) {
		var during http.Header
		handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			during = w.Header().Clone()
			w.WriteHeader(http.StatusTeapot)
		}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		want := map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "same-origin",
			"Content-Security-Policy": "frame-ancestors 'none'",
			"X-Frame-Options":         "DENY",
		}
		for header, value := range want {
			if during.Get(header) != value || rec.Header().Get(header) != value {
				t.Errorf("%s = %q while handling and %q after, want %q", header, during.Get(header), rec.Header().Get(header), value)
			}
		}
		if rec.Code != http.StatusTeapot {
			t.Errorf("status = %d, want the handler's 418", rec.Code)
		}
	})
}

func TestWrappedWriter(t *testing.T) {
	t.Run("SV-35 WriteHeader records the status and passes it on", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &WrappedWriter{rec, http.StatusOK}
		w.WriteHeader(http.StatusBadGateway)
		if w.statusCode != http.StatusBadGateway || rec.Code != http.StatusBadGateway {
			t.Errorf("recorded %d, passed on %d, want 502 for both", w.statusCode, rec.Code)
		}
	})
}

func TestHeaderTracker(t *testing.T) {
	t.Run("SV-36 a final status, a switch to another protocol or a body starts the response, an informational status does not", func(t *testing.T) {
		cases := map[string]func(w *headerTracker){
			"200":          func(w *headerTracker) { w.WriteHeader(http.StatusOK) },
			"404":          func(w *headerTracker) { w.WriteHeader(http.StatusNotFound) },
			"101":          func(w *headerTracker) { w.WriteHeader(http.StatusSwitchingProtocols) },
			"body":         func(w *headerTracker) { _, _ = w.Write([]byte("x")) },
			"103 then 200": func(w *headerTracker) { w.WriteHeader(http.StatusEarlyHints); w.WriteHeader(http.StatusOK) },
		}
		for name, act := range cases {
			w := &headerTracker{ResponseWriter: httptest.NewRecorder()}
			act(w)
			if !w.started {
				t.Errorf("%s: started = false, want true", name)
			}
		}
		for _, status := range []int{http.StatusContinue, http.StatusEarlyHints} {
			w := &headerTracker{ResponseWriter: httptest.NewRecorder()}
			w.WriteHeader(status)
			if w.started {
				t.Errorf("%d: started = true, want false", status)
			}
		}
	})

	t.Run("SV-37 Unwrap gives the writer underneath", func(t *testing.T) {
		rec := httptest.NewRecorder()
		if got := (&headerTracker{ResponseWriter: rec}).Unwrap(); got != rec {
			t.Errorf("Unwrap = %v, want the wrapped writer", got)
		}
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	panics := func(before func(w http.ResponseWriter)) http.Handler {
		return RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if before != nil {
				before(w)
			}
			panic("broken handler")
		}))
	}

	t.Run("SV-38 a handler that does not panic is left alone and nothing is logged", func(t *testing.T) {
		logged := captureLog(t)
		rec := httptest.NewRecorder()
		RecoveryMiddleware(http.HandlerFunc(okHandler)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusNoContent || logged.Len() != 0 {
			t.Errorf("status %d, log %q, want 204 and no log", rec.Code, logged.String())
		}
	})

	t.Run("SV-39 a panic before anything is written answers a plain 500 and logs the place and the stack, not the request", func(t *testing.T) {
		logged := captureLog(t)
		req := httptest.NewRequest(http.MethodPost, "/boom", strings.NewReader("password=hunter2"))
		req.AddCookie(&http.Cookie{Name: "session", Value: "secret-session-value"})
		rec := httptest.NewRecorder()
		panics(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "Internal Server Error\n" {
			t.Errorf("status %d, body %q, want 500 and Internal Server Error", rec.Code, rec.Body.String())
		}
		for _, want := range []string{"panic serving POST /boom", "broken handler", "goroutine "} {
			if !strings.Contains(logged.String(), want) {
				t.Errorf("the log is missing %q:\n%s", want, logged.String())
			}
		}
		for _, unwanted := range []string{"secret-session-value", "hunter2"} {
			if strings.Contains(logged.String(), unwanted) {
				t.Errorf("the log holds %q", unwanted)
			}
		}
	})

	t.Run("SV-40 the panic log carries the request ID of the request, from the stack or from the header", func(t *testing.T) {
		logged := captureLog(t)
		handler := CreateStack(RecoveryMiddleware, RequestIDMiddleware)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("broken handler")
		}))
		req := httptest.NewRequest(http.MethodGet, "/boom", nil)
		req.Header.Set(XRequestIDHeader, "424242")
		handler.ServeHTTP(httptest.NewRecorder(), req)
		if !strings.Contains(logged.String(), "panic serving GET /boom: request_id=424242 broken handler") {
			t.Errorf("the log line does not carry the request ID:\n%s", logged.String())
		}

		logged.Reset()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))
		if !strings.Contains(logged.String(), "request_id=") || strings.Contains(logged.String(), "request_id=N/A") {
			t.Errorf("the log line does not carry a generated request ID:\n%s", logged.String())
		}
	})

	t.Run("SV-41 a panic after the response started leaves the status and the body as they were", func(t *testing.T) {
		cases := []struct {
			name       string
			before     func(w http.ResponseWriter)
			wantStatus int
			wantBody   string
		}{
			{"after the body started", func(w http.ResponseWriter) { _, _ = w.Write([]byte("partial")) }, http.StatusOK, "partial"},
			{"after the header", func(w http.ResponseWriter) { w.WriteHeader(http.StatusCreated) }, http.StatusCreated, ""},
			{"after a redirect", func(w http.ResponseWriter) { w.Header().Set("HX-Redirect", "/x"); w.WriteHeader(http.StatusOK) }, http.StatusOK, ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				logged := captureLog(t)
				rec := httptest.NewRecorder()
				panics(tc.before).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
				if rec.Code != tc.wantStatus || rec.Body.String() != tc.wantBody {
					t.Errorf("status %d, body %q, want %d and %q", rec.Code, rec.Body.String(), tc.wantStatus, tc.wantBody)
				}
				if !strings.Contains(logged.String(), "panic serving GET /boom") {
					t.Errorf("the panic was not logged:\n%s", logged.String())
				}
			})
		}
	})

	t.Run("SV-42 an informational status before the panic still gets the 500", func(t *testing.T) {
		captureLog(t)
		server := httptest.NewServer(panics(func(w http.ResponseWriter) { w.WriteHeader(http.StatusEarlyHints) }))
		t.Cleanup(server.Close)
		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", resp.StatusCode)
		}
	})

	t.Run("SV-43 an aborted handler panics on and is not logged", func(t *testing.T) {
		logged := captureLog(t)
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			RecoveryMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic(http.ErrAbortHandler)
			})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		}()
		if !errors.Is(recovered.(error), http.ErrAbortHandler) || logged.Len() != 0 {
			t.Errorf("recovered %v, log %q, want http.ErrAbortHandler and no log", recovered, logged.String())
		}
	})
}

func TestRequestIDOf(t *testing.T) {
	t.Run("SV-44 the ID in the context wins, then a numeric header, then N/A", func(t *testing.T) {
		withHeader := httptest.NewRequest(http.MethodGet, "/", nil)
		withHeader.Header.Set(XRequestIDHeader, "99")
		inContext := withHeader.Clone(context.WithValue(withHeader.Context(), RequestIDKey, 5))
		for name, tc := range map[string]struct {
			req  *http.Request
			want any
		}{
			"context": {inContext, 5},
			"header":  {withHeader, 99},
			"nothing": {httptest.NewRequest(http.MethodGet, "/", nil), "N/A"},
			"bad header": {func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Header.Set(XRequestIDHeader, "abc")
				return r
			}(), "N/A"},
		} {
			if got := requestIDOf(tc.req); got != tc.want {
				t.Errorf("%s: requestIDOf = %v, want %v", name, got, tc.want)
			}
		}
	})
}

func TestSessionMiddleware(t *testing.T) {
	secret := useSecret(t)
	seen := func(r *http.Request) (*SessionClaims, bool) {
		claims, ok := r.Context().Value(SessionKey).(*SessionClaims)
		return claims, ok
	}

	t.Run("SV-45 a valid session cookie puts the claims in the context", func(t *testing.T) {
		var claims *SessionClaims
		var ok bool
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: signToken(t, jwt.SigningMethodHS256, secret, validClaims(config.SiteURL))})
		SessionMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { claims, ok = seen(r) })).ServeHTTP(httptest.NewRecorder(), req)
		if !ok || claims.Subject != "user-123" {
			t.Errorf("claims = %+v, %t, want subject user-123", claims, ok)
		}
	})

	t.Run("SV-46 a missing or invalid cookie leaves the context without a session and still calls the handler", func(t *testing.T) {
		for name, cookie := range map[string]*http.Cookie{
			"no cookie":     nil,
			"invalid token": {Name: "session", Value: "garbage"},
			"other cookie":  {Name: "theme", Value: signToken(t, jwt.SigningMethodHS256, secret, validClaims(config.SiteURL))},
		} {
			t.Run(name, func(t *testing.T) {
				called, found := false, false
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if cookie != nil {
					req.AddCookie(cookie)
				}
				SessionMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					called = true
					_, found = seen(r)
				})).ServeHTTP(httptest.NewRecorder(), req)
				if !called || found {
					t.Errorf("called %t, session found %t, want called and no session", called, found)
				}
			})
		}
	})
}

func TestRequestIDMiddleware(t *testing.T) {
	run := func(header string) (id any, after string) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set(XRequestIDHeader, header)
		}
		RequestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			id = r.Context().Value(RequestIDKey)
		})).ServeHTTP(httptest.NewRecorder(), req)
		return id, req.Header.Get(XRequestIDHeader)
	}

	t.Run("SV-47 an ID sent with the request is the ID in the context and the header stays", func(t *testing.T) {
		id, header := run("424242")
		if id != 424242 || header != "424242" {
			t.Errorf("context ID %v, header %q, want 424242 for both", id, header)
		}
	})

	t.Run("SV-48 without an ID one is made, set on the header and put in the context", func(t *testing.T) {
		id, header := run("")
		number, ok := id.(int)
		if !ok || number == 0 || header == "" {
			t.Fatalf("context ID %v, header %q, want a made ID in both", id, header)
		}
		if want := regexp.MustCompile(`^\d+$`); !want.MatchString(header) || header != strconv.Itoa(number) {
			t.Errorf("header %q does not hold the context ID %d", header, number)
		}
	})

	t.Run("SV-49 an ID that is not a number becomes 0 in the context and stays as sent on the header", func(t *testing.T) {
		id, header := run("abc")
		if id != 0 || header != "abc" {
			t.Errorf("context ID %v, header %q, want 0 and abc", id, header)
		}
	})
}

func TestIPMiddleware(t *testing.T) {
	run := func(headers map[string]string) string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for header, value := range headers {
			req.Header.Set(header, value)
		}
		var seen string
		IPMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr })).ServeHTTP(httptest.NewRecorder(), req)
		return seen
	}

	t.Run("SV-50 CF-Connecting-IP replaces the address and wins over X-Forwarded-For", func(t *testing.T) {
		if got := run(map[string]string{CFConnectingIPHeader: "198.51.100.1", XForwardedForHeader: "203.0.113.9"}); got != "198.51.100.1" {
			t.Errorf("RemoteAddr = %q, want 198.51.100.1", got)
		}
	})

	t.Run("SV-51 X-Forwarded-For replaces the address as sent when there is no CF-Connecting-IP", func(t *testing.T) {
		if got := run(map[string]string{XForwardedForHeader: "203.0.113.9, 10.0.0.1"}); got != "203.0.113.9, 10.0.0.1" {
			t.Errorf("RemoteAddr = %q, want the header as sent", got)
		}
	})

	t.Run("SV-52 without either header the address stays", func(t *testing.T) {
		if got := run(nil); got != "192.0.2.1:1234" {
			t.Errorf("RemoteAddr = %q, want 192.0.2.1:1234", got)
		}
	})
}

func TestRequestLoggerMiddleware(t *testing.T) {
	serve := func(t *testing.T, handler http.HandlerFunc, target string) string {
		logged := captureLog(t)
		inner := RequestIDMiddleware(RequestLoggerMiddleware(handler))
		inner.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
		return logged.String()
	}
	line := regexp.MustCompile(`^\S+ \S+ \d+ N/A 192\.0\.2\.1:1234 (\d+) (\S+) (\S+) \S+\n$`)

	t.Run("SV-53 one line per request holds the status the handler set, the method, the path and the time taken", func(t *testing.T) {
		got := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }, "/x")
		match := line.FindStringSubmatch(got)
		if match == nil || match[1] != "418" || match[2] != "GET" || match[3] != "/x" {
			t.Errorf("log = %q, want one line with 418 GET /x and a duration", got)
		}
	})

	t.Run("SV-54 a handler that never sets a status is logged as 200", func(t *testing.T) {
		got := serve(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("body")) }, "/x")
		if match := line.FindStringSubmatch(got); match == nil || match[1] != "200" {
			t.Errorf("log = %q, want status 200", got)
		}
	})

	t.Run("SV-55 the path is logged without its query and an error answer is logged with its status", func(t *testing.T) {
		got := serve(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "gone", http.StatusGone) }, "/x/y?token=secret")
		match := line.FindStringSubmatch(got)
		if match == nil || match[1] != "410" || match[3] != "/x/y" || strings.Contains(got, "secret") {
			t.Errorf("log = %q, want 410 /x/y without the query", got)
		}
	})
}
