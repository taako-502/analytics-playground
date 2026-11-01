BEGIN;
CREATE TABLE IF NOT EXISTS mart.daily_sales (day date PRIMARY KEY, gross_sales bigint NOT NULL, returns bigint NOT NULL, net_sales bigint NOT NULL);
CREATE TABLE IF NOT EXISTS mart.daily_product_sales (day date NOT NULL, product_id bigint NOT NULL, product_name text NOT NULL, category text NOT NULL, quantity bigint NOT NULL, gross_sales bigint NOT NULL, PRIMARY KEY(day,product_id));
DELETE FROM mart.daily_sales WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR day BETWEEN NULLIF(current_setting('app.range_from',true),'')::date AND NULLIF(current_setting('app.range_to',true),'')::date);
DELETE FROM mart.daily_product_sales WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR day BETWEEN NULLIF(current_setting('app.range_from',true),'')::date AND NULLIF(current_setting('app.range_to',true),'')::date);
INSERT INTO mart.daily_sales WITH events AS (
 SELECT (paid_at AT TIME ZONE 'Asia/Tokyo')::date AS sales_day, quantity::bigint*unit_price AS gross, 0::bigint AS refunds FROM dwh.fact_order_items
 UNION ALL SELECT (r.returned_at AT TIME ZONE 'Asia/Tokyo')::date, 0, r.quantity::bigint*i.unit_price FROM dwh.fact_returns r JOIN dwh.fact_order_items i ON i.id=r.order_item_id
) SELECT sales_day,sum(gross),sum(refunds),sum(gross)-sum(refunds) FROM events WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR sales_day BETWEEN NULLIF(current_setting('app.range_from',true),'')::date AND NULLIF(current_setting('app.range_to',true),'')::date) GROUP BY sales_day;
INSERT INTO mart.daily_product_sales SELECT (i.paid_at AT TIME ZONE 'Asia/Tokyo')::date,i.product_id,p.name,p.category,sum(i.quantity),sum(i.quantity::bigint*i.unit_price) FROM dwh.fact_order_items i JOIN dwh.dim_products p ON p.id=i.product_id WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR (i.paid_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN NULLIF(current_setting('app.range_from',true),'')::date AND NULLIF(current_setting('app.range_to',true),'')::date) GROUP BY 1,2,3,4;
CREATE TABLE IF NOT EXISTS mart.monthly_customer_activity (month date PRIMARY KEY, customers bigint NOT NULL, repeat_customers bigint NOT NULL, repeat_rate numeric);
DELETE FROM mart.monthly_customer_activity WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR month BETWEEN date_trunc('month',NULLIF(current_setting('app.range_from',true),'')::date)::date AND date_trunc('month',NULLIF(current_setting('app.range_to',true),'')::date)::date);
INSERT INTO mart.monthly_customer_activity
WITH customer_months AS (
 SELECT date_trunc('month', paid_at AT TIME ZONE 'Asia/Tokyo')::date AS activity_month, customer_id, count(DISTINCT order_id) orders
 FROM dwh.fact_order_items WHERE (NULLIF(current_setting('app.range_from',true),'') IS NULL OR date_trunc('month',paid_at AT TIME ZONE 'Asia/Tokyo')::date BETWEEN date_trunc('month',NULLIF(current_setting('app.range_from',true),'')::date)::date AND date_trunc('month',NULLIF(current_setting('app.range_to',true),'')::date)::date) GROUP BY 1,2
)
SELECT activity_month,count(*),count(*) FILTER (WHERE orders >= 2),round((count(*) FILTER (WHERE orders >= 2))::numeric / NULLIF(count(*),0),4) FROM customer_months GROUP BY activity_month;
COMMIT;
