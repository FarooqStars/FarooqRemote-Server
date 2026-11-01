// Package frsignup - Farooq Remote (FA-020): sign-up and "forgot password" by an e-mail code.
//
// The owner, 1 Nov 2026: anyone may make an account with an e-mail address and a password; the account works as soon as
// the e-mail is proved with a code, and a forgotten password is replaced the same way.
//
// This file uses the Go standard library only. The settings come from the environment (on the server: a systemd
// EnvironmentFile written by FR-11; the mailbox password is typed there by the owner and is never kept in this
// repository):
//
//	FR_SMTP_HOST, FR_SMTP_PORT (465 = TLS, 587 = STARTTLS), FR_SMTP_USER, FR_SMTP_PASS,
//	FR_MAIL_FROM (default FR_SMTP_USER), FR_MAIL_FROM_NAME (default "Farooq Remote"),
//	FR_SITE (default https://remote.farooqmughal.net)
//	FR_MAIL_TEST_DIR - tests only: mails and picture-code answers are written there instead of being sent.
package frsignup

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math/big"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// mailIcon is Farooq Remote's icon (96 x 96 PNG), sent inside every mail and shown at the top (owner 1 Nov 2026:
// his branding files «for icons and logos»). Inside the mail (cid:) it shows even where pictures from the web are blocked.
//
//go:embed mail-icon.png
var mailIcon []byte

// MailIconCID is the Content-ID of the icon inside the mail.
const MailIconCID = "frlogo@farooqmughal.net"

const (
	CodeTTL      = 15 * time.Minute
	CodeTries    = 5
	PerEmail15m  = 3
	PerEmailDay  = 10
	PerIPHour    = 20
	MinPassword  = 8
	MaxPassword  = 32
	MaxEmail     = 100
	PurposeSign  = "signup"
	PurposeReset = "reset"
)

// TestDir is set only by the build's own test.
func TestDir() string { return strings.TrimSpace(os.Getenv("FR_MAIL_TEST_DIR")) }

// Ready says whether mail can be sent (sign-up and "forgot password" are closed until it can).
func Ready() bool {
	if TestDir() != "" {
		return true
	}
	return env("FR_SMTP_HOST", "") != "" && os.Getenv("FR_SMTP_PASS") != "" && env("FR_SMTP_USER", "") != ""
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// Site is the address of the account website, used in the mails.
func Site() string { return strings.TrimRight(env("FR_SITE", "https://remote.farooqmughal.net"), "/") }

// NormalizeEmail returns the address in lower case, or ok=false when it is not a plain, valid address.
func NormalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || len(s) > MaxEmail || strings.ContainsAny(s, " \t\r\n<>\"'(),;:\\[]") {
		return "", false
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || a.Name != "" {
		return "", false
	}
	at := strings.LastIndex(s, "@")
	if at < 1 || at == len(s)-1 {
		return "", false
	}
	domain := s[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", false
	}
	return s, true
}

// PasswordOK - 8 to 32 characters (the app's login accepts at most 32).
func PasswordOK(p string) bool {
	n := utf8.RuneCountInString(p)
	return n >= MinPassword && n <= MaxPassword && strings.TrimSpace(p) != ""
}

// Lang picks the mail language: an explicit "ur"/"ar"/"en..." first, then the browser's Accept-Language.
func Lang(explicit, acceptLanguage string) string {
	for _, v := range []string{explicit, acceptLanguage} {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		if i := strings.IndexAny(v, ",;"); i >= 0 {
			v = v[:i]
		}
		switch {
		case strings.HasPrefix(v, "ur"):
			return "ur"
		case strings.HasPrefix(v, "ar"):
			return "ar"
		case strings.HasPrefix(v, "en"):
			return "en"
		}
	}
	return "en"
}

type code struct {
	hash    [32]byte
	expires time.Time
	tries   int
}

// Store keeps the codes and the request counters in memory (a restart only drops codes that are still waiting).
type Store struct {
	mu    sync.Mutex
	codes map[string]*code
	sends map[string][]time.Time
}

func NewStore() *Store {
	return &Store{codes: map[string]*code{}, sends: map[string][]time.Time{}}
}

// Default is the store used by the server.
var Default = NewStore()

func hashOf(purpose, email, c string) [32]byte {
	return sha256.Sum256([]byte(purpose + "|" + email + "|" + c))
}

func keep(ts []time.Time, since time.Time) []time.Time {
	out := ts[:0]
	for _, t := range ts {
		if t.After(since) {
			out = append(out, t)
		}
	}
	return out
}

// Allow counts one mail request; false when this address or this network address asked too often.
func (s *Store) Allow(email, ip string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sends) > 20000 {
		for k, v := range s.sends {
			if v = keep(v, now.Add(-24*time.Hour)); len(v) == 0 {
				delete(s.sends, k)
			} else {
				s.sends[k] = v
			}
		}
	}
	ek, ik := "e|"+email, "i|"+ip
	e := keep(s.sends[ek], now.Add(-24*time.Hour))
	recent := 0
	for _, t := range e {
		if t.After(now.Add(-15 * time.Minute)) {
			recent++
		}
	}
	i := keep(s.sends[ik], now.Add(-time.Hour))
	if recent >= PerEmail15m || len(e) >= PerEmailDay || len(i) >= PerIPHour {
		s.sends[ek], s.sends[ik] = e, i
		return false
	}
	s.sends[ek] = append(e, now)
	s.sends[ik] = append(i, now)
	return true
}

