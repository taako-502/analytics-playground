CREATE SCHEMA raw;
CREATE SCHEMA dwh;
CREATE SCHEMA mart;
CREATE TABLE raw.records (batch_id text NOT NULL, source_table text NOT NULL, payload jsonb NOT NULL);
CREATE TABLE dwh.dim_customers (id bigint PRIMARY KEY, name text NOT NULL);
CREATE TABLE dwh.dim_products (id bigint PRIMARY KEY, name text NOT NULL, category text NOT NULL);
CREATE TABLE dwh.fact_order_items (id bigint PRIMARY KEY, order_id bigint NOT NULL, customer_id bigint NOT NULL, product_id bigint NOT NULL, quantity integer NOT NULL, unit_price integer NOT NULL, paid_at timestamptz NOT NULL);
CREATE TABLE dwh.fact_returns (id bigint PRIMARY KEY, order_item_id bigint NOT NULL, quantity integer NOT NULL, returned_at timestamptz NOT NULL);
