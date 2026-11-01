#!/usr/bin/env python3
"""Farooq Remote (FA-020): the build's own test of sign-up and "forgot password" by e-mail code.

Run by .github/workflows/farooq-accounts.yml against a server started with FR_MAIL_TEST_DIR (mails and picture-code
answers are written there as files instead of being sent):
    python3 signup_test.py http://127.0.0.1:21114 <FR_MAIL_TEST_DIR> <admin password>
"""
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

BASE, MAILS, ADMIN_PW = sys.argv[1].rstrip('/'), sys.argv[2], sys.argv[3]


def call(method, path, body=None, lang='en'):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method,
                                 headers={'Content-Type': 'application/json', 'Accept-Language': lang})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            raw = r.read()
            return r.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except ValueError:
            return e.code, {'raw': raw.decode('utf-8', 'replace')}


def ok(cond, msg, extra=''):
    print(('OK    ' if cond else 'FAIL  ') + msg + (('  ' + str(extra)) if extra and not cond else ''))
    if not cond:
        sys.exit(1)


def captcha():
    s, d = call('GET', '/api/admin/fr/captcha')
    ok(s == 200 and d.get('code') == 0 and d['data']['captcha']['b64'].startswith('data:image'), 'picture code', d)
    cid = d['data']['captcha']['id']
    return cid, open(os.path.join(MAILS, f'captcha-{cid}.txt'), encoding='utf-8').read().strip()


def mail_for(email, wait=6.0):
    p = os.path.join(MAILS, f'mail-{email}.txt')
    t = time.time()
    while not os.path.exists(p) and time.time() - t < wait:
        time.sleep(0.1)
    if not os.path.exists(p):
        return None, None
    time.sleep(0.2)
    text = open(p, encoding='utf-8').read()
    os.remove(p)
    m = re.search(r'(?<!\d)(\d{6})(?!\d)', text)
    return (m.group(1) if m else None), text


def code_request(path, email, lang='en', good_captcha=True):
    cid, ans = captcha()
    if not good_captcha:
        ans = 'wrong' + ans
    return call('POST', path, {'email': email, 'captcha_id': cid, 'captcha': ans, 'lang': lang}, lang)


def app_login(username, password, lang='en'):
    return call('POST', '/api/login', {'username': username, 'password': password, 'id': '123456789', 'uuid': 'ci-test',
                                       'deviceInfo': {'os': 'linux', 'type': 'client', 'name': 'ci'}}, lang)


# 1. sign-up is open (the test server has a mail folder)
s, d = call('GET', '/api/admin/fr/options')
ok(s == 200 and d['data']['signup'] is True and d['data']['min_password'] == 8, 'options: sign-up open', d)

# 2. a wrong picture code is refused (and counted)
s, d = code_request('/api/admin/fr/signup/code', 'someone@example.com', good_captcha=False)
ok(d.get('code') == 101, 'wrong picture code refused', d)

# 3. a bad address is refused
s, d = code_request('/api/admin/fr/signup/code', 'not-an-address')
ok(d.get('code') == 101 and 'e-mail' in d.get('message', ''), 'bad address refused', d)

# 4. sign-up code in Urdu for a mixed-case address
EMAIL = 'new.user@example.com'
s, d = code_request('/api/admin/fr/signup/code', 'New.User@Example.COM', lang='ur')
ok(d.get('code') == 0, 'sign-up code sent', d)
code, text = mail_for(EMAIL)
ok(code is not None and 'فاروق ریموٹ' in text, 'Urdu mail with a 6-digit code (address stored in lower case)', text)

# 5. a wrong code and a short password are refused - in Urdu
s, d = call('POST', '/api/admin/fr/signup/finish', {'email': EMAIL, 'code': '000000' if code != '000000' else '111111',
                                                    'password': 'Test-pass-123'}, 'ur')
ok(d.get('code') == 101 and 'کوڈ' in d.get('message', ''), 'wrong code refused (Urdu message)', d)
s, d = call('POST', '/api/admin/fr/signup/finish', {'email': EMAIL, 'code': code, 'password': 'short'})
ok(d.get('code') == 101 and '8' in d.get('message', ''), 'short password refused', d)

