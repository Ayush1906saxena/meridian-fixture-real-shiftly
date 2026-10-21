package main

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// A session token names the user it belongs to.
func tokenFor(u *User) string {
	return base64.StdEncoding.EncodeToString([]byte("uid:" + strconv.Itoa(u.ID)))
}

func userIDFromToken(token string) (int, bool) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil || !strings.HasPrefix(string(raw), "uid:") {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(string(raw), "uid:"))
	return id, err == nil
}
