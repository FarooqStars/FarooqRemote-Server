package freasy

import (
	"testing"
	"time"
)

func TestAnnounce(t *testing.T) {
	s := NewStore()
	now := time.Now()
	if s.Announced("111222333", now) {
		t.Fatal("announced before any heartbeat")
	}
	s.Announce("111222333", true, now)
	if !s.Announced("111222333", now.Add(AnnounceTTL-time.Second)) {
		t.Fatal("not announced within the time")
	}
	if s.Announced("111222333", now.Add(AnnounceTTL)) {
		t.Fatal("announced after the time")
	}
	s.Announce("444555666", true, now)
	s.Announce("444555666", false, now)
	if s.Announced("444555666", now) {
		t.Fatal("still announced after «off»")
	}
	s.Announce("bad id!", true, now)
	if s.Announced("bad id!", now) {
		t.Fatal("a bad id was recorded")
	}
}

func TestTicketOnceForTargetAndTime(t *testing.T) {
	s := NewStore()
	now := time.Now()
	tk, err := s.Issue(7, "Baba@Example.com", "222", "111", now)
	if err != nil || len(tk) != ticketHexLen {
		t.Fatal("issue", err, tk)
	}
	if _, _, ok := s.Check(tk, "999", now); ok {
		t.Fatal("ticket accepted for another computer")
	}
	if _, _, ok := s.Check(tk, "111", now); ok {
		t.Fatal("ticket still usable after a wrong check (must be gone)")
	}
	tk, _ = s.Issue(7, "Baba@Example.com", "222", "111", now)
	user, from, ok := s.Check(tk, "111", now.Add(TicketTTL-time.Second))
	if !ok || user != "baba@example.com" || from != "222" {
		t.Fatal("good ticket refused", user, from, ok)
	}
	if _, _, ok := s.Check(tk, "111", now); ok {
		t.Fatal("ticket used twice")
	}
	tk, _ = s.Issue(7, "baba@example.com", "222", "111", now)
	if _, _, ok := s.Check(tk, "111", now.Add(TicketTTL)); ok {
		t.Fatal("expired ticket accepted")
	}
	if _, _, ok := s.Check("00", "111", now); ok {
		t.Fatal("short ticket accepted")
	}
}

func TestIssueRules(t *testing.T) {
	s := NewStore()
	now := time.Now()
	for _, c := range []struct {
		uid            uint
		user, from, to string
	}{{0, "a@b.c", "1", "2"}, {1, "", "1", "2"}, {1, "a@b.c", "", "2"}, {1, "a@b.c", "1", "1"}, {1, "a@b.c", "1", "x y"}} {
		if _, err := s.Issue(c.uid, c.user, c.from, c.to, now); err == nil {
			t.Fatal("bad request accepted", c)
		}
	}
	for i := 0; i < IssueMax; i++ {
		if _, err := s.Issue(1, "a@b.c", "1", "2", now); err != nil {
			t.Fatal("refused before the limit", i, err)
		}
	}
	if _, err := s.Issue(1, "a@b.c", "1", "2", now); err != ErrTooMany {
		t.Fatal("no limit", err)
	}
	if _, err := s.Issue(2, "x@y.z", "1", "2", now); err != nil {
		t.Fatal("another account limited", err)
	}
	if _, err := s.Issue(1, "a@b.c", "1", "2", now.Add(IssueWindow)); err != nil {
		t.Fatal("limit did not end", err)
	}
}
