package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// client is a minimal REST client for the 智御 API envelope
// {code, message, data}. It re-logs in once on HTTP 401.
type client struct {
	base     string
	username string
	password string
	http     *http.Client
	log      *slog.Logger

	mu    sync.Mutex
	token string
}

// apiError is a non-2xx response or a non-zero envelope code.
type apiError struct {
	Method  string
	Path    string
	Status  int
	Code    int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d code=%d %s", e.Method, e.Path, e.Status, e.Code, e.Message)
}

// isUnavailable reports a route that does not exist yet (module not implemented).
func (e *apiError) isUnavailable() bool {
	return e.Status == http.StatusNotImplemented || (e.Status == http.StatusNotFound && strings.Contains(e.Message, "route not found"))
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newClient(base, username, password string, log *slog.Logger) *client {
	return &client{
		base: strings.TrimRight(base, "/"), username: username, password: password, log: log,
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *client) login(ctx context.Context) error {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	body := map[string]string{"username": c.username, "password": c.password}
	if err := c.call(ctx, http.MethodPost, "/auth/login", body, nil, &out); err != nil {
		return err
	}
	c.mu.Lock()
	c.token = out.AccessToken
	c.mu.Unlock()
	return nil
}

func (c *client) bearer() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return "Bearer " + c.token
}

// do performs an admin (JWT) request; a 401 triggers one re-login and retry.
func (c *client) do(ctx context.Context, method, path string, body, out any) error {
	err := c.call(ctx, method, path, body, map[string]string{"Authorization": c.bearer()}, out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		c.log.Info("access token rejected, logging in again")
		if lerr := c.login(ctx); lerr != nil {
			return lerr
		}
		return c.call(ctx, method, path, body, map[string]string{"Authorization": c.bearer()}, out)
	}
	return err
}

// doDevice performs a gateway request authenticated with the device key.
func (c *client) doDevice(ctx context.Context, serial, key, method, path string, body, out any) error {
	return c.call(ctx, method, path, body, map[string]string{"X-Device-Serial": serial, "X-Device-Key": key}, out)
}

func (c *client) call(ctx context.Context, method, path string, body any, headers map[string]string, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "zhiyuche-simulator/VIG-100E")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var env envelope
	_ = json.Unmarshal(raw, &env)
	if resp.StatusCode >= 300 || env.Code != 0 {
		msg := env.Message
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		return &apiError{Method: method, Path: path, Status: resp.StatusCode, Code: env.Code, Message: msg}
	}
	if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}
