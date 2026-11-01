BEGIN;
CREATE TABLE IF NOT EXISTS mart.daily_sales (day date PRIMARY KEY, gross_sales bigint NOT NULL, returns bigint NOT NULL, net_sales bigint NOT NULL);
CREATE TABLE IF NOT EXISTS mart.daily_product_sales (day date NOT NULL, product_id bigint NOT NULL, product_name text NOT NULL, category text NOT NULL, quantity bigint NOT NULL, gross_sales bigint NOT NULL, PRIMARY KEY(day,product_id));
TRUNCATE mart.daily_sales,mart.daily_product_sales;
INSERT INTO mart.daily_sales WITH events AS (
 SELECT (paid_at AT TIME ZONE 'Asia/Tokyo')::date day, quantity::bigint*unit_price gross, 0::bigint refunds FROM dwh.fact_order_items
 UNION ALL SELECT (r.returned_at AT TIME ZONE 'Asia/Tokyo')::date, 0, r.quantity::bigint*i.unit_price FROM dwh.fact_returns r JOIN dwh.fact_order_items i ON i.id=r.order_item_id
) SELECT day,sum(gross),sum(refunds),sum(gross)-sum(refunds) FROM events GROUP BY day;
INSERT INTO mart.daily_product_sales SELECT (i.paid_at AT TIME ZONE 'Asia/Tokyo')::date,i.product_id,p.name,p.category,sum(i.quantity),sum(i.quantity::bigint*i.unit_price) FROM dwh.fact_order_items i JOIN dwh.dim_products p ON p.id=i.product_id GROUP BY 1,2,3,4;
COMMIT;
