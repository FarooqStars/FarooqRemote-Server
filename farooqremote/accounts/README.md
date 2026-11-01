# farooqremote/accounts — the Farooq Remote account server's own changes

FA-020 Phase 5. The account server is lejianwen/rustdesk-api 2.7 + rustdesk-api-web, built from **pinned** upstream
commits by `.github/workflows/farooq-accounts.yml` (GitHub ARM runner) and installed on the Oracle VM by
`FR-08-ACCOUNTS.bat` (NAS: `FA-020-farooqremote/1-working/`). This folder holds everything Farooq-specific; upstream
stays untouched and the build fails if a patch no longer matches.

| File | What |
|---|---|
| `apply-patches.py` | Run by the build after the checkouts. Exact, match-once replacements in 13 upstream files, English words appended to `en.json` / `en.toml`, and the new files below copied in (the upstream `views/register/index.vue` is replaced). |
| `web/src/utils/i18n/ur.json`, `ar.json` | The admin page in Urdu and Arabic (all 201 words). `en.add.json`: English words upstream lacks (added only where missing). |
| `web/src/utils/el-locale-ur.js`, `el-locale-ar.js` | element-plus 2.8.2 labels (tables, pages, dates, dialogs) in Urdu and Arabic. |
| `web/src/styles/farooq.scss` | Right-to-left layout for Urdu / Arabic and the Naskh font **FS Urdu** (first in every language; used only for Arabic-script letters). |
| `web/src/assets/fonts/` | FS Urdu = Noto Naskh Arabic woff2 400/700 (the same files as farooqstars.com) + `OFL-NotoNaskhArabic.txt` (SIL OFL 1.1). |
| `api/resources/i18n/ur.toml`, `ar.toml` | The server's own messages (wrong password, banned, …) in Urdu and Arabic. |
| `hello.html` | The welcome text on «My info», one block per language (`<!--lang:en-->` / `ur` / `ar`); packaged as `conf/admin/hello.html`. |
| `ci/screens.mjs`, `ci/demo.sql` | The build's pictures of the admin page in English, Urdu and Arabic with demo devices (release asset `…-screens.zip`). |
| `api/lib/frsignup/` | 2.7-3: sign-up and «forgot password» by a 6-digit e-mail code — codes (15 minutes, 5 tries, once), limits (3 mails per address per 15 minutes, 10 a day, 20 per network address an hour), the mails in English / Urdu / Arabic, SMTP (Go standard library only; unit test `frsignup_test.go`). |
| `api/http/controller/admin/farooq_signup.go`, `api/http/router/farooq.go` | 2.7-3: the open routes `/api/admin/fr/…` (options, captcha, signup/code, signup/finish, password/code, password/reset). The administrator's account is never reset by e-mail. |
| `web/src/views/register/index.vue`, `forgot.vue`, `web/src/api/farooq.js` | 2.7-3: the sign-up page (e-mail + picture code → code + password) and the forgot-password page; links on the login page. |
| `api-add/en.add.toml` | 2.7-3: English server messages upstream lacks (sign-up, `LoginBanned`, `UserDisabled`), appended to `en.toml`. |
| `ci/signup_test.py` | 2.7-3: the build runs the whole sign-up / forgot-password flow with mails written to files. |
| `api/utils/farooq_captcha.go` (+ `_test.go`) | 2.7-5: our own drawing of the picture code (see below). |
| `web/src/assets/logo.png`, `web/public/favicon.ico` | 2.7-6: Farooq Remote's own icon (the app's icon from the owner's branding files) as the logo and the browser-tab icon. |
| `web/public/company-logo.png`, `hello.html`, `api/lib/frsignup/mail-icon.png` | 2.7-7: the company logo in the welcome box; the app icon inside the sign-up / password mails. |

Upstream changes made by `apply-patches.py`: Arabic + Urdu in the language list; a browser language such as en-US,
ur-PK or ar-QA opens its own language (upstream showed raw keys for en-US); `dir="rtl"` + `lang` on `<html>`; a missing
word falls back to English; the welcome text per language; the language links under the login form; the login-record column names translated.

Owner's words (1 Nov 2026): «add Arabic & Urdu to this and even on remote client with URDU naskh fonts».

## 2.7-3 — sign-up by e-mail code (owner 1 Nov 2026)

