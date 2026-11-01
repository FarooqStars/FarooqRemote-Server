#!/usr/bin/env python3
"""Farooq Remote (FA-020): the build's own test of «easy access» (lib/freasy, http/controller/api/farooq_easy.go).
2.7-10: the target's key travels with the ticket, only a computer signed in to the same account gets one.

Run by .github/workflows/farooq-accounts.yml after signup_test.py, against a freshly restarted test server (the sign-up
test ends with this network address stopped by the limiter):
    python3 easy_test.py http://127.0.0.1:21114 <admin password> <second user> <second user's password>
"""
import json
import re
import sys
import urllib.error
import urllib.request

BASE, ADMIN_PW, USER2, USER2_PW = sys.argv[1].rstrip('/'), sys.argv[2], sys.argv[3], sys.argv[4]


def call(method, path, body=None, token=None, raw_body=None):
    data = raw_body if raw_body is not None else (json.dumps(body).encode() if body is not None else None)
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            raw = r.read()
            try:
                return r.status, (json.loads(raw) if raw else {})
            except ValueError:
                return r.status, {'raw': raw.decode('utf-8', 'replace')}
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


A = ('111111111', 'ci-easy-a')   # the computer to be reached (admin's)
B = ('222222222', 'ci-easy-b')   # admin's other computer
C = ('333333333', 'ci-easy-c')   # the second user's computer
D = ('444444444', 'ci-easy-d')   # a computer nobody is signed in on
PK_A = 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY='   # «A's key» as the app sends it (base64 of 32 bytes)

for pid, uuid in (A, B, C, D):
    s, d = call('POST', '/api/sysinfo', {'id': pid, 'uuid': uuid, 'hostname': 'ci-' + pid, 'os': 'linux', 'cpu': 'ci',
                                          'memory': '1GB', 'username': 'ci', 'version': '1.4.9'})
    ok(s == 200, f'computer {pid} recorded', d)


def login(user, pw, dev):
    s, d = call('POST', '/api/login', {'username': user, 'password': pw, 'id': dev[0], 'uuid': dev[1],
                                       'deviceInfo': {'os': 'linux', 'type': 'client', 'name': 'ci'}})
    ok(s == 200 and d.get('access_token'), f'{user} logged in on {dev[0]}', d)
    return d['access_token']


tok_a = login('admin', ADMIN_PW, A)
tok_b = login('admin', ADMIN_PW, B)
tok_c = login(USER2, USER2_PW, C)


def heartbeat(dev, easy, uuid=None):
    s, d = call('POST', '/api/heartbeat', {'id': dev[0], 'uuid': uuid or dev[1], 'ver': 1, 'fr_easy': easy})
    ok(s == 200, f'heartbeat of {dev[0]} (fr_easy={easy})', d)


def ticket(token, target, frm, pk=PK_A):
    return call('POST', '/api/fr/easy/ticket', {'id': target[0], 'from': frm[0], 'pk': pk}, token=token)


def check(t, dev_id):
    return call('POST', '/api/fr/easy/check', {'ticket': t, 'id': dev_id})


# 1. a ticket needs the signed-in account
s, d = ticket(None, A, B)
ok(s == 401, 'no ticket without signing in', (s, d))

# 2. no ticket while the computer has not said it allows easy access
s, d = ticket(tok_b, A, B)
ok(s == 200 and d.get('easy') is False, 'no ticket before the computer allows easy access', d)

# 3. a heartbeat with another uuid does not count
heartbeat(A, True, uuid='someone-else')
s, d = ticket(tok_b, A, B)
ok(d.get('easy') is False, 'a heartbeat with a wrong uuid is ignored', d)

# 4. the computer allows easy access -> its owner gets a ticket from his other computer
heartbeat(A, True)
s, d = ticket(tok_b, A, B)
t1 = d.get('ticket', '')
ok(s == 200 and d.get('easy') is True and re.fullmatch(r'[0-9a-f]{64}', t1), 'ticket for the owner', d)

# 5. presented to another computer: refused, and the ticket is gone
s, d = check(t1, '999999999')
ok(d.get('ok') is False, 'ticket refused by another computer', d)
s, d = check(t1, A[0])
ok(d.get('ok') is False, 'a ticket tried once cannot be used again', d)

# 6. the right computer gets the account, the asking computer and the key of the session, once
s, d = ticket(tok_b, A, B)
t2 = d.get('ticket', '')
s, d = check(t2, A[0])
ok(d.get('ok') is True and d.get('user') == 'admin' and d.get('from') == B[0] and d.get('pk') == PK_A,
   'the computer learns the account, the caller and the key', d)
s, d = check(t2, A[0])
ok(d.get('ok') is False, 'a used ticket is refused', d)

# 6b. no ticket without a proper key of the computer being reached
for bad in ('', 'abc', PK_A[:-1], 'MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZQ=='):
    s, d = ticket(tok_b, A, B, pk=bad)
    ok(d.get('easy') is False, f'no ticket with a bad key ({bad!r})', d)

# 6c. a computer nobody is signed in on gets no ticket, even with easy access said
heartbeat(D, True)
s, d = ticket(tok_b, D, B)
ok(d.get('easy') is False, 'no ticket for a computer not signed in to the account', d)

# 7. another account gets no ticket for a computer bound to the owner
s, d = ticket(tok_c, A, C)
ok(d.get('easy') is False, 'another account gets no ticket', d)

# 8. a ticket can not be asked for the same computer it comes from
s, d = ticket(tok_a, A, A)
ok(d.get('easy') is False, 'no ticket from a computer to itself', d)

# 9. easy access switched off -> no more tickets
heartbeat(A, False)
s, d = ticket(tok_b, A, B)
ok(d.get('easy') is False, 'no ticket after easy access is switched off', d)

# 10. rubbish is refused
s, d = check('zz', A[0])
ok(d.get('ok') is False, 'a malformed ticket is refused', d)
s, d = call('POST', '/api/fr/easy/check', raw_body=b'{not json')
ok(s == 400, 'malformed request refused', (s, d))

# 11. logging out on the computer ends easy access to it (it is no longer signed in to the account)
heartbeat(A, True)
s, d = ticket(tok_b, A, B)
ok(d.get('easy') is True, 'ticket again while A is signed in', d)
check(d.get('ticket', ''), A[0])
s, d = call('POST', '/api/logout', {}, token=tok_a)
ok(s == 200, 'admin logged out on A', (s, d))
s, d = ticket(tok_b, A, B)
ok(d.get('easy') is False, 'no ticket after logging out on the computer', d)

print('EASY ACCESS TEST PASSED')
