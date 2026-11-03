package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex
var tables = []string{"customers", "products", "orders", "order_items", "returns"}

func env(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}
func source() string {
	return env("SOURCE_URL", "postgres://postgres:playground@localhost:15432/source")
}
func analytics() string {
	return env("ANALYTICS_URL", "postgres://postgres:playground@localhost:15433/analytics")
}
func lake() string { return env("LAKE_DIR", "lake") }
func sql(db, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "psql", db, "-X", "-q", "-t", "-A", "-v", "ON_ERROR_STOP=1")
	cmd.Stdin = strings.NewReader(query)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return "", fmt.Errorf("database: %s: %w", strings.TrimSpace(stderr.String()), e)
	}
	return strings.TrimSpace(string(b)), nil
}
func extract() error {
	if e := os.MkdirAll(lake(), 0755); e != nil {
		return e
	}
	id := time.Now().UTC().Format("20060102T150405.000000000Z")
	tmp, e := os.MkdirTemp(lake(), ".batch-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	// A single repeatable-read transaction produces a consistent snapshot of all tables.
	var q strings.Builder
	q.WriteString("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;\n")
	for _, t := range tables {
		fmt.Fprintf(&q, "SELECT json_build_object('table','%s','rows',coalesce(json_agg(row_to_json(t)),'[]'::json))::text FROM %s t;\n", t, t)
	}
	q.WriteString("COMMIT;")
	out, e := sql(source(), q.String())
	if e != nil {
		return e
	}
	lines := strings.Split(out, "\n")
	if len(lines) != len(tables) {
		return errors.New("unexpected extraction output")
	}
	for i, line := range lines {
		var data struct {
			Rows json.RawMessage `json:"rows"`
		}
		if e = json.Unmarshal([]byte(line), &data); e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(tmp, tables[i]+".json"), data.Rows, 0644); e != nil {
			return e
		}
	}
	if e = os.WriteFile(filepath.Join(tmp, "manifest.json"), fmt.Appendf(nil, `{"batch_id":%q,"extracted_at":%q}`, id, time.Now().UTC().Format(time.RFC3339Nano)), 0644); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(lake(), id))
}
func latest() (string, error) {
	entries, e := os.ReadDir(lake())
	if e != nil {
		return "", e
	}
	var names []string
	for _, d := range entries {
		if d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			names = append(names, d.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "", errors.New("先に抽出を実行してください")
	}
	return names[len(names)-1], nil
}
func load() error { return loadMode(false) }
func loadMode(diff bool) error {
	id, e := latest()
	if e != nil {
		return e
	}
	var q strings.Builder
	if diff {
		q.WriteString("BEGIN; CREATE TEMP TABLE incoming (LIKE raw.records) ON COMMIT DROP;\n")
	} else {
		q.WriteString("BEGIN; TRUNCATE raw.records;\n")
	}
	target := "raw.records"
	if diff {
		target = "incoming"
	}
	for _, t := range tables {
		b, e := os.ReadFile(filepath.Join(lake(), id, t+".json"))
		if e != nil {
			return e
		}
		var rows []json.RawMessage
		if e = json.Unmarshal(b, &rows); e != nil {
			return e
		}
		for _, row := range rows {
			s := strings.ReplaceAll(string(row), "'", "''")
			fmt.Fprintf(&q, "INSERT INTO %s VALUES ('%s','%s','%s'::jsonb);\n", target, id, t, s)
		}
	}
	if diff {
		q.WriteString(`
DELETE FROM raw.records r WHERE NOT EXISTS (SELECT 1 FROM incoming i WHERE i.source_table=r.source_table AND i.payload->>'id'=r.payload->>'id');
DELETE FROM raw.records r USING incoming i WHERE i.source_table=r.source_table AND i.payload->>'id'=r.payload->>'id' AND i.payload IS DISTINCT FROM r.payload;
INSERT INTO raw.records SELECT i.* FROM incoming i WHERE NOT EXISTS (SELECT 1 FROM raw.records r WHERE r.source_table=i.source_table AND r.payload->>'id'=i.payload->>'id');
`)
	}
	q.WriteString("COMMIT;")
	_, e = sql(analytics(), q.String())
	return e
}
func runSQL(path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	_, e = sql(analytics(), string(b))
	return e
}
func pipeline(stage string) error {
	switch stage {
	case "seed":
		b, e := os.ReadFile("sql/seed.sql")
		if e != nil {
			return e
		}
		_, e = sql(source(), string(b))
		return e
	case "extract":
		return extract()
	case "load":
		return load()
	case "load-diff":
		return loadMode(true)
	case "warehouse":
		return runSQL("sql/warehouse.sql")
	case "marts":
		return runSQL("sql/marts.sql")
	case "all":
		for _, s := range []string{"extract", "load", "warehouse", "marts"} {
			if e := pipeline(s); e != nil {
				return fmt.Errorf("%s: %w", s, e)
			}
		}
		return nil
	}
	return errors.New("unknown stage")
}
func queryJSON(db, q string) (any, error) {
	v, e := sql(db, q)
	if e != nil {
		return nil, e
	}
	var x any
	e = json.Unmarshal([]byte(v), &x)
	return x, e
}

// Status は各データ層のプレビューと実行履歴を返します。
func Status() (map[string]any, error) {
	id, _ := latest()
	out := map[string]any{"batch": id, "preview_note": "業務DB・raw・DWH・レイクは最大30行。マートは新しい日付から最大1000行。全件はMetabaseで確認できます。"}
	if b, e := os.ReadFile(filepath.Join(lake(), "runs.json")); e == nil {
		var v any
		if json.Unmarshal(b, &v) == nil {
			out["history"] = v
		}
	}
	queries := map[string]struct{ db, q string }{"source": {source(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT o.id,o.customer_id,o.paid_at,i.product_id,i.quantity,i.unit_price FROM orders o JOIN order_items i ON i.order_id=o.id ORDER BY o.id LIMIT 30)t`}, "raw": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT batch_id,source_table,payload FROM raw.records ORDER BY source_table LIMIT 30)t`}, "warehouse": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM dwh.fact_order_items ORDER BY id LIMIT 30)t`}, "marts": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM mart.daily_sales ORDER BY day DESC LIMIT 1000)t`}, "monthly": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM mart.monthly_customer_activity ORDER BY month DESC LIMIT 1000)t`}, "products": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM mart.daily_product_sales ORDER BY day DESC LIMIT 1000)t`}}
	for k, v := range queries {
		x, e := queryJSON(v.db, v.q)
		if e != nil {
			out[k] = []any{}
			out[k+"Error"] = e.Error()
		} else {
			out[k] = x
		}
	}
	if id != "" {
		b, e := os.ReadFile(filepath.Join(lake(), id, "orders.json"))
		if e == nil {
			var x any
			if json.Unmarshal(b, &x) == nil {
				if rows, ok := x.([]any); ok && len(rows) > 30 {
					x = rows[:30]
				}
				out["lake"] = x
			}
		}
	}
	return out, nil
}

