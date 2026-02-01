package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	httpapi "github.com/nanobazaar/relay/internal/http"
	"github.com/nanobazaar/relay/internal/retention"
)

type Config struct {
	HTTPAddr          string
	DBPath            string
	RetentionEnabled  bool
	RetentionInterval time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if err := ensureDBDir(cfg.DBPath); err != nil {
		log.Fatalf("db dir: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	if err := configureSQLite(db); err != nil {
		log.Fatalf("db pragma: %v", err)
	}

	stopRetention := retention.Start(cfg.RetentionEnabled, cfg.RetentionInterval, log.Default())
	defer stopRetention()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	go func() {
		log.Printf("relay listening on %s", cfg.HTTPAddr)
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func loadConfig() (Config, error) {
	addr := os.Getenv("NBR_HTTP_ADDR")
	if addr == "" {
		if port := os.Getenv("PORT"); port != "" {
			addr = ":" + port
		} else {
			addr = ":8080"
		}
	}

	retentionEnabled := parseBoolEnv("NBR_RETENTION_ENABLED", false)
	retentionInterval, err := parseDurationEnv("NBR_RETENTION_INTERVAL", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:          addr,
		DBPath:            envOrDefault("NBR_DB_PATH", "./data/relay.db"),
		RetentionEnabled:  retentionEnabled,
		RetentionInterval: retentionInterval,
	}, nil
}

func configureSQLite(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		return err
	}
	return nil
}

func ensureDBDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func envOrDefault(key, def string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return def
}

func parseBoolEnv(key string, def bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return def
	}
	return parsed
}

func parseDurationEnv(key string, def time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return def, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}
