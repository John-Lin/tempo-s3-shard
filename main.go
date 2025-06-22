package main

import (
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"

	"tempo-s3-shard/internal/config"
	"tempo-s3-shard/internal/server"
)

func main() {
	// Initialize structured logger with logfmt format
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	
	configFile := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Warn("Failed to load config file, using defaults", "error", err)
		cfg = config.DefaultConfig()
	}

	logger.Info("Starting Tempo S3 Shard Server",
		"listen_addr", cfg.ListenAddr,
		"endpoint", cfg.Endpoint,
		"buckets", cfg.Buckets,
	)
	
	s3Server, err := server.NewTempoS3ShardServer(cfg)
	if err != nil {
		logger.Error("Failed to create Tempo S3 shard server", "error", err)
		log.Fatal("Failed to create server")
	}
	
	if err := http.ListenAndServe(cfg.ListenAddr, s3Server); err != nil {
		logger.Error("Server failed to start", "error", err)
		log.Fatal("Server startup failed")
	}
}