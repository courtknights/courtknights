package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// deviceStatus is the state of a device authorization request.
type deviceStatus int

const (
	devicePending deviceStatus = iota
	deviceApproved
	deviceDenied
)

// authCode is a one-time authorization code issued by the redirect flow.
type authCode struct {
	provider    string
	redirectURI string
	identity    Identity
	expiresAt   time.Time
}

// deviceAuth is a pending, approved or denied device authorization request.
type deviceAuth struct {
	provider  string
	userCode  string
	status    deviceStatus
	identity  Identity
	expiresAt time.Time
	interval  time.Duration
	lastPoll  time.Time
	redeemed  bool
}

// accessToken is a bearer token accepted by the userinfo endpoints.
type accessToken struct {
	provider string
	identity Identity
}

// Errors returned by store operations. They map to RFC 6749 / RFC 8628 error codes.
var (
	errInvalidGrant  = errors.New("invalid_grant")
	errPending       = errors.New("authorization_pending")
	errSlowDown      = errors.New("slow_down")
	errAccessDenied  = errors.New("access_denied")
	errExpiredToken  = errors.New("expired_token")
	errUnknownCode   = errors.New("unknown user code")
	errNotPending    = errors.New("device request is not pending")
	errInvalidBearer = errors.New("invalid access token")
)

// userCodeAlphabet excludes vowels and look-alike characters, as recommended by RFC 8628 §6.1.
const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

// store holds all codes and tokens in memory. It is safe for concurrent use.
type store struct {
	mu        sync.Mutex
	now       func() time.Time
	codeTTL   time.Duration
	interval  time.Duration
	codes     map[string]*authCode
	devices   map[string]*deviceAuth // keyed by device code
	userCodes map[string]string      // user code → device code
	tokens    map[string]accessToken
}

// newStore returns an empty store. now is injectable for tests.
func newStore(now func() time.Time, codeTTL, interval time.Duration) *store {
	return &store{
		now:       now,
		codeTTL:   codeTTL,
		interval:  interval,
		codes:     map[string]*authCode{},
		devices:   map[string]*deviceAuth{},
		userCodes: map[string]string{},
		tokens:    map[string]accessToken{},
	}
}

// issueCode creates a one-time authorization code for the redirect flow.
func (s *store) issueCode(provider, redirectURI string, id Identity) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	code := randomHex(16)
	s.codes[code] = &authCode{provider: provider, redirectURI: redirectURI, identity: id, expiresAt: s.now().Add(s.codeTTL)}
	return code
}

// redeemCode exchanges an authorization code for an access token. Codes are single-use.
// redirectURI is checked only when the client sends one.
func (s *store) redeemCode(provider, code, redirectURI string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[code]
	if !ok || c.provider != provider {
		return "", errInvalidGrant
	}
	delete(s.codes, code)
	if s.now().After(c.expiresAt) || (redirectURI != "" && redirectURI != c.redirectURI) {
		return "", errInvalidGrant
	}
	return s.issueTokenLocked(provider, c.identity), nil
}

// deviceStart is the data returned to the client when a device request is created.
type deviceStart struct {
	deviceCode string
	userCode   string
	expiresIn  time.Duration
	interval   time.Duration
}

// startDevice creates a pending device authorization request.
func (s *store) startDevice(provider string) deviceStart {
	s.mu.Lock()
	defer s.mu.Unlock()
	deviceCode := randomHex(20)
	userCode := s.newUserCodeLocked()
	s.devices[deviceCode] = &deviceAuth{
		provider:  provider,
		userCode:  userCode,
		status:    devicePending,
		expiresAt: s.now().Add(s.codeTTL),
		interval:  s.interval,
	}
	s.userCodes[userCode] = deviceCode
	return deviceStart{deviceCode: deviceCode, userCode: userCode, expiresIn: s.codeTTL, interval: s.interval}
}

// resolveDevice approves or denies the pending request identified by userCode.
func (s *store) resolveDevice(provider, userCode string, approve bool, id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deviceCode, ok := s.userCodes[normalizeUserCode(userCode)]
	if !ok {
		return errUnknownCode
	}
	d := s.devices[deviceCode]
	if d.provider != provider {
		return errUnknownCode
	}
	if d.status != devicePending || s.now().After(d.expiresAt) {
		return errNotPending
	}
	if approve {
		d.status = deviceApproved
		d.identity = id
	} else {
		d.status = deviceDenied
	}
	return nil
}

// pollDevice implements one device-code token request (RFC 8628 §3.4–3.5).
func (s *store) pollDevice(provider, deviceCode string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[deviceCode]
	if !ok || d.provider != provider || d.redeemed {
		return "", errInvalidGrant
	}
	now := s.now()
	if now.After(d.expiresAt) {
		return "", errExpiredToken
	}
	tooFast := !d.lastPoll.IsZero() && now.Sub(d.lastPoll) < d.interval
	d.lastPoll = now
	switch d.status {
	case deviceDenied:
		return "", errAccessDenied
	case deviceApproved:
		d.redeemed = true
		return s.issueTokenLocked(provider, d.identity), nil
	}
	if tooFast {
		// RFC 8628 §3.5: the client must add 5 seconds to its interval.
		d.interval += 5 * time.Second
		return "", errSlowDown
	}
	return "", errPending
}

// lookupToken returns the identity behind a bearer token issued for provider.
func (s *store) lookupToken(provider, token string) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[token]
	if !ok || t.provider != provider {
		return Identity{}, errInvalidBearer
	}
	return t.identity, nil
}

func (s *store) issueTokenLocked(provider string, id Identity) string {
	token := randomHex(24)
	s.tokens[token] = accessToken{provider: provider, identity: id}
	return token
}

func (s *store) newUserCodeLocked() string {
	for {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		code := make([]byte, 0, 9)
		for i, v := range b {
			if i == 4 {
				code = append(code, '-')
			}
			code = append(code, userCodeAlphabet[int(v)%len(userCodeAlphabet)])
		}
		if _, taken := s.userCodes[string(code)]; !taken {
			return string(code)
		}
	}
}

// normalizeUserCode upper-cases the code and restores the dash, so that users
// may type "bcdf ghjk", "BCDFGHJK" or "BCDF-GHJK".
func normalizeUserCode(code string) string {
	var b []byte
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z':
			b = append(b, byte(r-'a'+'A'))
		case r >= 'A' && r <= 'Z':
			b = append(b, byte(r))
		}
	}
	if len(b) == 8 {
		return string(b[:4]) + "-" + string(b[4:])
	}
	return string(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
