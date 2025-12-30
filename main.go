package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/0x0Glitch/alerts"
	"github.com/0x0Glitch/config"
	"github.com/0x0Glitch/workers"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: .env file not loaded: %v", err)
	}

	// Load configuration
	cfg := config.LoadOrDefault("config.json")
	log.Println("loaded configuration")

	// Validate required environment variables
	alchemyKey := os.Getenv("ALCHEMY_PRICE_API_KEY")
	if alchemyKey == "" {
		log.Fatal("ALCHEMY_PRICE_API_KEY is required")
	}

	// Initialize alert service
	alertService := alerts.New(
		os.Getenv("TELEGRAM_BUSINESS_BOT_TOKEN"),
		os.Getenv("TELEGRAM_BUSINESS_CHAT_ID"),
		os.Getenv("TELEGRAM_DEVELOPER_BOT_TOKEN"),
		os.Getenv("TELEGRAM_DEVELOPER_CHAT_ID"),
	)

	if alertService.BusinessBotToken == "" || alertService.BusinessChatID == "" {
		log.Println("warning: business alerts not configured")
	}
	if alertService.DeveloperBotToken == "" || alertService.DeveloperChatID == "" {
		log.Println("warning: developer alerts not configured")
	}

	// Initialize alert manager
	alertManager := alerts.NewManager(alertService)
	log.Println("initialized alert manager")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize worker
	worker := NewWorker()

	// Get enabled chains from environment
	enabledChains := os.Getenv("ENABLED_CHAINS")
	if enabledChains == "" {
		enabledChains = "base" // Default to Base
	}

	chainConfigs, err := workers.GetChainsByEnv(enabledChains)
	if err != nil {
		log.Fatalf("failed to parse enabled chains: %v", err)
	}

	log.Printf("monitoring %d chains: %s", len(chainConfigs), enabledChains)

	// Initialize oracle monitors for each chain
	for _, chainCfg := range chainConfigs {
		if err := setupOracleMonitor(ctx, chainCfg, alchemyKey, alertManager, &cfg.Oracle, worker); err != nil {
			log.Printf("failed to setup %s oracle monitor: %v", chainCfg.Name, err)
			continue
		}
		log.Printf("registered oracle monitor for %s (%d tokens)", chainCfg.Name, len(chainCfg.Tokens))
	}

	// Initialize database-dependent monitors if configured
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		if err := setupDatabaseMonitors(databaseURL, alertManager, cfg, worker); err != nil {
			log.Printf("warning: database monitors not available: %v", err)
		}
	} else {
		log.Println("DATABASE_URL not configured, database monitors disabled")
	}

	// Start all workers
	log.Printf("starting %d monitoring jobs", len(worker.jobs))
	worker.Start(ctx)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("received %s signal, shutting down...", sig)
	cancel()
	worker.Wait()

	// Log final alert state
	activeIncidents := alertManager.GetActiveIncidents()
	if len(activeIncidents) > 0 {
		log.Printf("shutting down with %d active incidents", len(activeIncidents))
	}

	log.Println("monitors stopped gracefully")
}

// setupOracleMonitor initializes an oracle monitor for a specific chain
func setupOracleMonitor(
	ctx context.Context,
	chainCfg workers.ChainConfig,
	alchemyKey string,
	alertManager *alerts.Manager,
	oracleCfg *config.OracleConfig,
	worker *Worker,
) error {
	// Get RPC URL for this chain
	rpcURL := getRPCURL(chainCfg.ID, alchemyKey)
	if rpcURL == "" {
		return fmt.Errorf("no RPC URL configured for %s", chainCfg.Name)
	}

	// Connect to RPC
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to %s RPC: %w", chainCfg.Name, err)
	}

	// Create oracle monitor
	monitor, err := workers.NewOracleMonitor(chainCfg, client, alchemyKey, alertManager, oracleCfg)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create oracle monitor: %w", err)
	}

	worker.Register(monitor)
	return nil
}

// setupDatabaseMonitors initializes database-dependent monitoring jobs
func setupDatabaseMonitors(
	databaseURL string,
	alertManager *alerts.Manager,
	cfg *config.Config,
	worker *Worker,
) error {
	// Test database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}
	db.Close()

	// Individual position monitoring
	healthJob, err := workers.NewHealthJobV2(databaseURL, alertManager)
	if err != nil {
		log.Printf("health factor monitoring disabled: %v", err)
	} else {
		worker.Register(healthJob)
		log.Println("registered health factor monitor")
	}

	// Aggregate health monitoring
	healthAggJob, err := workers.NewHealthAggregateJob(databaseURL, alertManager)
	if err != nil {
		log.Printf("aggregate health monitoring disabled: %v", err)
	} else {
		worker.Register(healthAggJob)
		log.Println("registered aggregate health monitor")
	}

	// Concentration risk monitoring
	concentrationJob, err := workers.NewConcentrationJob(databaseURL, alertManager)
	if err != nil {
		log.Printf("concentration monitoring disabled: %v", err)
	} else {
		worker.Register(concentrationJob)
		log.Println("registered concentration monitor")
	}

	return nil
}

// getRPCURL returns the RPC URL for a specific chain
func getRPCURL(chainID workers.ChainID, alchemyKey string) string {
	// Check for chain-specific environment variable first
	envKey := fmt.Sprintf("%s_RPC_URL", chainID)
	if url := os.Getenv(envKey); url != "" {
		return url
	}

	// Fall back to Alchemy defaults
	switch chainID {
	case workers.ChainBase:
		return fmt.Sprintf("https://base-mainnet.g.alchemy.com/v2/%s", alchemyKey)
	case workers.ChainOptimism:
		return fmt.Sprintf("https://opt-mainnet.g.alchemy.com/v2/%s", alchemyKey)
	case workers.ChainMoonbeam:
		return os.Getenv("MOONBEAM_RPC_URL")
	case workers.ChainMoonriver:
		return os.Getenv("MOONRIVER_RPC_URL")
	default:
		return ""
	}
}