// recordRun keeps a bounded history independent of database availability.
func recordRun(stage string, started time.Time, runErr error) error {
	if e := os.MkdirAll(lake(), 0755); e != nil {
		return e
	}
	path := filepath.Join(lake(), "runs.json")
	type entry struct {
		Stage    string `json:"stage"`
		Started  string `json:"started"`
		Finished string `json:"finished"`
		Error    string `json:"error,omitempty"`
	}
	var history []entry
	b, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if readErr == nil {
		if e := json.Unmarshal(b, &history); e != nil {
			return e
		}
	}
	item := entry{Stage: stage, Started: started.Format(time.RFC3339), Finished: time.Now().UTC().Format(time.RFC3339)}
	if runErr != nil {
		item.Error = runErr.Error()
	}
	history = append(history, item)
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	b, e := json.Marshal(history)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(lake(), ".history-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}

// Trace は注文IDに対応する各データ層の情報を返します。
func Trace(id int64) (map[string]any, error) {
	result := map[string]any{"order_id": id}
	order := strconv.FormatInt(id, 10)
	queries := map[string]struct{ db, q string }{
		"source":    {source(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT o.id,o.customer_id,o.paid_at,i.id item_id,i.product_id,i.quantity,i.unit_price FROM orders o JOIN order_items i ON i.order_id=o.id WHERE o.id=` + order + `) t`},
		"raw":       {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT batch_id,source_table,payload FROM raw.records WHERE (source_table='orders' AND payload->>'id'='` + order + `') OR (source_table='order_items' AND payload->>'order_id'='` + order + `')) t`},
		"warehouse": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM dwh.fact_order_items WHERE order_id=` + order + `) t`},
		"mart":      {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT m.* FROM mart.daily_sales m WHERE m.day IN (SELECT (paid_at AT TIME ZONE 'Asia/Tokyo')::date FROM dwh.fact_order_items WHERE order_id=` + order + ` UNION SELECT (r.returned_at AT TIME ZONE 'Asia/Tokyo')::date FROM dwh.fact_returns r JOIN dwh.fact_order_items i ON i.id=r.order_item_id WHERE i.order_id=` + order + `)) t`},
	}
	queries["source_returns"] = struct{ db, q string }{source(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT r.* FROM returns r JOIN order_items i ON i.id=r.order_item_id WHERE i.order_id=` + order + `)t`}
	queries["dwh_returns"] = struct{ db, q string }{analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT r.* FROM dwh.fact_returns r JOIN dwh.fact_order_items i ON i.id=r.order_item_id WHERE i.order_id=` + order + `)t`}
	queries["raw_returns"] = struct{ db, q string }{analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT r.* FROM raw.records r JOIN raw.records i ON i.source_table='order_items' AND i.payload->>'id'=r.payload->>'order_item_id' WHERE r.source_table='returns' AND i.payload->>'order_id'='` + order + `')t`}

	for k, q := range queries {
		x, e := queryJSON(q.db, q.q)
		if e != nil {
			return nil, e
		}
		result[k] = x
	}
	batch, e := latest()
	if e == nil {
		b, e := os.ReadFile(filepath.Join(lake(), batch, "orders.json"))
		if e == nil {
			var rows []map[string]any
			if json.Unmarshal(b, &rows) == nil {
				var found []map[string]any
				for _, r := range rows {
					if n, ok := r["id"].(float64); ok && int64(n) == id {
						found = append(found, r)
					}
				}
				result["lake"] = found
				result["batch"] = batch
			}
		}
	}
	if batch != "" {
		var items, returns []map[string]json.RawMessage
		ib, ie := os.ReadFile(filepath.Join(lake(), batch, "order_items.json"))
		rb, re := os.ReadFile(filepath.Join(lake(), batch, "returns.json"))
		if ie == nil && re == nil && json.Unmarshal(ib, &items) == nil && json.Unmarshal(rb, &returns) == nil {
			ids := map[string]bool{}
			for _, i := range items {
				if string(i["order_id"]) == order {
					ids[string(i["id"])] = true
				}
			}
			selected := []map[string]json.RawMessage{}
			for _, r := range returns {
				if ids[string(r["order_item_id"])] {
					selected = append(selected, r)
				}
			}
			result["lake_returns"] = selected
		}
	}
	return result, nil
}

// runRange refreshes whole days and every intersecting month, atomically.
func runRange(from, to string) error {
	a, e := time.Parse("2006-01-02", from)
	if e != nil {
		return errors.New("開始日が不正です")
	}
	b, e := time.Parse("2006-01-02", to)
	if e != nil || b.Before(a) {
		return errors.New("終了日が不正です")
	}
	data, e := os.ReadFile("sql/marts.sql")
	if e != nil {
		return e
	}
	q := fmt.Sprintf("SET app.range_from='%s'; SET app.range_to='%s';\n", a.Format("2006-01-02"), b.Format("2006-01-02")) + string(data)
	_, e = sql(analytics(), q)
	return e
}

// Run は指定された処理を直列に実行し、成功・失敗を履歴に保存します。
// from と to は marts-range の場合に使用します。
func Run(stage, from, to string) error {
	mu.Lock()
	defer mu.Unlock()
	started := time.Now().UTC()
	var err error
	if stage == "marts-range" {
		err = runRange(from, to)
	} else {
		err = pipeline(stage)
	}
	if historyErr := recordRun(stage, started, err); historyErr != nil {
		err = errors.Join(err, fmt.Errorf("履歴保存: %w", historyErr))
	}
	return err
}

// AddReturn は学習用の返品を追加します。同じ明細には重複して追加しません。
func AddReturn() error {
	mu.Lock()
	defer mu.Unlock()
	_, err := sql(source(), `INSERT INTO returns(id,order_item_id,quantity,returned_at) SELECT (SELECT coalesce(max(id),0)+1 FROM returns), 2, 1, now() WHERE NOT EXISTS (SELECT 1 FROM returns WHERE order_item_id=2)`)
	return err
}
