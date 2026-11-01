"""Run with `docker compose up --build -d source analytics app` first."""
import json
import time
from urllib.request import Request, urlopen

BASE = 'http://localhost:8080'
def get(path):
    with urlopen(BASE + path, timeout=60) as r:
        return json.load(r)
def post(path):
    with urlopen(Request(BASE + path, data=b'', method='POST'), timeout=120) as r:
        return r.read()

for _ in range(60):
    try:
        get('/api/status')
        break
    except OSError:
        time.sleep(2)
else:
    raise RuntimeError('application failed to start')
post('/api/run?stage=all')
s = get('/api/status')
assert not s.get('martsError'), s
sales = {x['day']: x for x in s['marts']}
assert sales['2026-10-01']['gross_sales'] == 1000, sales
assert sales['2026-10-02']['gross_sales'] == 1200, sales
assert sales['2026-10-03']['returns'] == 500, sales
trace = get('/api/trace?order_id=1')
assert trace['source'][0]['id'] == 1 and trace['warehouse'][0]['order_id'] == 1, trace
assert trace['lake'][0]['id'] == 1 and trace['raw'], trace
post('/api/run?stage=all')
assert get('/api/status')['marts'] == s['marts'], 'rerun changed totals'
post('/api/return')
post('/api/return')
assert get('/api/status')['marts'] == s['marts'], 'mart changed before processing'
post('/api/run?stage=all')
updated = get('/api/status')['marts']
assert sum(x['returns'] for x in updated) == 1700, updated
post('/api/run?stage=seed')
post('/api/run?stage=all')
s = get('/api/status')
assert s['monthly'], s
print('End-to-end pipeline checks passed')
