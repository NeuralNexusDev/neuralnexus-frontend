package middleware

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//goland:noinspection GoSnakeCaseUsage
var (
	NN_API_URL     = os.Getenv("NN_API_URL")
	NN_SITE_URL    = os.Getenv("NN_SITE_URL")
	JWT_SECRET     = []byte(os.Getenv("JWT_SECRET"))
	validAudiences = []string{NN_SITE_URL, NN_API_URL}
)

// WrappedWriter - Wrapper for http.ResponseWriter
type WrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader - Write the header
func (w *WrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

// Middleware - Middleware type
type Middleware func(http.Handler) http.Handler

// key - Type for context keys
type key int

const (
	// SessionKey - Key for session in context
	SessionKey key = iota
	// RequestIDKey - Key for request ID in context
	RequestIDKey
)

const (
	XRequestIDHeader     = "X-Request-ID"
	XForwardedForHeader  = "X-Forwarded-For"
	CFConnectingIPHeader = "CF-Connecting-IP"
)

// SessionClaims custom JWT claims for session
type SessionClaims struct {
	Scope []string `json:"scope"`
	jwt.RegisteredClaims
}

// SessionFromJWT reads the user ID from the JWT token
func SessionFromJWT(tokenStr string) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		return JWT_SECRET, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*SessionClaims); ok {
		// Validate audience
		for _, aud := range claims.Audience {
			valid := false
			for _, validAud := range validAudiences {
				if aud == validAud {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("invalid audience: %s", aud)
			}
		}

		return claims, nil
	} else {
		return nil, errors.New("invalid token claims")
	}
}

func LogRequest(r *http.Request, message ...string) {
	requestId := r.Context().Value(RequestIDKey).(int)
	userId := "N/A"
	if session, ok := r.Context().Value(SessionKey).(*SessionClaims); ok && session != nil {
		userId = session.Subject
	}

	log.Printf("%d %s %s %s",
		requestId,
		userId,
		r.RemoteAddr,
		strings.Join(message, " "),
	)
}

// CreateStack - Create a stack of middlewares
func CreateStack(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// SessionMiddleware - Read the session cookie from the request
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err == nil {
			session, err := SessionFromJWT(cookie.Value)
			if err != nil {
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, SessionKey, session)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// RequestIDMiddleware - Set the request ID in the context
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIdStr := r.Header.Get(XRequestIDHeader)
		var requestId int
		if requestIdStr == "" {
			requestId = int(time.Now().UnixNano())
			r.Header.Set(XRequestIDHeader, strconv.Itoa(requestId))
		} else {
			requestId, _ = strconv.Atoi(requestIdStr)
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestIDKey, requestId)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// IPMiddleware - Update the remote address based on headers
func IPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfConnectingIP := r.Header.Get(CFConnectingIPHeader)
		forwardedFor := r.Header.Get(XForwardedForHeader)
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		next.ServeHTTP(w, r)
	})
}

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &WrappedWriter{w, http.StatusOK}

		next.ServeHTTP(wrapped, r)

		LogRequest(r, fmt.Sprintf("%d %s %s %s",
			wrapped.statusCode,
			r.Method,
			r.URL.Path,
			time.Since(start),
		))
	})
}
