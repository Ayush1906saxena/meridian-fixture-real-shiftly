package main

// The happy paths: sign up, log in, see your team's rota, a manager plans a shift, download the schedule.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type client struct {
	t    *testing.T
	base string
}

func newClient(t *testing.T) *client {
	t.Setenv("SHIFTLY_SECRET", "visible-tests-secret")
	srv := httptest.NewServer(NewApp(NewStore()).Handler())
	t.Cleanup(srv.Close)
	return &client{t: t, base: srv.URL}
}

func (c *client) call(method, path string, body any, token string) (int, map[string]any, string) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out, string(raw)
}

func (c *client) login(email, password string) string {
	status, out, raw := c.call("POST", "/api/login", map[string]string{"email": email, "password": password}, "")
	if status != 200 {
		c.t.Fatalf("%s should log in: %d %s", email, status, raw)
	}
	return out["token"].(string)
}

func TestSignUpLogInAndSeeTheRota(t *testing.T) {
	c := newClient(t)
	status, out, _ := c.call("POST", "/api/signup", map[string]string{"email": "ivy@shiftly.test", "name": "Ivy", "password": "ivy-pass-9"}, "")
	if status != 201 || out["user"].(map[string]any)["role"] != "employee" {
		t.Fatalf("signup: %d %v", status, out)
	}
	token := c.login("ivy@shiftly.test", "ivy-pass-9")
	status, out, _ = c.call("GET", "/api/shifts", nil, token)
	if status != 200 || len(out["shifts"].([]any)) != 1 {
		t.Fatalf("team 1's rota: %d %v", status, out)
	}
	status, out, _ = c.call("PATCH", "/api/me", map[string]string{"name": "Ivy B"}, token)
	if status != 200 || out["user"].(map[string]any)["name"] != "Ivy B" {
		t.Fatalf("rename: %d %v", status, out)
	}
	if status, _, _ := c.call("POST", "/api/login", map[string]string{"email": "ivy@shiftly.test", "password": "nope"}, ""); status != 401 {
		t.Fatalf("a wrong password logged in: %d", status)
	}
}

func TestAManagerPlansAShiftAndTheEmployeeSeesIt(t *testing.T) {
	c := newClient(t)
	mia := c.login("mia@shiftly.test", "mia-pass-1")
	status, out, raw := c.call("POST", "/api/shifts", map[string]any{"user_id": 2, "start": "2026-11-03T09:00", "end": "2026-11-03T17:00", "note": "Stockroom"}, mia)
	if status != 201 {
		t.Fatalf("create: %d %s", status, raw)
	}
	id := int(out["shift"].(map[string]any)["id"].(float64))
	leo := c.login("leo@shiftly.test", "leo-pass-2")
	status, out, _ = c.call("GET", "/api/shifts/"+itoa(id), nil, leo)
	if status != 200 || out["shift"].(map[string]any)["note"] != "Stockroom" {
		t.Fatalf("leo's new shift: %d %v", status, out)
	}
	if status, _, _ := c.call("GET", "/api/shifts/1", nil, leo); status != 200 {
		t.Fatalf("leo's own shift: %d", status)
	}
}

func TestTheScheduleDownloads(t *testing.T) {
	c := newClient(t)
	leo := c.login("leo@shiftly.test", "leo-pass-2")
	status, _, raw := c.call("GET", "/files?name=schedule.csv", nil, leo)
	if status != 200 || !bytes.Contains([]byte(raw), []byte("Leo")) {
		t.Fatalf("schedule: %d %s", status, raw)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
