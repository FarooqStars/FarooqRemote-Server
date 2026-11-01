// Package freasy - Farooq Remote (FA-020): «easy access», like TeamViewer's (owner 1 Nov 2026: «if i already use app
// login password for both side all the devices which visible under that account it should not ask password there
// should extra option in password windows for this feature»).
//
// How it works:
//   - On the computer to be reached, its owner (signed in to his account) ticks «Easy access» once. The app keeps, in
//     that computer's own settings, the account (user name) chosen at that moment, and its heartbeat says
//     «fr_easy: true» to this server every 15 seconds.
//   - When the owner connects from another computer signed in to the same account and has no saved password, that app
//     asks this server for a ticket (POST /api/fr/easy/ticket, signed in), on an end-to-end encrypted connection only,
//     once per connection, with the key of the computer it reaches (checked through the ID server). A ticket is given
//     only for a computer that announced easy access in the last 3 minutes and that is signed in to the same account;
//     it is 32 random bytes, valid for 60 seconds, for one use, and records the account, the asking computer's ID, the
//     target's ID and that key.
//   - The ticket goes to the target inside the end-to-end encrypted session (LoginRequest.fr_ticket). The target asks
//     this server about it (POST /api/fr/easy/check); the server answers once, with the account, the asking computer's
//     ID and the key, only when the ticket is for that target and not expired. The target lets the session in only when
//     that account is the one stored in its own settings, the ID is the one that connected and the key is its own;
//     otherwise it asks for the password as before.
//
// 2.7-10 (security review of 2.7-9, before it was given to anyone): the key in the ticket, the target must be signed in
// to the asking account, every ticket request counts in the limit, and the memory tables have caps.
//
// So a stranger needs the owner's signed-in account; the target computer itself makes the final decision. Standard
// library only.
package freasy

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	TicketTTL    = 60 * time.Second // a ticket must be used within a minute
	AnnounceTTL  = 3 * time.Minute  // a heartbeat with fr_easy keeps easy access «on» for 3 minutes
	IssueWindow  = 10 * time.Minute // rate limit window for ticket requests per account
	IssueMax     = 60               // ticket requests per account per window - every request counts, given or not
	MaxAnnounced = 20000            // computers remembered with easy access on
	MaxTickets   = 20000            // tickets waiting to be used
	cleanEvery   = time.Minute      // expired entries are removed at least this often
	maxIDLen     = 32
	ticketBytes  = 32
	ticketHexLen = 2 * ticketBytes
	pkBytes      = 32
)

// ErrTooMany is returned when too many tickets are waiting.
var ErrTooMany = errors.New("too many easy-access tickets")

// ErrBad is returned for an empty or malformed request.
var ErrBad = errors.New("bad easy-access request")

type ticket struct {
	user string
	uid  uint
	from string
	to   string
	pk   string
	exp  time.Time
}

// Store keeps the tickets, the announcements and the rate limit in memory (one server process).
type Store struct {
	mu        sync.Mutex
	tickets   map[string]ticket
	announced map[string]time.Time
	requests  map[uint][]time.Time
	lastClean time.Time
}

// Default is the store used by the server.
var Default = NewStore()

// NewStore makes an empty store.
func NewStore() *Store {
	return &Store{tickets: map[string]ticket{}, announced: map[string]time.Time{}, requests: map[uint][]time.Time{}}
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

// goodPK: the standard base64 (with padding) of a 32-byte key, as the app writes it.
func goodPK(pk string) bool {
	if len(pk) != base64.StdEncoding.EncodedLen(pkBytes) {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(pk)
	return err == nil && len(b) == pkBytes
}

func recent(ts []time.Time, now time.Time) []time.Time {
	keep := ts[:0]
	for _, t := range ts {
		if now.Sub(t) < IssueWindow && !now.Before(t) {
			keep = append(keep, t)
		}
	}
	return keep
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
	for uid, ts := range s.requests {
		if keep := recent(ts, now); len(keep) == 0 {
			delete(s.requests, uid)
		} else {
			s.requests[uid] = keep
		}
	}
	s.lastClean = now
}

// cleanIfDue runs clean at most once per cleanEvery (called under the lock).
func (s *Store) cleanIfDue(now time.Time) {
	if now.Sub(s.lastClean) >= cleanEvery || now.Before(s.lastClean) {
		s.clean(now)
	}
}

// Announce records a heartbeat of computer id with or without easy access.
func (s *Store) Announce(id string, on bool, now time.Time) {
	if !goodID(id) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanIfDue(now)
	if !on {
		delete(s.announced, id)
		return
	}
	if _, known := s.announced[id]; !known && len(s.announced) >= MaxAnnounced {
		s.clean(now)
		if len(s.announced) >= MaxAnnounced {
			return
		}
	}
	s.announced[id] = now
}

// Announced says whether computer id announced easy access in the last AnnounceTTL.
func (s *Store) Announced(id string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanIfDue(now)
	at, ok := s.announced[id]
	return ok && now.Sub(at) < AnnounceTTL
}

// Spend counts one ticket request of account uid, whatever its outcome; false when the account already asked IssueMax
// times in the last IssueWindow.
func (s *Store) Spend(uid uint, now time.Time) bool {
	if uid == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanIfDue(now)
	keep := recent(s.requests[uid], now)
	if len(keep) >= IssueMax {
		s.requests[uid] = keep
		return false
	}
	s.requests[uid] = append(keep, now)
	return true
}

// Issue makes a one-time ticket for account uid/user, from computer `from` to computer `to`, for the session whose
// target key is pk. The caller has already counted the request with Spend.
func (s *Store) Issue(uid uint, user, from, to, pk string, now time.Time) (string, error) {
	user = strings.ToLower(strings.TrimSpace(user))
	if uid == 0 || user == "" || !goodID(from) || !goodID(to) || from == to || !goodPK(pk) {
		return "", ErrBad
	}
	b := make([]byte, ticketBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanIfDue(now)
	if len(s.tickets) >= MaxTickets {
		s.clean(now)
		if len(s.tickets) >= MaxTickets {
			return "", ErrTooMany
		}
	}
	s.tickets[t] = ticket{user: user, uid: uid, from: from, to: to, pk: pk, exp: now.Add(TicketTTL)}
	return t, nil
}

// Check answers once for ticket t presented to computer id: the account (user name), the computer it was given to and
// the key of the session it was given for. The ticket is removed whatever the answer, so it can never be tried twice.
func (s *Store) Check(t, id string, now time.Time) (user, from, pk string, ok bool) {
	if len(t) != ticketHexLen || !goodID(id) {
		return "", "", "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tk, found := s.tickets[t]
	delete(s.tickets, t)
	if !found || !now.Before(tk.exp) || tk.to != id {
		return "", "", "", false
	}
	return tk.user, tk.from, tk.pk, true
}