«make system from app user can Register this App by Email id and password» — his choices: the account works at once
after the e-mail code; «forgot password» by e-mail in the first release. The e-mail address is the user name (stored
in lower case); a second sign-up for the same address makes no second account — its owner gets a mail instead.
New users join the default group (they see only their own devices; the administrator sees all).

Upstream changes for it: the `/forgot` route (open without login), the two links and «E-mail or user name» on the
login page, the open `/api/admin/fr/…` routes, and the app's login accepts up to 100 letters and any case. Wrong
picture codes and wrong e-mail codes count as failed tries, so upstream's own limiter (`ban-threshold` failures in 10
minutes → that network address waits 30 minutes, code 423 on every route) covers the sign-up pages too.

Mail settings come from the server's environment (systemd `EnvironmentFile`, written by FR-11 on the VM — the mailbox
password is typed there by the owner and is never stored in this repository):
`FR_SMTP_HOST`, `FR_SMTP_PORT` (465 TLS / 587 STARTTLS), `FR_SMTP_USER`, `FR_SMTP_PASS`, `FR_MAIL_FROM`,
`FR_MAIL_FROM_NAME`, `FR_SITE`. Without them sign-up stays closed and the pages say so. `FR_MAIL_TEST_DIR` is for the
build's test only.

## 2.7-4 — a picture code people can read (owner 1 Nov 2026)

«Captcha code very hard to read i tried reset refresh 10 time» — upstream draws 4 letters/digits in decorative fonts
with crossing lines at 150×50. `utils/captcha.go` now draws **4 digits 2–9** (no 0/O or 1/l mix-up) in one plain font
(wqy-microhei) at 200×64 with only a thin wave line; the sign-up and forgot-password pages show it full size above its
box, with an «Another picture» link and a number keypad on phones. The admin login's picture code (after 3 wrong
passwords) uses the same drawing. The e-mail code and the limits stay the real protection.

## 2.7-5 — the picture code drawn by us (owner 1 Nov 2026)

2.7-4 was built but never installed: its build pictures still showed upstream's drawer giving each digit a random size
and a random colour (some pale grey or light green) and a wide wave band through the digits. `utils/farooq_captcha.go`
(`FrDrawDigits`) now draws the picture itself: every digit the same big size (wqy-microhei, a little bold) in one of
seven dark colours, slightly shifted, on a near-white background, with only one faint line and a few faint dots
behind. It is drawn at 400×128 and shown at 200×64, so it stays sharp when the page is zoomed. Upstream's generator
still chooses the digits (`capdString`: 2–9, 4 long); `captcha.go`'s string provider calls `FrDrawDigits`, so the admin
login's picture code is the same. `go test -run TestFrDrawDigits ./utils/` checks size, dark digits and a light
background before the build.

## 2.7-6 — Farooq Remote's own logo (owner 1 Nov 2026)

The owner sent his branding files «for icons and logos»: the account website still showed the RustDesk logo in the
header, on the login, sign-up and forgot-password pages and as the browser-tab icon. `web/src/assets/logo.png` is now
the app's own icon (the teal monitor with the amber arc — the same picture the app uses since September, made from
`App-logo-icon.jpg` with its painted checkerboard keyed out) on a white rounded square, and `web/public/favicon.ico`
the icon itself in 16/32/48/64 pixels. `index.html` and the store's default title say «Farooq Remote» instead of
«Rustdesk API Admin». Everything of 2.7-5 stays.

## 2.7-7 — the company logo and the icon in the mails (owner 1 Nov 2026)

His choices for the branding files: the company logo at the bottom of the pages and in the welcome box, and the icon at
the top of the e-mails. `web/public/company-logo.png` (his `Company-logo.jpg` — the FM mark and FAROOQ MUGHAL — cut to its
content on a white rounded plate, 360 px, 256 colours) is served at `/_admin/company-logo.png` and shown 180 px wide under
the welcome text in every language (`hello.html`). The sign-up, password and «already have an account» mails carry the
app icon inside the mail (`mail-icon.png`, 96 px, `Content-ID: <frlogo@farooqmughal.net>`, multipart/related), shown 40 px
before «Farooq Remote», so it shows even where a mail program blocks pictures from the web; `frsignup_test.go` checks it.
`ci/screens.mjs` removes passing message boxes («Login success», «We sent the code») before each picture.
