package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"roblox-lookup/pkg/roblox"
	"time"
)

func main() {
	port := flag.String("port", "8080", "Port to serve on")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		*port = envPort
	}

	client := roblox.NewClient()

	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	http.HandleFunc("/api/lookup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		query := r.URL.Query().Get("q")
		if query == "" {
			query = r.URL.Query().Get("username")
		}
		if query == "" {
			query = r.URL.Query().Get("id")
		}

		if query == "" {
			http.Error(w, `{"error":"Missing query parameter 'q' or 'username' or 'id'"}`, http.StatusBadRequest)
			return
		}

		start := time.Now()
		profile, err := client.LookupUser(r.Context(), query)
		duration := time.Since(start)

		if err != nil {
			log.Printf("Lookup error for '%s': %v", query, err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   err.Error(),
				"took_ms": duration.Milliseconds(),
			})
			return
		}

		resp := map[string]interface{}{
			"success": true,
			"took_ms": duration.Milliseconds(),
			"data":    profile,
		}

		json.NewEncoder(w).Encode(resp)
	})

	log.Printf("🚀 Roblox Lookup Server listening on http://localhost:%s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
