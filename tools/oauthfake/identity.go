package main

import (
	"hash/fnv"
	"strings"
)

// Identity is the fake end user who approves an authorization request.
type Identity struct {
	// Login is the stable handle that identifies the user across logins.
	Login string
	// Email is the user's email address.
	Email string
	// Name is the user's display name.
	Name string
}

// newIdentity builds an Identity, filling in defaults for empty email and name.
// It returns false if login is empty after trimming.
func newIdentity(login, email, name string) (Identity, bool) {
	login = strings.TrimSpace(login)
	if login == "" {
		return Identity{}, false
	}
	email = strings.TrimSpace(email)
	if email == "" {
		email = login + "@example.com"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = login
	}
	return Identity{Login: login, Email: email, Name: name}, true
}

// googleSub returns a stable Google subject identifier for the identity.
func (i Identity) googleSub() string {
	return "fake-google-" + i.Login
}

// githubID returns a stable, positive numeric GitHub user ID for the identity.
// It is masked to 53 bits so that JavaScript clients (e2e tests) can represent
// it exactly.
func (i Identity) githubID() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(i.Login))
	return int64(h.Sum64() & (1<<53 - 1))
}
