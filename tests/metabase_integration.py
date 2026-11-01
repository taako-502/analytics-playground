import builtins
import importlib.util
import time
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('dashboard','scripts/metabase_dashboard.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
for _ in range(120):
    try:
        props=m.request('GET','/api/session/properties')
        if props.get('setup-token'): break
    except (OSError,RuntimeError): pass
    time.sleep(3)
else: raise RuntimeError('Metabase did not start')
admin={'first_name':'Test','last_name':'Admin','email':'admin@example.com','password':'ExamplePass123!'}
setup=m.request('POST','/api/setup',{'token':props['setup-token'],'user':admin,'prefs':{'site_name':'Test'}})
session=setup['id']
db=m.request('POST','/api/database',{'name':'Analytics','engine':'postgres','details':{'host':'analytics','port':5432,'dbname':'analytics','user':'postgres','password':'playground','ssl':False}},session)
with patch.object(builtins,'input',side_effect=['admin@example.com',str(db['id'])]), patch.object(m.getpass,'getpass',return_value=admin['password']):
    m.main()
dashboards=m.request('GET','/api/dashboard',session=session)
assert any(d['name']=='EC売上分析' for d in dashboards), dashboards
print('Metabase dashboard created')