# 6. the right code makes the account and logs in
s, d = call('POST', '/api/admin/fr/signup/finish', {'email': EMAIL, 'code': code, 'password': 'Test-pass-123'})
ok(d.get('code') == 0 and d['data'].get('token') and d['data'].get('username') == EMAIL, 'account made + logged in', d)
s, d = call('POST', '/api/admin/fr/signup/finish', {'email': EMAIL, 'code': code, 'password': 'Test-pass-123'})
ok(d.get('code') == 101, 'the code works only once', d)

# 7. the app logs in with the address in any case
s, d = app_login('NEW.User@example.com', 'Test-pass-123')
ok(s == 200 and d.get('access_token'), 'app login with the e-mail (any case)', d)

# 8. a long address (over the old 32-letter limit) can sign up and log in to the app
LONG = 'a.very.long.address.for.the.test@example-domain.com'
s, d = code_request('/api/admin/fr/signup/code', LONG)
code, _ = mail_for(LONG)
s, d = call('POST', '/api/admin/fr/signup/finish', {'email': LONG, 'code': code, 'password': 'Another-pass-9'})
ok(d.get('code') == 0, f'long address signed up ({len(LONG)} letters)', d)
s, d = app_login(LONG, 'Another-pass-9')
ok(s == 200 and d.get('access_token'), 'app login with the long address', d)

# 9. the same address again: no second account - its owner gets an "already has an account" mail without a code
s, d = code_request('/api/admin/fr/signup/code', EMAIL)
ok(d.get('code') == 0, 'second sign-up for the same address answered the same way', d)
code, text = mail_for(EMAIL)
ok(code is None and text and 'already' in text, '"you already have an account" mail, no code', text)

# 10. too many codes for one address in 15 minutes
L = 'limit@example.com'
answers = [code_request('/api/admin/fr/signup/code', L)[1].get('code') for _ in range(4)]
ok(answers[:3] == [0, 0, 0] and answers[3] == 101, '4th code in 15 minutes refused', answers)
mail_for(L)

# 11. forgot password: code, wrong code, new password
s, d = code_request('/api/admin/fr/password/code', EMAIL, lang='ar')
ok(d.get('code') == 0, 'password code answered', d)
code, text = mail_for(EMAIL)
ok(code is not None and 'فاروق ريموت' in text, 'Arabic mail with a 6-digit code', text)
s, d = call('POST', '/api/admin/fr/password/reset', {'email': EMAIL, 'code': '000000' if code != '000000' else '111111',
                                                     'password': 'New-pass-456'})
ok(d.get('code') == 101, 'wrong reset code refused', d)
s, d = call('POST', '/api/admin/fr/password/reset', {'email': EMAIL, 'code': code, 'password': 'New-pass-456'})
ok(d.get('code') == 0, 'new password saved', d)
s, d = app_login(EMAIL, 'Test-pass-123')
ok(s != 200, 'old password no longer works', d)
s, d = app_login(EMAIL, 'New-pass-456')
ok(s == 200 and d.get('access_token'), 'new password works', d)

# 12. an address without an account gets the same answer and no mail
s, d = code_request('/api/admin/fr/password/code', 'nobody@example.com')
ok(d.get('code') == 0, 'unknown address: same answer', d)
code, text = mail_for('nobody@example.com', wait=2.0)
ok(text is None, 'unknown address: no mail')

# 13. the web page contains the sign-up and the forgot-password pages
found = []
for root, _, files in os.walk(os.environ.get('FR_ADMIN_DIR', 'resources/admin')):
    for f in files:
        if f.endswith('.js'):
            t = open(os.path.join(root, f), encoding='utf-8', errors='replace').read()
            found += [w for w in ('/fr/signup/code', '/fr/password/reset') if w in t]
ok('/fr/signup/code' in found and '/fr/password/reset' in found, 'admin page has the sign-up + forgot-password calls', found)

# 14. too many wrong passwords stop the network address on every route (upstream's own limiter, code 423); the wrong
#     picture codes and e-mail codes above count too
banned = False
for i in range(15):
    s, d = app_login('admin', f'wrong-pass-{i}')
    if d.get('code') == 423:
        banned = True
        break
ok(banned, f'network address stopped after wrong passwords (after {i + 1} more)', d)
s, d = app_login('admin', ADMIN_PW)
ok(d.get('code') == 423, 'even the right password waits while the address is stopped', d)
s, d = call('GET', '/api/admin/fr/captcha')
ok(d.get('code') == 423, 'the sign-up pages wait too', d)

print('SIGN-UP TEST PASSED')
