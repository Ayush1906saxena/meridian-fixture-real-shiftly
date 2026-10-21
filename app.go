package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// App serves Shiftly's API over a Store.
type App struct {
	store   *Store
	exports string
	client  *http.Client
}

func NewApp(store *Store) *App {
	return &App{store: store, exports: "exports", client: &http.Client{}}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/signup", a.signup)
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("GET /api/me", a.auth(a.me))
	mux.HandleFunc("PATCH /api/me", a.auth(a.updateMe))
	mux.HandleFunc("GET /api/shifts", a.auth(a.listShifts))
	mux.HandleFunc("GET /api/shifts/{id}", a.auth(a.getShift))
	mux.HandleFunc("POST /api/shifts", a.auth(a.createShift))
	mux.HandleFunc("GET /files", a.auth(a.download))
	mux.HandleFunc("POST /api/webhooks/test", a.auth(a.testWebhook))
	return cors(mux)
}

// cors lets the web front end, wherever it is hosted, call the API with the user's credentials.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		next.ServeHTTP(w, r)
	})
}

type handler func(w http.ResponseWriter, r *http.Request, u *User)

func (a *App) auth(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDFromToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		a.store.mu.Lock()
		u := a.store.users[id]
		a.store.mu.Unlock()
		if !ok || u == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "please log in"})
			return
		}
		h(w, r, u)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readJSON(r *http.Request, into any) bool {
	return json.NewDecoder(r.Body).Decode(into) == nil
}

// ---- accounts

func (a *App) signup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Name, Password string }
	if !readJSON(r, &in) || in.Email == "" || in.Name == "" || in.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, name and password are required"})
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	if a.store.userByEmail(in.Email) != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "that email already has an account"})
		return
	}
	u := &User{ID: a.store.newID(), Email: in.Email, Name: in.Name, Password: in.Password, Role: "employee", TeamID: 1}
	a.store.users[u.ID] = u
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "token": tokenFor(u)})
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	readJSON(r, &in)
	a.store.mu.Lock()
	u := a.store.userByEmail(in.Email)
	a.store.mu.Unlock()
	if u == nil || u.Password != in.Password {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong email or password"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "token": tokenFor(u)})
}

func (a *App) me(w http.ResponseWriter, r *http.Request, u *User) {
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (a *App) updateMe(w http.ResponseWriter, r *http.Request, u *User) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	// Decoding into the user itself updates whatever fields were sent.
	if !readJSON(r, u) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

// ---- shifts

func (a *App) listShifts(w http.ResponseWriter, r *http.Request, u *User) {
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	out := []*Shift{}
	for id := 1; id <= a.store.nextID; id++ {
		if s := a.store.shifts[id]; s != nil && s.TeamID == u.TeamID {
			out = append(out, s)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"shifts": out})
}

func (a *App) getShift(w http.ResponseWriter, r *http.Request, u *User) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	a.store.mu.Lock()
	s := a.store.shifts[id]
	a.store.mu.Unlock()
	if s == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such shift"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shift": s})
}

func (a *App) createShift(w http.ResponseWriter, r *http.Request, u *User) {
	var in struct {
		UserID     int    `json:"user_id"`
		Start, End string
		Note       string `json:"note"`
	}
	if !readJSON(r, &in) || in.Start == "" || in.End == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id, start and end are required"})
		return
	}
	a.store.mu.Lock()
	defer a.store.mu.Unlock()
	worker := a.store.users[in.UserID]
	if worker == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no such user"})
		return
	}
	s := &Shift{ID: a.store.newID(), TeamID: worker.TeamID, UserID: worker.ID, Start: in.Start, End: in.End, Note: in.Note}
	a.store.shifts[s.ID] = s
	writeJSON(w, http.StatusCreated, map[string]any{"shift": s})
}

// ---- exports and webhooks

func (a *App) download(w http.ResponseWriter, r *http.Request, u *User) {
	path := filepath.Join(a.exports, r.URL.Query().Get("name"))
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	_, _ = w.Write(data)
}

func (a *App) testWebhook(w http.ResponseWriter, r *http.Request, u *User) {
	var in struct{ URL string }
	if !readJSON(r, &in) || in.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	res, err := a.client.Post(in.URL, "application/json", strings.NewReader(`{"text":"Shiftly test message"}`))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	res.Body.Close()
	writeJSON(w, http.StatusOK, map[string]any{"delivered": true, "status": res.StatusCode})
}
