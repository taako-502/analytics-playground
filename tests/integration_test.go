//go:build integration

package tests

import (
	"analytics-playground/internal/metabase"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func endpoint(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type obj = map[string]any

func call(t *testing.T, c *metabase.Client, method, path string, data any, out any) {
	t.Helper()
	if e := c.Request(method, path, data, out); e != nil {
		t.Fatal(e)
	}
}
func wait(t *testing.T, c *metabase.Client, path string, attempts int) {
	t.Helper()
	for i := 0; i < attempts; i++ {
		if c.Request("GET", path, nil, nil) == nil {
			return
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatal("service not ready")
}
func TestPipeline(t *testing.T) {
	c := &metabase.Client{URL: endpoint("TEST_APP_URL", "http://localhost:8080")}
	wait(t, c, "/api/status", 60)
	post := func(p string) { call(t, c, "POST", p, nil, nil) }
	get := func(p string) obj { var v obj; call(t, c, "GET", p, nil, &v); return v }
	stage := func(s string) { post("/api/run?stage=" + s) }
	sales := func(s obj) map[string]obj {
		r := map[string]obj{}
		for _, x := range s["marts"].([]any) {
			v := x.(map[string]any)
			r[v["day"].(string)] = v
		}
		return r
	}
	stage("all")
	s := get("/api/status")
	v := sales(s)
	if len(s["chartDaily"].([]any)) != len(s["marts"].([]any)) {
		t.Fatal("chart daily rows mismatch")
	}
	var chartSales float64
	for _, row := range s["chartProducts"].([]any) {
		chartSales += row.(map[string]any)["sales"].(float64)
	}
	if chartSales != 2200 {
		t.Fatalf("chart product sales = %v", chartSales)
	}
	if v["2026-10-01"]["gross_sales"] != float64(1000) || v["2026-10-02"]["gross_sales"] != float64(1200) || v["2026-10-03"]["returns"] != float64(500) {
		t.Fatal(v)
	}
	trace := get("/api/trace?order_id=1")
	for _, k := range []string{"source", "warehouse", "lake", "raw"} {
		if len(trace[k].([]any)) == 0 {
			t.Fatal(trace)
		}
	}
	stage("all")
	if !reflect.DeepEqual(get("/api/status")["marts"], s["marts"]) {
		t.Fatal("rerun changed totals")
	}
	post("/api/return")
	post("/api/return")
	if !reflect.DeepEqual(get("/api/status")["marts"], s["marts"]) {
		t.Fatal("premature mart update")
	}
	stage("all")
	total := 0.
	for _, row := range sales(get("/api/status")) {
		total += row["returns"].(float64)
	}
	trace = get("/api/trace?order_id=2")
	for _, k := range []string{"source_returns", "raw_returns", "dwh_returns", "lake_returns"} {
		if len(trace[k].([]any)) != 1 {
			t.Fatal(k, trace)
		}
	}
	if total != 1700 {
		t.Fatal(total)
	}
	cmd := exec.Command("docker", "compose", "exec", "-T", "source", "psql", "-U", "postgres", "-d", "source", "-v", "ON_ERROR_STOP=1", "-c", "UPDATE order_items SET unit_price=800 WHERE id=1; DELETE FROM returns WHERE id=1")
	cmd.Dir = ".."
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("%s: %v", out, e)
	}
	for _, x := range []string{"extract", "load-diff", "load-diff", "warehouse", "marts-range&from=2026-10-01&to=2026-10-03"} {
		stage(x)
	}
	trace = get("/api/trace?order_id=1")
	if trace["warehouse"].([]any)[0].(map[string]any)["unit_price"] != float64(800) {
		t.Fatal(trace)
	}
	v = sales(get("/api/status"))
	if v["2026-10-01"]["gross_sales"] != float64(1600) {
		t.Fatal(v)
	}
	if row := v["2026-10-03"]; row != nil && row["returns"] != float64(0) {
		t.Fatal(row)
	}
	stage("seed")
	stage("all")
	if len(get("/api/status")["monthly"].([]any)) == 0 {
		t.Fatal("empty monthly mart")
	}
}
func TestMetabase(t *testing.T) {
	app := &metabase.Client{URL: endpoint("TEST_APP_URL", "http://localhost:8080")}
	wait(t, app, "/api/status", 60)
	call(t, app, "POST", "/api/run?stage=all", nil, nil)
	c := &metabase.Client{URL: endpoint("TEST_METABASE_URL", "http://localhost:3000")}
	wait(t, c, "/api/health", 120)
	var props obj
	call(t, c, "GET", "/api/session/properties", nil, &props)
	var setup struct{ ID string }
	call(t, c, "POST", "/api/setup", obj{"token": props["setup-token"], "user": obj{"first_name": "Test", "last_name": "Admin", "email": "admin@example.com", "password": "ExamplePass123!"}, "prefs": obj{"site_name": "Test"}}, &setup)
	if e := c.Login("admin@example.com", "ExamplePass123!"); e != nil {
		t.Fatal(e)
	}
	var db struct{ ID int }
	call(t, c, "POST", "/api/database", obj{"name": "Analytics", "engine": "postgres", "details": obj{"host": "analytics", "port": 5432, "dbname": "analytics", "user": "postgres", "password": "playground", "ssl": false}}, &db)
	id, e := c.CreateDashboard(db.ID)
	if e != nil {
		t.Fatal(e)
	}
	var full obj
	call(t, c, "GET", fmt.Sprintf("/api/dashboard/%d", id), nil, &full)
	cards, ok := full["dashcards"]
	if !ok {
		cards = full["ordered_cards"]
	}
	if len(cards.([]any)) != 3 {
		t.Fatal(full)
	}
	for _, v := range cards.([]any) {
		card := v.(map[string]any)
		var result obj
		call(t, c, "POST", fmt.Sprintf("/api/card/%.0f/query", card["card_id"].(float64)), obj{}, &result)
		if result["status"] != "completed" {
			t.Fatal(result)
		}
		data := result["data"].(map[string]any)
		if len(data["rows"].([]any)) == 0 {
			t.Fatal("empty card result", card)
		}
	}
}
