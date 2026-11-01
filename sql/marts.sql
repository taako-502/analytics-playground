BEGIN;
CREATE TABLE IF NOT EXISTS mart.daily_sales (day date PRIMARY KEY, gross_sales bigint NOT NULL, returns bigint NOT NULL, net_sales bigint NOT NULL);
CREATE TABLE IF NOT EXISTS mart.daily_product_sales (day date NOT NULL, product_id bigint NOT NULL, product_name text NOT NULL, category text NOT NULL, quantity bigint NOT NULL, gross_sales bigint NOT NULL, PRIMARY KEY(day,product_id));
TRUNCATE mart.daily_sales,mart.daily_product_sales;
INSERT INTO mart.daily_sales WITH events AS (
 SELECT (paid_at AT TIME ZONE 'Asia/Tokyo')::date AS sales_day, quantity::bigint*unit_price AS gross, 0::bigint AS refunds FROM dwh.fact_order_items
 UNION ALL SELECT (r.returned_at AT TIME ZONE 'Asia/Tokyo')::date, 0, r.quantity::bigint*i.unit_price FROM dwh.fact_returns r JOIN dwh.fact_order_items i ON i.id=r.order_item_id
) SELECT sales_day,sum(gross),sum(refunds),sum(gross)-sum(refunds) FROM events GROUP BY sales_day;
INSERT INTO mart.daily_product_sales SELECT (i.paid_at AT TIME ZONE 'Asia/Tokyo')::date,i.product_id,p.name,p.category,sum(i.quantity),sum(i.quantity::bigint*i.unit_price) FROM dwh.fact_order_items i JOIN dwh.dim_products p ON p.id=i.product_id GROUP BY 1,2,3,4;
CREATE TABLE IF NOT EXISTS mart.monthly_customer_activity (month date PRIMARY KEY, customers bigint NOT NULL, repeat_customers bigint NOT NULL, repeat_rate numeric);
TRUNCATE mart.monthly_customer_activity;
INSERT INTO mart.monthly_customer_activity
WITH customer_months AS (
 SELECT date_trunc('month', paid_at AT TIME ZONE 'Asia/Tokyo')::date AS activity_month, customer_id, count(DISTINCT order_id) orders
 FROM dwh.fact_order_items GROUP BY 1,2
)
SELECT activity_month,count(*),count(*) FILTER (WHERE orders >= 2),round((count(*) FILTER (WHERE orders >= 2))::numeric / NULLIF(count(*),0),4) FROM customer_months GROUP BY activity_month;
COMMIT;