// NewCode makes a 6-digit code for this address and purpose; an older waiting code stops working.
func (s *Store) NewCode(purpose, email string, now time.Time) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	c := fmt.Sprintf("%06d", n.Int64())
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.codes {
		if now.After(v.expires) {
			delete(s.codes, k)
		}
	}
	s.codes[purpose+"|"+email] = &code{hash: hashOf(purpose, email, c), expires: now.Add(CodeTTL)}
	return c, nil
}

// Check uses up the code when it is right; after CodeTries wrong tries the code stops working.
func (s *Store) Check(purpose, email, c string, now time.Time) bool {
	c = strings.TrimSpace(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	k := purpose + "|" + email
	v := s.codes[k]
	if v == nil {
		return false
	}
	if now.After(v.expires) {
		delete(s.codes, k)
		return false
	}
	h := hashOf(purpose, email, c)
	if len(c) == 6 && subtle.ConstantTimeCompare(h[:], v.hash[:]) == 1 {
		delete(s.codes, k)
		return true
	}
	v.tries++
	if v.tries >= CodeTries {
		delete(s.codes, k)
	}
	return false
}

// ----- the mails -----

type texts struct{ subject, intro, after, ignore, dir string }

var mails = map[string]map[string]texts{
	PurposeSign: {
		"en": {"Farooq Remote: your code %s", "Your code to make a Farooq Remote account is:", "It works for 15 minutes. Type it on the sign-up page.", "If you did not ask for this, you can ignore this e-mail.", "ltr"},
		"ur": {"فاروق ریموٹ: آپ کا کوڈ %s", "فاروق ریموٹ کا اکاؤنٹ بنانے کے لیے آپ کا کوڈ یہ ہے:", "یہ 15 منٹ تک کام کرے گا۔ اسے سائن اپ کے صفحے پر لکھیں۔", "اگر آپ نے یہ نہیں مانگا تو اس ای میل کو نظر انداز کر دیں۔", "rtl"},
		"ar": {"فاروق ريموت: رمزك %s", "رمز إنشاء حسابك في فاروق ريموت هو:", "الرمز صالح لمدة 15 دقيقة. اكتبه في صفحة التسجيل.", "إذا لم تطلب ذلك، يمكنك تجاهل هذه الرسالة.", "rtl"},
	},
	PurposeReset: {
		"en": {"Farooq Remote: code for a new password %s", "Your code to set a new Farooq Remote password is:", "It works for 15 minutes. Type it on the «Forgot password» page.", "If you did not ask for this, ignore this e-mail - your password stays as it is.", "ltr"},
		"ur": {"فاروق ریموٹ: نیا پاس ورڈ رکھنے کا کوڈ %s", "فاروق ریموٹ کا نیا پاس ورڈ رکھنے کے لیے آپ کا کوڈ یہ ہے:", "یہ 15 منٹ تک کام کرے گا۔ اسے «پاس ورڈ بھول گئے» کے صفحے پر لکھیں۔", "اگر آپ نے یہ نہیں مانگا تو اس ای میل کو نظر انداز کر دیں — آپ کا پاس ورڈ جیسا ہے ویسا رہے گا۔", "rtl"},
		"ar": {"فاروق ريموت: رمز كلمة المرور الجديدة %s", "رمز تعيين كلمة مرور جديدة في فاروق ريموت هو:", "الرمز صالح لمدة 15 دقيقة. اكتبه في صفحة «نسيت كلمة المرور».", "إذا لم تطلب ذلك فتجاهل هذه الرسالة - ستبقى كلمة المرور كما هي.", "rtl"},
	},
	"exists": {
		"en": {"Farooq Remote: you already have an account", "Someone (maybe you) tried to make a new Farooq Remote account with this e-mail address, but an account with it already exists.", "Log in with this address and your password. If you forgot the password, use «Forgot password» on the login page.", "If this was not you, you can ignore this e-mail.", "ltr"},
		"ur": {"فاروق ریموٹ: آپ کا اکاؤنٹ پہلے سے موجود ہے", "کسی نے (شاید آپ نے) اس ای میل سے فاروق ریموٹ کا نیا اکاؤنٹ بنانے کی کوشش کی، مگر اس ای میل کا اکاؤنٹ پہلے سے موجود ہے۔", "اسی ای میل اور اپنے پاس ورڈ سے لاگ ان کریں۔ پاس ورڈ بھول گئے ہوں تو لاگ ان کے صفحے پر «پاس ورڈ بھول گئے» استعمال کریں۔", "اگر یہ آپ نہیں تھے تو اس ای میل کو نظر انداز کر دیں۔", "rtl"},
		"ar": {"فاروق ريموت: لديك حساب بالفعل", "حاول شخص ما (ربما أنت) إنشاء حساب جديد في فاروق ريموت بهذا البريد الإلكتروني، لكن الحساب موجود بالفعل.", "سجّل الدخول بهذا البريد وكلمة المرور. إذا نسيت كلمة المرور فاستخدم «نسيت كلمة المرور» في صفحة الدخول.", "إذا لم تكن أنت، يمكنك تجاهل هذه الرسالة.", "rtl"},
	},
}

// Mail returns the subject, the plain text and the HTML of one mail. kind: PurposeSign, PurposeReset or "exists".
func Mail(kind, lang, c string) (string, string, string) {
	m, ok := mails[kind][lang]
	if !ok {
		m = mails[kind]["en"]
	}
	subject := m.subject
	if strings.Contains(subject, "%s") {
		subject = fmt.Sprintf(subject, c)
	}
	site := Site()
	foot := "Farooq Remote - " + site + "\nFAROOQ MUGHAL (SMC-PRIVATE) LIMITED"
	var text string
	if c != "" {
		text = m.intro + "\n\n    " + c + "\n\n" + m.after + "\n" + m.ignore + "\n\n" + foot + "\n"
	} else {
		text = m.intro + "\n\n" + m.after + "\n" + m.ignore + "\n\n" + foot + "\n"
	}
	align := "left"
	if m.dir == "rtl" {
		align = "right"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><html><body style="margin:0;padding:24px;background:#f4f6f8">`)
	fmt.Fprintf(&b, `<div dir="%s" style="max-width:520px;margin:0 auto;background:#fff;border-radius:10px;padding:28px;text-align:%s;font-family:'Noto Naskh Arabic','Segoe UI',Tahoma,Arial,sans-serif;font-size:16px;line-height:1.8;color:#1f2933">`, m.dir, align)
	fmt.Fprintf(&b, `<div style="font-size:20px;font-weight:bold;color:#0e7490;line-height:40px" dir="ltr"><img src="cid:%s" width="40" height="40" alt="" style="vertical-align:middle;border:0;margin-right:10px">Farooq Remote</div>`, MailIconCID)
	fmt.Fprintf(&b, `<p>%s</p>`, html.EscapeString(m.intro))
	if c != "" {
		fmt.Fprintf(&b, `<div dir="ltr" style="font-size:34px;font-weight:bold;letter-spacing:8px;text-align:center;padding:14px;margin:18px 0;background:#ecfeff;border-radius:8px;color:#0e7490">%s</div>`, html.EscapeString(c))
	}
	fmt.Fprintf(&b, `<p>%s</p><p style="color:#52606d">%s</p>`, html.EscapeString(m.after), html.EscapeString(m.ignore))
	fmt.Fprintf(&b, `<p dir="ltr" style="font-size:13px;color:#7b8794;border-top:1px solid #e4e7eb;padding-top:12px;margin-top:22px">Farooq Remote · <a href="%s">%s</a><br>FAROOQ MUGHAL (SMC-PRIVATE) LIMITED</p>`, html.EscapeString(site), html.EscapeString(strings.TrimPrefix(site, "https://")))
	fmt.Fprintf(&b, `</div></body></html>`)
	return subject, text, b.String()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func wrap76(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteString("\r\n")
		s = s[76:]
	}
	b.WriteString(s)
	b.WriteString("\r\n")
	return b.String()
}

// Message builds the MIME message (text + HTML, UTF-8, base64): multipart/alternative of the plain text and a
// multipart/related part that holds the HTML and the icon it shows (cid:MailIconCID).
func Message(fromName, from, to, subject, text, htmlBody string, now time.Time) []byte {
	boundary := "fr-" + randHex(12)
	related := "fr-rel-" + randHex(12)
	domain := "farooqmughal.net"
	if i := strings.LastIndex(from, "@"); i >= 0 {
		domain = from[i+1:]
	}
	var b bytes.Buffer
	b.WriteString("From: " + mime.QEncoding.Encode("utf-8", fromName) + " <" + from + ">\r\n")
	b.WriteString("To: <" + to + ">\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + randHex(16) + "@" + domain + ">\r\n")
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrap76(base64.StdEncoding.EncodeToString([]byte(text))))
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: multipart/related; boundary=\"" + related + "\"\r\n\r\n")
	b.WriteString("--" + related + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrap76(base64.StdEncoding.EncodeToString([]byte(htmlBody))))
	b.WriteString("--" + related + "\r\n")
	b.WriteString("Content-Type: image/png; name=\"farooq-remote.png\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("Content-ID: <" + MailIconCID + ">\r\n")
	b.WriteString("Content-Disposition: inline; filename=\"farooq-remote.png\"\r\n\r\n")
	b.WriteString(wrap76(base64.StdEncoding.EncodeToString(mailIcon)))
	b.WriteString("--" + related + "--\r\n")
	b.WriteString("--" + boundary + "--\r\n")
	return b.Bytes()
}

// WriteTest writes a file into the test folder (tests only; does nothing on the server).
func WriteTest(name, content string) {
	dir := TestDir()
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
}

// Send sends one mail through the company mailbox (or writes it into the test folder).
func Send(to, subject, text, htmlBody string) error {
	if _, ok := NormalizeEmail(to); !ok {
		return errors.New("bad recipient")
	}
	if TestDir() != "" {
		WriteTest("mail-"+to+".txt", "To: "+to+"\nSubject: "+subject+"\n\n"+text)
		return nil
	}
	host := env("FR_SMTP_HOST", "")
	port := env("FR_SMTP_PORT", "465")
	user := env("FR_SMTP_USER", "")
	pass := os.Getenv("FR_SMTP_PASS")
	from := env("FR_MAIL_FROM", user)
	name := env("FR_MAIL_FROM_NAME", "Farooq Remote")
	if host == "" || user == "" || pass == "" {
		return errors.New("mail is not set up")
	}
	addr := net.JoinHostPort(host, port)
	tc := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	d := &net.Dialer{Timeout: 20 * time.Second}
	if port == "465" {
		conn, err = tls.DialWithDialer(d, "tcp", addr, tc)
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if port != "465" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("the mail server offers no STARTTLS")
		}
		if err = c.StartTLS(tc); err != nil {
			return err
		}
	}
	if err = c.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
		return err
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(Message(name, from, to, subject, text, htmlBody, time.Now())); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
