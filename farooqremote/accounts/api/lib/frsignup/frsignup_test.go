package frsignup

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeEmail(t *testing.T) {
	good := map[string]string{
		"  Farooq@Example.COM ": "farooq@example.com",
		"a.b-c+d@sub.domain.pk": "a.b-c+d@sub.domain.pk",
		"info@farooqmughal.net": "info@farooqmughal.net",
	}
	for in, want := range good {
		got, ok := NormalizeEmail(in)
		if !ok || got != want {
			t.Fatalf("NormalizeEmail(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "a", "a@", "@b.com", "a@b", "a@.com", "a@b..com", "a b@c.com", "x <a@b.com>",
		"a@b.com\r\nBcc: z@z.com", "\"a\"@b.com", strings.Repeat("a", 95) + "@b.com"} {
		if _, ok := NormalizeEmail(bad); ok {
			t.Fatalf("NormalizeEmail(%q) accepted", bad)
		}
	}
}

func TestPassword(t *testing.T) {
	if PasswordOK("1234567") || !PasswordOK("12345678") || !PasswordOK(strings.Repeat("ب", 32)) || PasswordOK(strings.Repeat("x", 33)) || PasswordOK("        ") {
		t.Fatal("password rule")
	}
}

func TestLang(t *testing.T) {
	if Lang("", "ur-PK,ur;q=0.9") != "ur" || Lang("ar", "en") != "ar" || Lang("", "") != "en" || Lang("zh-CN", "fr") != "en" {
		t.Fatal("lang")
	}
}

func TestCodes(t *testing.T) {
	s := NewStore()
	now := time.Now()
	c, err := s.NewCode(PurposeSign, "a@b.com", now)
	if err != nil || len(c) != 6 {
		t.Fatal("code", c, err)
	}
	if s.Check(PurposeReset, "a@b.com", c, now) {
		t.Fatal("purpose must matter")
	}
	if s.Check(PurposeSign, "other@b.com", c, now) {
		t.Fatal("address must matter")
	}
	if !s.Check(PurposeSign, "a@b.com", c, now) {
		t.Fatal("right code refused")
	}
	if s.Check(PurposeSign, "a@b.com", c, now) {
		t.Fatal("a code works only once")
	}
	// five wrong tries end the code
	c, _ = s.NewCode(PurposeSign, "a@b.com", now)
	wrong := "000000"
	if wrong == c {
		wrong = "111111"
	}
	for i := 0; i < CodeTries; i++ {
		s.Check(PurposeSign, "a@b.com", wrong, now)
	}
	if s.Check(PurposeSign, "a@b.com", c, now) {
		t.Fatal("code must stop after the wrong tries")
	}
	// expiry
	c, _ = s.NewCode(PurposeSign, "a@b.com", now)
	if s.Check(PurposeSign, "a@b.com", c, now.Add(CodeTTL+time.Second)) {
		t.Fatal("expired code accepted")
	}
	// a new code replaces the old one
	c1, _ := s.NewCode(PurposeSign, "a@b.com", now)
	c2, _ := s.NewCode(PurposeSign, "a@b.com", now)
	if c1 != c2 && s.Check(PurposeSign, "a@b.com", c1, now) {
		t.Fatal("old code still works")
	}
	if !s.Check(PurposeSign, "a@b.com", c2, now) {
		t.Fatal("new code refused")
	}
}

func TestAllow(t *testing.T) {
	s := NewStore()
	now := time.Now()
	for i := 0; i < PerEmail15m; i++ {
		if !s.Allow("a@b.com", "1.1.1.1", now) {
			t.Fatal("refused too early", i)
		}
	}
	if s.Allow("a@b.com", "1.1.1.1", now) {
		t.Fatal("4th mail in 15 minutes allowed")
	}
	if !s.Allow("a@b.com", "1.1.1.1", now.Add(16*time.Minute)) {
		t.Fatal("refused after 15 minutes")
	}
	for i := 0; i < PerIPHour; i++ {
		s.Allow("x"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@b.com", "2.2.2.2", now)
	}
	if s.Allow("new@b.com", "2.2.2.2", now) {
		t.Fatal("too many from one network address allowed")
	}
}

func TestMailIcon(t *testing.T) {
	if len(mailIcon) < 500 || string(mailIcon[1:4]) != "PNG" {
		t.Fatal("mail-icon.png missing or not a PNG")
	}
}

func TestMail(t *testing.T) {
	for _, kind := range []string{PurposeSign, PurposeReset, "exists"} {
		for _, l := range []string{"en", "ur", "ar", "xx"} {
			c := "123456"
			if kind == "exists" {
				c = ""
			}
			subj, text, html := Mail(kind, l, c)
			if subj == "" || text == "" || !strings.Contains(html, "</html>") {
				t.Fatal("empty mail", kind, l)
			}
			if c != "" && (!strings.Contains(subj, c) || !strings.Contains(text, c) || !strings.Contains(html, c)) {
				t.Fatal("code missing", kind, l)
			}
			msg := string(Message("Farooq Remote", "info@farooqmughal.net", "a@b.com", subj, text, html, time.Now()))
			if !strings.Contains(msg, "Content-Transfer-Encoding: base64") || strings.Contains(msg, "\n\n\n") {
				t.Fatal("message shape", kind, l)
			}
			if !strings.Contains(html, "cid:"+MailIconCID) || !strings.Contains(msg, "Content-ID: <"+MailIconCID+">") ||
				!strings.Contains(msg, "multipart/related") || !strings.Contains(msg, "Content-Type: image/png") {
				t.Fatal("the icon is not inside the mail", kind, l)
			}
			for _, line := range strings.Split(msg, "\r\n") {
				if len(line) > 998 {
					t.Fatal("line too long")
				}
			}
		}
	}
}
