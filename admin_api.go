package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

const (
	sessionCookieName = "session"
	maxAdminResponse  = 1 << 20
	adminAPITimeout   = 15 * time.Second
)

var adminClient = &http.Client{Timeout: adminAPITimeout}

var errUnauthorized = errors.New("not signed in")

// adminError is a failed call with the status and the message to show for it.
type adminError struct {
	Status  int
	Message string
}

func (e *adminError) Error() string {
	return e.Message
}

func invalidInput(message string) error {
	return &adminError{Status: http.StatusBadRequest, Message: message}
}

// adminAPI calls nn-api as the signed-in user by forwarding the session cookie of the request.
type adminAPI struct {
	r *http.Request
}

func (a adminAPI) call(method string, path string, body any, fallback string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, &adminError{Status: http.StatusInternalServerError, Message: fallback}
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(a.r.Context(), method, config.APIURL+"/api/v1"+path, reader)
	if err != nil {
		return nil, &adminError{Status: http.StatusBadGateway, Message: fallback}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie, err := a.r.Cookie(sessionCookieName); err == nil {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie.Value})
	}
	res, err := adminClient.Do(req)
	if err != nil {
		return nil, &adminError{Status: http.StatusBadGateway, Message: fallback}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxAdminResponse))
	if err != nil {
		return nil, &adminError{Status: http.StatusBadGateway, Message: fallback}
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if res.StatusCode >= http.StatusBadRequest {
		var problem struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(data, &problem)
		message := problem.Detail
		if message == "" {
			message = fallback
		}
		return nil, &adminError{Status: res.StatusCode, Message: message}
	}
	return data, nil
}

func adminGet[T any](a adminAPI, path string, fallback string) (T, error) {
	var out T
	data, err := a.call(http.MethodGet, path, nil, fallback)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, &adminError{Status: http.StatusBadGateway, Message: fallback}
	}
	return out, nil
}

func adminSend[T any](a adminAPI, method string, path string, body any, fallback string) (T, error) {
	var out T
	data, err := a.call(method, path, body, fallback)
	if err != nil {
		return out, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, &adminError{Status: http.StatusBadGateway, Message: fallback}
	}
	return out, nil
}
