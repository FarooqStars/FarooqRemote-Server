#!/usr/bin/env python3
"""Farooq Remote (FA-020) - the account server's Farooq changes (owner 1 Nov 2026):
- Urdu + Arabic for the admin page ("add Arabic & Urdu to this ... with URDU naskh fonts");
- sign-up by anyone with an e-mail address + password, proved by an e-mail code, and "forgot password" by an e-mail
  code (lib/frsignup + http/controller/admin/farooq_signup.go + the web pages register/index.vue, register/forgot.vue).

Run by .github/workflows/farooq-accounts.yml after the pinned checkouts:
    python3 farooqremote/accounts/apply-patches.py <api checkout> <web checkout>

Every change to an upstream file is an exact text replacement that must match ONCE - if upstream ever differs, the
build stops here instead of producing something half-patched. New files are copied from farooqremote/accounts/.
"""
import json
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
API, WEB = sys.argv[1], sys.argv[2]


def replace_once(path, old, new):
    with open(path, encoding='utf-8') as f:
        text = f.read()
    n = text.count(old)
    if n != 1:
        sys.exit(f'PATCH FAILED: {path}: expected 1 match, found {n}: {old[:80]!r}')
    with open(path, 'w', encoding='utf-8') as f:
        f.write(text.replace(old, new))
    print(f'  patched {os.path.relpath(path)}')


def copy_tree(src_rel, dst_root):
    src_root = os.path.join(HERE, src_rel)
    for d, _, files in os.walk(src_root):
        for name in files:
            src = os.path.join(d, name)
            rel = os.path.relpath(src, src_root)
            if rel.endswith('en.add.json'):
                continue
            dst = os.path.join(dst_root, rel)
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            existed = os.path.exists(dst)
            shutil.copyfile(src, dst)
            print(f'  {"replaced" if existed else "added   "} {os.path.relpath(dst)}')


print('== new files')
copy_tree('web', WEB)
copy_tree('api', API)

print('== English words upstream lacks (added only where missing)')
en_path = os.path.join(WEB, 'src/utils/i18n/en.json')
en = json.load(open(en_path, encoding='utf-8'))
add = json.load(open(os.path.join(HERE, 'web/src/utils/i18n/en.add.json'), encoding='utf-8'))
added = [k for k in add if k not in en]
for k in added:
    en[k] = add[k]
