#!/usr/bin/env python3
"""Farooq Remote (FA-020) - Urdu + Arabic for the account server's admin page (owner 1 Nov 2026:
"add Arabic & Urdu to this ... with URDU naskh fonts").

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
            shutil.copyfile(src, dst)
            print(f'  added   {os.path.relpath(dst)}')


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
             "      <div class=\"fr-langs\">\n"
             "        <a v-for=\"(v, k) in appStore.setting.langs\" :key=\"k\" :lang=\"k\" :class=\"{ on: appStore.setting.lang === k }\"\n"
             "           @click=\"appStore.changeLang(k)\">{{ v.name }}</a>\n"
             "      </div>\n\n"
             "      <div class=\"oidc-options\">\n")
replace_once(p, "  import { useUserStore } from '@/store/user'\n",
             "  import { useUserStore } from '@/store/user'\n  import { useAppStore } from '@/store/app'\n")
replace_once(p, "  const userStore = useUserStore()\n", "  const userStore = useUserStore()\n  const appStore = useAppStore()\n")

# 6. the login records: their column names were fixed English words
for rel in ('src/views/my/login_log/index.vue', 'src/views/login/log.vue'):
    p = os.path.join(WEB, rel)
    replace_once(p, '<el-table-column prop="client" label="client"', '<el-table-column prop="client" :label="T(\'Client\')"')
    replace_once(p, '<el-table-column prop="uuid" label="uuid"', '<el-table-column prop="uuid" :label="T(\'Uuid\')"')
    replace_once(p, '<el-table-column prop="ip" label="ip"', '<el-table-column prop="ip" :label="T(\'Ip\')"')
    replace_once(p, '<el-table-column prop="type" label="type"', '<el-table-column prop="type" :label="T(\'Type\')"')
    replace_once(p, '<el-table-column prop="platform" label="Platform/UA"', '<el-table-column prop="platform" :label="T(\'PlatformUA\')"')

print('== done')
