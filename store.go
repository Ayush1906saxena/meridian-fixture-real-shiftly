package main

import "sync"

type User struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"-"`
	Role     string `json:"role"`
	TeamID   int    `json:"team_id"`
}

type Shift struct {
	ID     int    `json:"id"`
	TeamID int    `json:"team_id"`
	UserID int    `json:"user_id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Note   string `json:"note"`
}

// Store keeps everything in memory, behind one lock.
type Store struct {
	mu     sync.Mutex
	users  map[int]*User
	shifts map[int]*Shift
	nextID int
}

func NewStore() *Store {
	s := &Store{users: map[int]*User{}, shifts: map[int]*Shift{}, nextID: 100}
	// Demo data: a manager and an employee in team 1, and someone in another company's team 2.
	s.users[1] = &User{ID: 1, Email: "mia@shiftly.test", Name: "Mia", Password: "mia-pass-1", Role: "manager", TeamID: 1}
	s.users[2] = &User{ID: 2, Email: "leo@shiftly.test", Name: "Leo", Password: "leo-pass-2", Role: "employee", TeamID: 1}
	s.users[3] = &User{ID: 3, Email: "zoe@other.test", Name: "Zoe", Password: "zoe-pass-3", Role: "employee", TeamID: 2}
	s.shifts[1] = &Shift{ID: 1, TeamID: 1, UserID: 2, Start: "2026-11-02T09:00", End: "2026-11-02T17:00", Note: "Till 2"}
	s.shifts[2] = &Shift{ID: 2, TeamID: 2, UserID: 3, Start: "2026-11-02T06:00", End: "2026-11-02T14:00", Note: "Opening, alarm code 4471"}
	return s
}

func (s *Store) newID() int {
	s.nextID++
	return s.nextID
}

func (s *Store) userByEmail(email string) *User {
	for _, u := range s.users {
		if u.Email == email {
			return u
		}
	}
	return nil
}
