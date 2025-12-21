package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add ngrok bypass header to skip interstitial page
		w.Header().Set("ngrok-skip-browser-warning", "true")

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Removed X-Frame-Options - using CSP frame-ancestors instead (modern standard)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")

		// Add CSP header that allows Discord to embed in iframe and ngrok resources
		w.Header().Set("Content-Security-Policy", "frame-ancestors *; default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob: https:; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.ngrok.com https://*.ngrok.com; style-src 'self' 'unsafe-inline' https://cdn.ngrok.com https://*.ngrok.com; img-src 'self' data: blob: https: https://ngrok.com https://*.ngrok.com; font-src 'self' data: https:;")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create file server for static files
	fs := http.FileServer(http.Dir("./static"))

	handler := corsMiddleware(fs)

	http.HandleFunc("/health", healthCheckHandler)

	http.Handle("/", handler)

	server := &http.Server{
		Addr: ":" + port,
	}

	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Println("Server is running. Press Ctrl+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down gracefully...")
	if err := server.Close(); err != nil {
		log.Printf("Error closing server: %v", err)
	}
}
