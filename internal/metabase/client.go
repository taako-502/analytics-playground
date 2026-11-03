package metabase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct{ URL, Session string }

func (c *Client) Request(method, path string, data any, out any) error {
	var body io.Reader
	if data != nil {
		b, e := json.Marshal(data)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, strings.TrimRight(c.URL, "/")+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Session != "" {
		req.Header.Set("X-Metabase-Session", c.Session)
	}
	res, e := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, b)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
func (c *Client) Login(user, password string) error {
	var r struct {
		ID string `json:"id"`
	}
	if e := c.Request("POST", "/api/session", map[string]string{"username": user, "password": password}, &r); e != nil {
		return e
	}
	c.Session = r.ID
	return nil
}

type Database struct {
	ID           int `json:"id"`
	Name, Engine string
}

func (c *Client) Databases() ([]Database, error) {
	var r struct{ Data []Database }
	e := c.Request("GET", "/api/database", nil, &r)
	return r.Data, e
}
func (c *Client) CreateDashboard(db int) (int, error) {
	databases, e := c.Databases()
	if e != nil {
		return 0, e
	}
	found := false
	for _, d := range databases {
		if d.ID == db && d.Engine == "postgres" {
			found = true
		}
	}
	if !found {
		return 0, fmt.Errorf("PostgreSQLの分析DB IDを指定してください")
	}
	var dashboard struct{ ID int }
	if e = c.Request("POST", "/api/dashboard", map[string]string{"name": "EC売上分析", "description": "Analytics Playground"}, &dashboard); e != nil {
		return 0, e
	}
	specs := []struct {
		name, display, query, dimension, axis string
		metrics                               []string
	}{
		{"日別売上と返品", "line", `SELECT day AS "日付", gross_sales AS "売上", net_sales AS "純売上", returns AS "返品" FROM mart.daily_sales ORDER BY day`, "日付", "金額（円）", []string{"売上", "純売上", "返品"}},
		{"商品別売上 TOP 10", "bar", `SELECT product_name AS "商品", SUM(gross_sales) AS "売上" FROM mart.daily_product_sales GROUP BY product_id,product_name ORDER BY SUM(gross_sales) DESC,product_id LIMIT 10`, "商品", "売上（円・返品控除前）", []string{"売上"}},
		{"月別リピート率", "line", `SELECT month AS "月", repeat_rate * 100 AS "リピート率（%）" FROM mart.monthly_customer_activity ORDER BY month`, "月", "リピート率（%）", []string{"リピート率（%）"}},
	}
	var layout []map[string]any
	for i, s := range specs {
		var card struct{ ID int }
		e = c.Request("POST", "/api/card", map[string]any{"name": s.name, "display": s.display, "visualization_settings": map[string]any{"graph.dimensions": []string{s.dimension}, "graph.metrics": s.metrics, "graph.y_axis.title_text": s.axis, "graph.x_axis.title_text": s.dimension, "graph.show_values": i == 2}, "dataset_query": map[string]any{"database": db, "type": "native", "native": map[string]string{"query": s.query}}}, &card)
		if e != nil {
			return 0, e
		}
		row, col, width := 0, 0, 24
		if i > 0 {
			row, col, width = 8, (i-1)*12, 12
		}
		layout = append(layout, map[string]any{"id": -(i + 1), "card_id": card.ID, "row": row, "col": col, "size_x": width, "size_y": 8})
	}
	e = c.Request("PUT", fmt.Sprintf("/api/dashboard/%d", dashboard.ID), map[string]any{"dashcards": layout}, nil)
	return dashboard.ID, e
}
