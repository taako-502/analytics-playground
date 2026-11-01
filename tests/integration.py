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
# Compare snapshot-based incremental updates, including changed and deleted records.
import subprocess

def db(query):
    subprocess.run(['docker','compose','exec','-T','source','psql','-U','postgres','-d','source','-v','ON_ERROR_STOP=1','-c',query],check=True,stdout=subprocess.DEVNULL)

db("UPDATE order_items SET unit_price=800 WHERE id=1; DELETE FROM returns WHERE id=1")
post('/api/run?stage=extract')
post('/api/run?stage=load-diff')
post('/api/run?stage=load-diff')
post('/api/run?stage=warehouse')
post('/api/run?stage=marts-range&from=2026-10-01&to=2026-10-03')
s = get('/api/status')
assert get('/api/trace?order_id=1')['warehouse'][0]['unit_price'] == 800
sales = {x['day']:x for x in s['marts']}
assert sales['2026-10-01']['gross_sales'] >= 1600, sales
assert not sales.get('2026-10-03',{}).get('returns'), sales
print('Incremental and range checks passed')

post('/api/run?stage=seed')
post('/api/run?stage=all')
s = get('/api/status')
assert s['monthly'], s
print('Seed checks passed')
