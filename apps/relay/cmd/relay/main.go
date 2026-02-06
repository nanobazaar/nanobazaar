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
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	"github.com/nanobazaar/relay/internal/auth"
	httpapi "github.com/nanobazaar/relay/internal/http"
	"github.com/nanobazaar/relay/internal/metrics"
	"github.com/nanobazaar/relay/internal/ratelimit"
	"github.com/nanobazaar/relay/internal/retention"
	"github.com/nanobazaar/relay/internal/store"
)

type Config struct {
	HTTPAddr          string
	AdminAddr         string
	AdminToken        string
	AdminPublic       bool
	DBPath            string
	RetentionEnabled  bool
	RetentionInterval time.Duration
	MetricsAddr       string
	HealthPublic      bool
	MigrateOnStart    bool
	RateLimits        RateLimitConfig
}

type RateLimitConfig struct {
	PollRPS      float64
	PollBurst    int
	OfferRPS     float64
	OfferBurst   int
	WritesRPS    float64
	WritesBurst  int
	PayloadRPS   float64
	PayloadBurst int
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

	if cfg.MigrateOnStart {
		if err := runMigrations(db); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	} else {
		log.Printf("migrations disabled (NBR_MIGRATE_ON_START=false)")
	}

	store := store.New(db)
	metricsRegistry := metrics.NewRegistry()
	verifier := auth.NewVerifier(store)
	verifier.Metrics = metricsRegistry

	limiter := ratelimit.NewLimiter(ratelimit.Config{
		PollAck: ratelimit.BucketConfig{
			Rate:  cfg.RateLimits.PollRPS,
			Burst: cfg.RateLimits.PollBurst,
		},
		OfferSearch: ratelimit.BucketConfig{
			Rate:  cfg.RateLimits.OfferRPS,
			Burst: cfg.RateLimits.OfferBurst,
		},
		Writes: ratelimit.BucketConfig{
			Rate:  cfg.RateLimits.WritesRPS,
			Burst: cfg.RateLimits.WritesBurst,
		},
		PayloadFetch: ratelimit.BucketConfig{
			Rate:  cfg.RateLimits.PayloadRPS,
			Burst: cfg.RateLimits.PayloadBurst,
		},
	})

	stopRetention := retention.Start(cfg.RetentionEnabled, cfg.RetentionInterval, log.Default(), store)
	defer stopRetention()

	streamHub := httpapi.NewStreamHub(store)

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Verifier:     verifier,
		Store:        store,
		Metrics:      metricsRegistry,
		Limiter:      limiter,
		HealthPublic: cfg.HealthPublic,
		StreamHub:    streamHub,
		AdminToken:   cfg.AdminToken,
		AdminPublic:  cfg.AdminPublic,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
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

	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		payloadStats := func(ctx context.Context) (int64, int64, error) {
			var pending int64
			var bytes int64
			row := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN fetched_at IS NULL THEN 1 ELSE 0 END), 0), COALESCE(SUM(LENGTH(ciphertext_b64)), 0) FROM payloads`)
			if err := row.Scan(&pending, &bytes); err != nil {
				return 0, 0, err
			}
			return pending, bytes, nil
		}
		metricsServer = &http.Server{
			Addr:              cfg.MetricsAddr,
			Handler:           metrics.NewHandler(metricsRegistry, payloadStats),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			log.Printf("metrics listening on %s", cfg.MetricsAddr)
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("metrics server: %v", err)
			}
		}()
	}

	var adminServer *http.Server
	if cfg.AdminAddr != "" {
		adminRouter := httpapi.NewAdminRouter(httpapi.AdminRouterConfig{
			Store:      store,
			Metrics:    metricsRegistry,
			AdminToken: cfg.AdminToken,
			StreamHub:  streamHub,
			Mode:       "separate_listener",
		})
		adminServer = &http.Server{
			Addr:              cfg.AdminAddr,
			Handler:           adminRouter,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			log.Printf("admin listening on %s", cfg.AdminAddr)
			if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("admin server: %v", err)
			}
		}()
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Printf("metrics shutdown: %v", err)
		}
	}
	if adminServer != nil {
		if err := adminServer.Shutdown(ctx); err != nil {
			log.Printf("admin shutdown: %v", err)
		}
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

	metricsAddr := os.Getenv("NBR_METRICS_ADDR")
	healthPublic := parseBoolEnv("NBR_HEALTH_PUBLIC", true)
	migrateOnStart := parseBoolEnv("NBR_MIGRATE_ON_START", true)
	adminAddr := strings.TrimSpace(os.Getenv("NBR_ADMIN_ADDR"))
	adminPublic := parseBoolEnv("NBR_ADMIN_PUBLIC", false)
	adminToken := os.Getenv("NBR_ADMIN_TOKEN")
	if (adminAddr != "" || adminPublic) && strings.TrimSpace(adminToken) == "" {
		return Config{}, fmt.Errorf("NBR_ADMIN_TOKEN required when NBR_ADMIN_ADDR is set or NBR_ADMIN_PUBLIC=true")
	}

	rateLimits := RateLimitConfig{
		PollRPS:      parseFloatEnv("NBR_RL_POLL_RPS", 5),
		PollBurst:    parseIntEnv("NBR_RL_POLL_BURST", 10),
		OfferRPS:     parseFloatEnv("NBR_RL_OFFER_RPS", 2),
		OfferBurst:   parseIntEnv("NBR_RL_OFFER_BURST", 5),
		WritesRPS:    parseFloatEnv("NBR_RL_WRITES_RPS", 2),
		WritesBurst:  parseIntEnv("NBR_RL_WRITES_BURST", 5),
		PayloadRPS:   parseFloatEnv("NBR_RL_PAYLOAD_RPS", 5),
		PayloadBurst: parseIntEnv("NBR_RL_PAYLOAD_BURST", 10),
	}

	return Config{
		HTTPAddr:          addr,
		AdminAddr:         adminAddr,
		AdminToken:        adminToken,
		AdminPublic:       adminPublic,
		DBPath:            envOrDefault("NBR_DB_PATH", "./data/relay.db"),
		RetentionEnabled:  retentionEnabled,
		RetentionInterval: retentionInterval,
		MetricsAddr:       metricsAddr,
		HealthPublic:      healthPublic,
		MigrateOnStart:    migrateOnStart,
		RateLimits:        rateLimits,
	}, nil
}

func runMigrations(db *sql.DB) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	migrationsDir := filepath.Join("db", "migrations")
	if _, err := os.Stat(migrationsDir); err != nil {
		return fmt.Errorf("migrations dir: %w", err)
	}
	return goose.Up(db, migrationsDir)
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

func parseIntEnv(key string, def int) int {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return def
	}
	return parsed
}

func parseFloatEnv(key string, def float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return def
	}
	return parsed
}