json.dump(en, open(en_path, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
print(f'  en.json +{len(added)}: {", ".join(added)}')

print('== server messages upstream lacks (English; Urdu + Arabic are in our own ur.toml / ar.toml)')
toml_path = os.path.join(API, 'resources/i18n/en.toml')
toml = open(toml_path, encoding='utf-8').read()
add_toml = open(os.path.join(HERE, 'api-add/en.add.toml'), encoding='utf-8').read()
for line in add_toml.splitlines():
    if line.startswith('[') and line in toml.splitlines():
        sys.exit(f'PATCH FAILED: en.toml already has {line}')
open(toml_path, 'w', encoding='utf-8').write(toml.rstrip('\n') + '\n\n' + add_toml)
print(f'  en.toml +{add_toml.count(chr(10) + "[") + 1} messages')

print('== upstream files')
# 1. the word lists: Arabic + Urdu, and a missing word falls back to English instead of showing its raw key
p = os.path.join(WEB, 'src/utils/i18n.js')
replace_once(p, "import zhTW from '@/utils/i18n/zh_TW.json'\n",
             "import zhTW from '@/utils/i18n/zh_TW.json'\nimport ar from '@/utils/i18n/ar.json'\nimport ur from '@/utils/i18n/ur.json'\n")
replace_once(p, "  'zh-TW': zhTW,\n}", "  'zh-TW': zhTW,\n  'ar': ar,\n  'ur': ur,\n}")
replace_once(p, "  const tran = trans[lang]?.[key]\n", "  const tran = trans[lang]?.[key] || trans['en']?.[key]\n")

# 2. the language list, the browser language (en-US -> English, ur-PK -> Urdu ...) and right-to-left
p = os.path.join(WEB, 'src/store/app.js')
replace_once(p, "import zhTw from 'element-plus/es/locale/lang/zh-tw'\n",
             "import zhTw from 'element-plus/es/locale/lang/zh-tw'\n"
             "import arEl from '@/utils/el-locale-ar'\n"
             "import urEl from '@/utils/el-locale-ur'\n")
replace_once(p, "  'zh-TW': { name: '中文繁体', value: zhTw, sideBarWidth: '210px' },\n",
             "  'zh-TW': { name: '中文繁体', value: zhTw, sideBarWidth: '210px' },\n"
             "  'ar': { name: 'العربية', value: arEl, sideBarWidth: '280px' },\n"
             "  'ur': { name: 'اردو', value: urEl, sideBarWidth: '280px' },\n")
replace_once(p, "const defaultLang = localStorage.getItem('lang') || navigator.language || 'zh-CN'\n",
             "// Farooq Remote: a browser language like en-US, ur-PK or ar-QA opens its own language (upstream showed raw keys)\n"
             "const pickLang = (l) => {\n"
             "  if (!l) return 'en'\n"
             "  if (langs[l]) return l\n"
             "  const base = String(l).split('-')[0].toLowerCase()\n"
             "  if (langs[base]) return base\n"
             "  if (base === 'zh') return 'zh-CN'\n"
             "  return 'en'\n"
             "}\n"
             "export const applyLangDir = (l) => {\n"
             "  const rtl = l === 'ar' || l === 'ur'\n"
             "  document.documentElement.setAttribute('dir', rtl ? 'rtl' : 'ltr')\n"
             "  document.documentElement.setAttribute('lang', l || 'en')\n"
             "}\n"
             "const defaultLang = pickLang(localStorage.getItem('lang') || navigator.language)\n"
             "applyLangDir(defaultLang)\n")
replace_once(p, "      localStorage.setItem('lang', lang)\n",
             "      localStorage.setItem('lang', lang)\n      applyLangDir(lang)\n")

# 3. the Urdu / Arabic font and the right-to-left styles
p = os.path.join(WEB, 'src/main.js')
replace_once(p, "import '@/styles/style.scss'\n", "import '@/styles/style.scss'\nimport '@/styles/farooq.scss'\n")

# 4. the welcome text: one block per language (<!--lang:en--> ... <!--lang:ur--> ... <!--lang:ar--> ...)
p = os.path.join(WEB, 'src/views/my/info.vue')
replace_once(p, "  const html = computed(_ => marked(appStore.setting.hello||''))\n",
             "  // Farooq Remote: the hello file may hold one block per language; show the block of the page language\n"
             "  const helloFor = (text, lang) => {\n"
             "    if (!text || text.indexOf('<!--lang:') < 0) return text || ''\n"
             "    const parts = {}\n"
             "    let cur = ''\n"
             "    text.split(/<!--lang:([a-zA-Z-]+)-->/).forEach((p, i) => { if (i % 2 === 1) cur = p; else if (cur) parts[cur] = p })\n"
             "    return parts[lang] || parts['en'] || ''\n"
             "  }\n"
             "  const html = computed(_ => marked(helloFor(appStore.setting.hello, appStore.setting.lang)))\n")

# 5. the login page: the languages under the form (upstream has no way to change the language before logging in)
p = os.path.join(WEB, 'src/views/login/login.vue')
replace_once(p, "      <div class=\"oidc-options\">\n",
             "      <div class=\"fr-links\" v-if=\"!disablePwd\">\n"
             "        <a @click=\"router.push('/register')\">{{ T('FrCreateAccount') }}</a>\n"
             "        <a @click=\"router.push('/forgot')\">{{ T('FrForgotPassword') }}</a>\n"
             "      </div>\n\n"
             "      <div class=\"fr-langs\">\n"
             "        <a v-for=\"(v, k) in appStore.setting.langs\" :key=\"k\" :lang=\"k\" :class=\"{ on: appStore.setting.lang === k }\"\n"
             "           @click=\"appStore.changeLang(k)\">{{ v.name }}</a>\n"
             "      </div>\n\n"
             "      <div class=\"oidc-options\">\n")
replace_once(p, "  import { useUserStore } from '@/store/user'\n",
             "  import { useUserStore } from '@/store/user'\n  import { useAppStore } from '@/store/app'\n")
replace_once(p, "  const userStore = useUserStore()\n", "  const userStore = useUserStore()\n  const appStore = useAppStore()\n")
replace_once(p, "        <el-form-item :label=\"T('Username')\">\n",
             "        <el-form-item :label=\"T('FrEmailOrUsername')\">\n")

# 6. the login records: their column names were fixed English words
for rel in ('src/views/my/login_log/index.vue', 'src/views/login/log.vue'):
    p = os.path.join(WEB, rel)
    replace_once(p, '<el-table-column prop="client" label="client"', '<el-table-column prop="client" :label="T(\'Client\')"')
    replace_once(p, '<el-table-column prop="uuid" label="uuid"', '<el-table-column prop="uuid" :label="T(\'Uuid\')"')
    replace_once(p, '<el-table-column prop="ip" label="ip"', '<el-table-column prop="ip" :label="T(\'Ip\')"')
    replace_once(p, '<el-table-column prop="type" label="type"', '<el-table-column prop="type" :label="T(\'Type\')"')
    replace_once(p, '<el-table-column prop="platform" label="Platform/UA"', '<el-table-column prop="platform" :label="T(\'PlatformUA\')"')

# 7. sign-up + "forgot password" pages: the new route, open without logging in
p = os.path.join(WEB, 'src/router/index.js')
replace_once(p, "    component: () => import('@/views/register/index.vue'),\n  },\n",
             "    component: () => import('@/views/register/index.vue'),\n  },\n"
             "  {\n"
             "    path: '/forgot',\n"
             "    name: 'Forgot',\n"
             "    meta: { title: 'FrForgotTitle' },\n"
             "    component: () => import('@/views/register/forgot.vue'),\n"
             "  },\n")
p = os.path.join(WEB, 'src/permission.js')
replace_once(p, "const whiteList = ['/login', '/register']\n", "const whiteList = ['/login', '/register', '/forgot']\n")

# 8. server: the open routes of the sign-up (lib/frsignup + http/controller/admin/farooq_signup.go + http/router/farooq.go)
p = os.path.join(API, 'http/router/admin.go')
replace_once(p, "\tadg.POST(\"/user/register\", (&admin.User{}).Register)\n",
             "\tadg.POST(\"/user/register\", (&admin.User{}).Register)\n"
             "\t// Farooq Remote (FA-020): sign-up and \"forgot password\" by an e-mail code\n"
             "\tFarooqSignupBind(adg)\n")

# 9. server: an e-mail is the user name of a signed-up account - the app's login must accept up to 100 characters
p = os.path.join(API, 'http/request/api/user.go')
replace_once(p, 'Username   string            `json:"username" validate:"required,gte=2,lte=32" label:"用户名"`',
             'Username   string            `json:"username" validate:"required,gte=2,lte=100" label:"用户名"`')

# 10. server: sign-up stores the address in lower case - the login accepts it in any case
p = os.path.join(API, 'service/user.go')
replace_once(p, "\tu := &model.User{}\n\tDB.Where(\"username = ?\", username).First(u)\n\tif u.Id == 0 {\n\t\treturn u\n\t}\n\tok, newHash, err := utils.VerifyPassword(u.Password, password)\n",
             "\tu := &model.User{}\n\tDB.Where(\"username = ?\", username).First(u)\n"
             "\tif u.Id == 0 {\n"
             "\t\t// Farooq Remote: accounts made by e-mail sign-up are stored in lower case; accept the address in any case\n"
             "\t\tif lu := strings.ToLower(strings.TrimSpace(username)); lu != username {\n"
             "\t\t\tDB.Where(\"username = ?\", lu).First(u)\n"
             "\t\t}\n"
             "\t}\n"
             "\tif u.Id == 0 {\n\t\treturn u\n\t}\n\tok, newHash, err := utils.VerifyPassword(u.Password, password)\n")

# 11. server: the picture code (owner 1 Nov 2026: «Captcha code very hard to read») - 4 digits 2-9 (no 0/O or 1/l
#     mix-up; upstream: letters + digits in decorative fonts with crossing lines), drawn by our own utils/farooq_captcha.go:
#     one big size, dark colours, a little bold, only a faint line behind (2.7-4 still used upstream's drawer, whose
#     random sizes, pale colours and wave band were still hard). Also used by the admin login after 3 wrong passwords.
p = os.path.join(API, 'utils/captcha.go')
replace_once(p, 'var capdString = base64Captcha.NewDriverString(50, 150, 0, 5, 4, "123456789abcdefghijklmnopqrstuvwxyz", nil, nil,\n'
             '\t[]string{"3Dumb.ttf", "ApothecaryFont.ttf", "Comismsh.ttf", "Flim-Flam.ttf", "RitaSmith.ttf", "wqy-microhei.ttc"})\n',
             '// Farooq Remote: the code is 4 digits 2-9 (no 0/O or 1/l mix-up); the picture is drawn by FrDrawDigits\n'
             'var capdString = base64Captcha.NewDriverString(64, 200, 0, 0, 4, "23456789", nil, nil,\n'
             '\t[]string{"wqy-microhei.ttc"})\n')
replace_once(p, 'func (p B64StringCaptchaProvider) Draw(content string) (string, error) {\n'
             '\titem, err := capdString.DrawCaptcha(content)\n'
             '\tif err != nil {\n\t\treturn "", err\n\t}\n'
             '\tb64str := item.EncodeB64string()\n'
             '\treturn b64str, nil\n}\n',
             'func (p B64StringCaptchaProvider) Draw(content string) (string, error) {\n'
             '\t// Farooq Remote: our own clear picture (farooq_captcha.go)\n'
             '\treturn FrDrawDigits(content)\n}\n')

print('== done')
