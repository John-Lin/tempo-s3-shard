package main

import (
	"flag"
	"log"
	"net/http"

	"tempo-s3-shard/internal/config"
	"tempo-s3-shard/internal/server"
)

func main() {
	configFile := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Printf("Failed to load config file, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	log.Printf("Starting Tempo S3 Shard Server on %s", cfg.ListenAddr)
	log.Printf("Backend S3 endpoint: %s", cfg.Endpoint)
	log.Printf("Managing buckets: %v", cfg.Buckets)
	
	s3Server, err := server.NewTempoS3ShardServer(cfg)
	if err != nil {
		log.Fatal("Failed to create Tempo S3 shard server:", err)
	}
	
	if err := http.ListenAndServe(cfg.ListenAddr, s3Server); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}