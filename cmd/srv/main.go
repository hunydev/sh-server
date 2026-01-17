package main

import (
	"log"
	"os"

	"github.com/hunydev/sh-server/srv"
)

func main() {
	dbPath := getEnv("DB_PATH", "./sh.db")
	hostname := getEnv("HOSTNAME", "sh.huny.dev")
	adminToken := getEnv("ADMIN_TOKEN", "")
	addr := ":" + getEnv("PORT", "8000")

	if adminToken == "" {
		log.Println("WARNING: ADMIN_TOKEN not set, API access will be unrestricted")
	}

	server, err := srv.New(srv.Config{
		DBPath:     dbPath,
		Hostname:   hostname,
		AdminToken: adminToken,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Starting sh.huny.dev server on %s", addr)
	log.Printf("Database: %s", dbPath)
	log.Printf("Hostname: %s", hostname)

	if err := server.Serve(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
