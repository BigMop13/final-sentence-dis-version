package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create file server for static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	server := &http.Server{
		Addr: ":8080",
	}

	go func() {
		log.Println("HTTP server listening on :8080")
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
