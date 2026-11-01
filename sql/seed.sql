-- Deterministic, repeatable synthetic orders. IDs 1 and 2 remain the hand-calculable fixture.
BEGIN;
INSERT INTO customers SELECT n, '顧客' || n FROM generate_series(3,1000) n ON CONFLICT DO NOTHING;
INSERT INTO products SELECT n, '商品' || n, CASE WHEN n%2=0 THEN '雑貨' ELSE '食品' END FROM generate_series(3,20) n ON CONFLICT DO NOTHING;
INSERT INTO orders SELECT n, 3+(n%998), (CURRENT_DATE - (n%90))::timestamp AT TIME ZONE 'Asia/Tokyo' + interval '12 hours' FROM generate_series(3,10002) n ON CONFLICT DO NOTHING;
INSERT INTO order_items SELECT n,n,3+(n%18),1+(n%3),100+(n%20)*100 FROM generate_series(3,10002) n ON CONFLICT DO NOTHING;
COMMIT;
