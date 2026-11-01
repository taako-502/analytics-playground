package main

import (
	"analytics-playground/internal/pipeline"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
)

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("web")))
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		x, _ := pipeline.Status()
		json.NewEncoder(w).Encode(x)
	})
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		e := pipeline.Run(r.URL.Query().Get("stage"), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		io.WriteString(w, "ok")
	})
	mux.HandleFunc("/api/trace", func(w http.ResponseWriter, r *http.Request) {
		id, e := strconv.ParseInt(r.URL.Query().Get("order_id"), 10, 64)
		if e != nil || id < 1 {
			http.Error(w, "有効な注文IDを指定してください", 400)
			return
		}
		data, e := pipeline.Trace(id)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})
	mux.HandleFunc("/api/return", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		e := pipeline.AddReturn()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		io.WriteString(w, "ok")
	})
	return mux
}

func main() {
	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", newHandler()))
}
