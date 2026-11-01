BEGIN;
TRUNCATE dwh.dim_customers, dwh.dim_products, dwh.fact_order_items, dwh.fact_returns;
INSERT INTO dwh.dim_customers SELECT (payload->>'id')::bigint,payload->>'name' FROM raw.records WHERE source_table='customers';
INSERT INTO dwh.dim_products SELECT (payload->>'id')::bigint,payload->>'name',payload->>'category' FROM raw.records WHERE source_table='products';
INSERT INTO dwh.fact_order_items SELECT (i.payload->>'id')::bigint,(i.payload->>'order_id')::bigint,(o.payload->>'customer_id')::bigint,(i.payload->>'product_id')::bigint,(i.payload->>'quantity')::int,(i.payload->>'unit_price')::int,(o.payload->>'paid_at')::timestamptz FROM raw.records i JOIN raw.records o ON o.source_table='orders' AND o.payload->>'id'=i.payload->>'order_id' WHERE i.source_table='order_items';
INSERT INTO dwh.fact_returns SELECT (payload->>'id')::bigint,(payload->>'order_item_id')::bigint,(payload->>'quantity')::int,(payload->>'returned_at')::timestamptz FROM raw.records WHERE source_table='returns';
COMMIT;
