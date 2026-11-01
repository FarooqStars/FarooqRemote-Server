package freasy

import (
	"strconv"
	"testing"
	"time"
)

const pkA = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // base64 of 32 bytes

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

func TestAnnounceCap(t *testing.T) {
	s := NewStore()
	now := time.Now()
	for i := 0; i < MaxAnnounced; i++ {
		s.Announce(strconv.Itoa(100000000+i), true, now)
	}
	s.Announce("999999999", true, now)
	if s.Announced("999999999", now) {
		t.Fatal("more computers than the cap were remembered")
	}
	if !s.Announced("100000000", now) {
		t.Fatal("a remembered computer was lost")
	}
	s.Announce("100000000", true, now.Add(time.Second)) // a known computer can always renew
	if !s.Announced("100000000", now.Add(AnnounceTTL)) {
		t.Fatal("a known computer could not renew at the cap")
	}
	later := now.Add(AnnounceTTL + time.Second) // all but the renewed one have expired: room again
	s.Announce("999999999", true, later)
	if !s.Announced("999999999", later) {
		t.Fatal("no room after the old ones expired")
	}
	if len(s.announced) > 2 {
		t.Fatal("expired computers were not removed", len(s.announced))
	}
}

func TestTicketOnceForTargetTimeAndKey(t *testing.T) {
	s := NewStore()
	now := time.Now()
	tk, err := s.Issue(7, "Baba@Example.com", "222", "111", pkA, now)
	if err != nil || len(tk) != ticketHexLen {
		t.Fatal("issue", err, tk)
	}
	if _, _, _, ok := s.Check(tk, "999", now); ok {
		t.Fatal("ticket accepted for another computer")
	}
	if _, _, _, ok := s.Check(tk, "111", now); ok {
		t.Fatal("ticket still usable after a wrong check (must be gone)")
	}
	tk, _ = s.Issue(7, "Baba@Example.com", "222", "111", pkA, now)
	user, from, pk, ok := s.Check(tk, "111", now.Add(TicketTTL-time.Second))
	if !ok || user != "baba@example.com" || from != "222" || pk != pkA {
		t.Fatal("good ticket refused", user, from, pk, ok)
	}
	if _, _, _, ok := s.Check(tk, "111", now); ok {
		t.Fatal("ticket used twice")
	}
	tk, _ = s.Issue(7, "baba@example.com", "222", "111", pkA, now)
	if _, _, _, ok := s.Check(tk, "111", now.Add(TicketTTL)); ok {
		t.Fatal("expired ticket accepted")
	}
	if _, _, _, ok := s.Check("00", "111", now); ok {
		t.Fatal("short ticket accepted")
	}
}

func TestIssueRules(t *testing.T) {
	s := NewStore()
	now := time.Now()
	for _, c := range []struct {
		uid                uint
		user, from, to, pk string
	}{
		{0, "a@b.c", "1", "2", pkA}, {1, "", "1", "2", pkA}, {1, "a@b.c", "", "2", pkA}, {1, "a@b.c", "1", "1", pkA},
		{1, "a@b.c", "1", "x y", pkA}, {1, "a@b.c", "1", "2", ""}, {1, "a@b.c", "1", "2", "abc"},
		{1, "a@b.c", "1", "2", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZQ=="}, // 31 bytes
		{1, "a@b.c", "1", "2", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"},  // no padding
		{1, "a@b.c", "1", "2", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY!"}, // not base64
	} {
		if _, err := s.Issue(c.uid, c.user, c.from, c.to, c.pk, now); err == nil {
			t.Fatal("bad request accepted", c)
		}
	}
}

func TestSpendCountsEveryRequest(t *testing.T) {
	s := NewStore()
	now := time.Now()
	if s.Spend(0, now) {
		t.Fatal("account 0 allowed")
	}
	for i := 0; i < IssueMax; i++ {
		if !s.Spend(1, now) {
			t.Fatal("refused before the limit", i)
		}
	}
	if s.Spend(1, now) {
		t.Fatal("no limit")
	}
	if !s.Spend(2, now) {
		t.Fatal("another account limited")
	}
	if !s.Spend(1, now.Add(IssueWindow)) {
		t.Fatal("limit did not end")
	}
	if n := len(s.requests[1]); n > IssueMax {
		t.Fatal("refused requests made the table grow", n)
	}
}

func TestTicketCap(t *testing.T) {
	s := NewStore()
	now := time.Now()
	for i := 0; i < MaxTickets; i++ {
		if _, err := s.Issue(1, "a@b.c", "1", "2", pkA, now); err != nil {
			t.Fatal("refused before the cap", i, err)
		}
	}
	if _, err := s.Issue(1, "a@b.c", "1", "2", pkA, now); err != ErrTooMany {
		t.Fatal("no cap", err)
	}
	if _, err := s.Issue(1, "a@b.c", "1", "2", pkA, now.Add(TicketTTL)); err != nil {
		t.Fatal("no room after the old tickets expired", err)
	}
	if len(s.tickets) != 1 {
		t.Fatal("expired tickets were not removed", len(s.tickets))
	}
}
