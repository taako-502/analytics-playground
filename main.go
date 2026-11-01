package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	cmd := exec.Command("psql", db, "-X", "-q", "-t", "-A", "-v", "ON_ERROR_STOP=1")
	cmd.Stdin = strings.NewReader(query)
	b, e := cmd.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("database: %s: %w", strings.TrimSpace(string(b)), e)
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
	if e = os.WriteFile(filepath.Join(tmp, "manifest.json"), []byte(fmt.Sprintf(`{"batch_id":%q,"extracted_at":%q}`, id, time.Now().UTC().Format(time.RFC3339Nano))), 0644); e != nil {
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
func load() error {
	id, e := latest()
	if e != nil {
		return e
	}
	var q strings.Builder
	q.WriteString("BEGIN; TRUNCATE raw.records;\n")
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
			fmt.Fprintf(&q, "INSERT INTO raw.records VALUES ('%s','%s','%s'::jsonb);\n", id, t, s)
		}
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
	case "extract":
		return extract()
	case "load":
		return load()
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
func status() (map[string]any, error) {
	id, _ := latest()
	out := map[string]any{"batch": id}
	queries := map[string]struct{ db, q string }{"source": {source(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT o.id,o.customer_id,o.paid_at,i.product_id,i.quantity,i.unit_price FROM orders o JOIN order_items i ON i.order_id=o.id ORDER BY o.id LIMIT 30)t`}, "raw": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT batch_id,source_table,payload FROM raw.records ORDER BY source_table LIMIT 30)t`}, "warehouse": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM dwh.fact_order_items ORDER BY id LIMIT 30)t`}, "marts": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM mart.daily_sales ORDER BY day LIMIT 30)t`}, "products": {analytics(), `SELECT coalesce(json_agg(row_to_json(t)),'[]'::json) FROM (SELECT * FROM mart.daily_product_sales ORDER BY day LIMIT 30)t`}}
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
				out["lake"] = x
			}
		}
	}
	return out, nil
}
func main() {
	http.Handle("/", http.FileServer(http.Dir("web")))
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		x, _ := status()
		json.NewEncoder(w).Encode(x)
	})
	http.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if e := pipeline(r.URL.Query().Get("stage")); e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		io.WriteString(w, "ok")
	})
	http.HandleFunc("/api/return", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_, e := sql(source(), `INSERT INTO returns(id,order_item_id,quantity,returned_at) SELECT (SELECT coalesce(max(id),0)+1 FROM returns), 2, 1, now() WHERE NOT EXISTS (SELECT 1 FROM returns WHERE order_item_id=2)`)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		io.WriteString(w, "ok")
	})
	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
