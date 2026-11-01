#!/usr/bin/env python3
"""Create an EC dashboard after Metabase's first-run setup and database connection."""
import getpass
import json
import os
import sys
import urllib.error
import urllib.request

BASE = os.environ.get('METABASE_URL', 'http://localhost:3000').rstrip('/')

def request(method, path, data=None, session=None):
    body = None if data is None else json.dumps(data).encode()
    headers = {'Content-Type': 'application/json'}
    if session:
        headers['X-Metabase-Session'] = session
    req = urllib.request.Request(BASE + path, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            return json.load(response)
    except urllib.error.HTTPError as e:
        raise RuntimeError(f'{method} {path}: {e.code} {e.read().decode()}') from e

def main():
    user = input('Metabase 管理者メールアドレス: ')
    password = getpass.getpass('パスワード: ')
    session = request('POST', '/api/session', {'username':user, 'password':password})['id']
    dbs = request('GET', '/api/database', session=session)['data']
    choices = [db for db in dbs if db.get('engine') == 'postgres']
    if not choices:
        raise RuntimeError('PostgreSQLデータベースがありません。READMEに従い先に接続してください。')
    for db in choices:
        print(f"{db['id']}: {db['name']}")
    db_id = int(input('分析DBのID: '))
    if db_id not in [db['id'] for db in choices]:
        raise RuntimeError('表示されたDB IDを指定してください')
    dashboard = request('POST', '/api/dashboard', {'name':'EC売上分析', 'description':'Analytics Playground'}, session)
    specs = [
        ('日別売上と返品', 'line', 'SELECT day, gross_sales, returns, net_sales FROM mart.daily_sales ORDER BY day'),
        ('商品別売上', 'bar', 'SELECT product_name, SUM(gross_sales) AS sales FROM mart.daily_product_sales GROUP BY product_name ORDER BY sales DESC'),
        ('月別リピート率', 'line', 'SELECT month, repeat_rate FROM mart.monthly_customer_activity ORDER BY month'),
    ]
    cards=[]
    for name, display, query in specs:
        card = request('POST','/api/card',{'name':name,'display':display,'visualization_settings':{},'dataset_query':{'database':db_id,'type':'native','native':{'query':query}}},session)
        cards.append(card['id'])
    layout=[{'id':-(i+1),'card_id':card,'row':(i//2)*7,'col':(i%2)*12,'size_x':12,'size_y':7} for i,card in enumerate(cards)]
    request('PUT',f"/api/dashboard/{dashboard['id']}",{'dashcards':layout},session)
    print(f"作成しました: {BASE}/dashboard/{dashboard['id']}")
    print('再実行すると別のダッシュボードが作成されます。')

if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, ValueError) as e:
        print(f'エラー: {e}',file=sys.stderr)
        sys.exit(1)
