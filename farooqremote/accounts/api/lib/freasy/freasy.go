// Package freasy - Farooq Remote (FA-020): «easy access», like TeamViewer's (owner 1 Nov 2026: «if i already use app
// login password for both side all the devices which visible under that account it should not ask password there
// should extra option in password windows for this feature»).
//
// How it works:
//   - On the computer to be reached, its owner (signed in to his account) ticks «Easy access» once. The app keeps, in
//     that computer's own settings, the account (user name) chosen at that moment, and its heartbeat says
//     «fr_easy: true» to this server every 15 seconds.
//   - When the owner connects from another computer signed in to the same account and has no saved password, that app
//     asks this server for a ticket (POST /api/fr/easy/ticket, signed in). A ticket is given only for a computer that
//     announced easy access in the last 3 minutes and that is not bound to another account; it is 32 random bytes,
//     valid for 60 seconds, for one use, and records the account, the asking computer's ID and the target's ID.
//   - The ticket goes to the target inside the end-to-end encrypted session (LoginRequest.fr_ticket). The target asks
//     this server about it (POST /api/fr/easy/check); the server answers once, with the account and the asking
//     computer's ID, only when the ticket is for that target and not expired. The target lets the session in only when
//     that account is the one stored in its own settings and the ID is the one that connected; otherwise it asks for the
//     password as before.
//
// So a stranger needs the owner's signed-in account; the target computer itself makes the final decision. Standard
// library only.
package freasy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	TicketTTL    = 60 * time.Second // a ticket must be used within a minute
	AnnounceTTL  = 3 * time.Minute  // a heartbeat with fr_easy keeps easy access «on» for 3 minutes
	IssueWindow  = 10 * time.Minute // rate limit window for tickets per account
	IssueMax     = 60               // tickets per account per window
	maxIDLen     = 32
	ticketBytes  = 32
	ticketHexLen = 2 * ticketBytes
)

// ErrTooMany is returned when an account asked for too many tickets.
var ErrTooMany = errors.New("too many easy-access tickets")

// ErrBad is returned for an empty or malformed request.
var ErrBad = errors.New("bad easy-access request")

type ticket struct {
	user string
	uid  uint
	from string
	to   string
	exp  time.Time
}

// Store keeps the tickets, the announcements and the rate limit in memory (one server process).
type Store struct {
	mu        sync.Mutex
	tickets   map[string]ticket
	announced map[string]time.Time
	issued    map[uint][]time.Time
}

// Default is the store used by the server.
var Default = NewStore()

// NewStore makes an empty store.
func NewStore() *Store {
	return &Store{tickets: map[string]ticket{}, announced: map[string]time.Time{}, issued: map[uint][]time.Time{}}
}

func goodID(id string) bool {
	if id == "" || len(id) > maxIDLen {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// clean removes what has expired (called under the lock).
func (s *Store) clean(now time.Time) {
	for k, t := range s.tickets {
		if !now.Before(t.exp) {
			delete(s.tickets, k)
		}
	}
	for k, at := range s.announced {
		if now.Sub(at) >= AnnounceTTL {
			delete(s.announced, k)
		}
	}
	for uid, ts := range s.issued {
		keep := ts[:0]
		for _, t := range ts {
			if now.Sub(t) < IssueWindow {
				keep = append(keep, t)
			}
		}
		if len(keep) == 0 {
			delete(s.issued, uid)
		} else {
			s.issued[uid] = keep
		}
	}
}

// Announce records a heartbeat of computer id with or without easy access.
func (s *Store) Announce(id string, on bool, now time.Time) {
	if !goodID(id) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if on {
		s.announced[id] = now
	} else {
		delete(s.announced, id)
	}
}

// Announced says whether computer id announced easy access in the last AnnounceTTL.
func (s *Store) Announced(id string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clean(now)
	_, ok := s.announced[id]
	return ok
}

// Issue makes a one-time ticket for account uid/user, from computer `from` to computer `to`.
func (s *Store) Issue(uid uint, user, from, to string, now time.Time) (string, error) {
	user = strings.ToLower(strings.TrimSpace(user))
	if uid == 0 || user == "" || !goodID(from) || !goodID(to) || from == to {
		return "", ErrBad
	}
	b := make([]byte, ticketBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clean(now)
	if len(s.issued[uid]) >= IssueMax {
		return "", ErrTooMany
	}
	s.issued[uid] = append(s.issued[uid], now)
	s.tickets[t] = ticket{user: user, uid: uid, from: from, to: to, exp: now.Add(TicketTTL)}
	return t, nil
}

// Check answers once for ticket t presented to computer id: the account (user name) and the computer it was given to.
// The ticket is removed whatever the answer, so it can never be tried twice.
func (s *Store) Check(t, id string, now time.Time) (user, from string, ok bool) {
	if len(t) != ticketHexLen || !goodID(id) {
		return "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tk, found := s.tickets[t]
	delete(s.tickets, t)
	if !found || !now.Before(tk.exp) || tk.to != id {
		return "", "", false
	}
	return tk.user, tk.from, true
}
