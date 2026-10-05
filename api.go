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
	maxAPIResponse    = 1 << 20
	apiTimeout        = 15 * time.Second
)

var apiClient = &http.Client{Timeout: apiTimeout}

var errUnauthorized = errors.New("not signed in")

const (
	loadRolesFailed       = "Failed to load roles"
	loadPermissionsFailed = "Failed to load permissions"
	loadYourPermsFailed   = "Failed to load your permissions"
	unknownPlatform       = "Unknown platform"
)

type apiError struct {
	Status  int
	Message string
	Method  string
	Path    string
	Err     error
}

func (e *apiError) Error() string {
	return e.Message
}

func (e *apiError) Unwrap() error {
	return e.Err
}

func invalidInput(message string) error {
	return &apiError{Status: http.StatusBadRequest, Message: message}
}

type apiSession struct {
	r *http.Request
}

func (a apiSession) call(method string, path string, body any, fallback string) ([]byte, error) {
	fail := func(status int, cause error) error {
		return &apiError{Status: status, Message: fallback, Method: method, Path: path, Err: cause}
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fail(http.StatusInternalServerError, err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(a.r.Context(), method, config.APIURL+"/api/v1"+path, reader)
	if err != nil {
		return nil, fail(http.StatusBadGateway, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie, err := a.r.Cookie(sessionCookieName); err == nil {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie.Value})
	}
	res, err := apiClient.Do(req)
	if err != nil {
		return nil, fail(http.StatusBadGateway, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxAPIResponse))
	if err != nil {
		return nil, fail(http.StatusBadGateway, err)
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
		return nil, &apiError{Status: res.StatusCode, Message: message, Method: method, Path: path}
	}
	return data, nil
}

func apiGet[T any](a apiSession, path string, fallback string) (T, error) {
	var out T
	data, err := a.call(http.MethodGet, path, nil, fallback)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, &apiError{Status: http.StatusBadGateway, Message: fallback, Method: http.MethodGet, Path: path, Err: err}
	}
	return out, nil
}

func apiSend[T any](a apiSession, method string, path string, body any, fallback string) (T, error) {
	var out T
	data, err := a.call(method, path, body, fallback)
	if err != nil {
		return out, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, &apiError{Status: http.StatusBadGateway, Message: fallback, Method: method, Path: path, Err: err}
	}
	return out, nil
}
