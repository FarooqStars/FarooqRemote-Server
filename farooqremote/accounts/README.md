# farooqremote/accounts — the Farooq Remote account server's own changes

FA-020 Phase 5. The account server is lejianwen/rustdesk-api 2.7 + rustdesk-api-web, built from **pinned** upstream
commits by `.github/workflows/farooq-accounts.yml` (GitHub ARM runner) and installed on the Oracle VM by
`FR-08-ACCOUNTS.bat` (NAS: `FA-020-farooqremote/1-working/`). This folder holds everything Farooq-specific; upstream
stays untouched and the build fails if a patch no longer matches.

| File | What |
|---|---|
| `apply-patches.py` | Run by the build after the checkouts. Exact, match-once replacements in 6 upstream files + copies the new files below. |
| `web/src/utils/i18n/ur.json`, `ar.json` | The admin page in Urdu and Arabic (all 199 words). `en.add.json`: English words upstream lacks (added only where missing). |
| `web/src/utils/el-locale-ur.js`, `el-locale-ar.js` | element-plus 2.8.2 labels (tables, pages, dates, dialogs) in Urdu and Arabic. |
| `web/src/styles/farooq.scss` | Right-to-left layout for Urdu / Arabic and the Naskh font **FS Urdu** (first in every language; used only for Arabic-script letters). |
| `web/src/assets/fonts/` | FS Urdu = Noto Naskh Arabic woff2 400/700 (the same files as farooqstars.com) + `OFL-NotoNaskhArabic.txt` (SIL OFL 1.1). |
| `api/resources/i18n/ur.toml`, `ar.toml` | The server's own messages (wrong password, banned, …) in Urdu and Arabic. |
| `hello.html` | The welcome text on «My info», one block per language (`<!--lang:en-->` / `ur` / `ar`); packaged as `conf/admin/hello.html`. |
| `ci/screens.mjs`, `ci/demo.sql` | The build's pictures of the admin page in English, Urdu and Arabic with demo devices (release asset `…-screens.zip`). |

Upstream changes made by `apply-patches.py`: Arabic + Urdu in the language list; a browser language such as en-US,
ur-PK or ar-QA opens its own language (upstream showed raw keys for en-US); `dir="rtl"` + `lang` on `<html>`; a missing
word falls back to English; the welcome text per language; the language links under the login form.

Owner's words (1 Nov 2026): «add Arabic & Urdu to this and even on remote client with URDU naskh fonts».
