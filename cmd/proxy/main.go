package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/client"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/fingerprint"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/http"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/models"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/session"
	"github.com/kilimcininkoroglu/commandcode-bridge/pkg/version"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logLevel := logging.ParseLevel(cfg.LogLevel)
	var logFile *os.File
	if cfg.LogFile != "" {
		logFile, err = os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
			os.Exit(1)
		}
		defer logFile.Close()
	}

	logger := logging.New(logLevel, os.Stdout, logFile, false)
	if err := version.RefreshCommandCodeVersion(); err != nil {
		logger.Debug("Failed to refresh CommandCode version on startup", map[string]any{
			"error": err.Error(),
		})
	}
	logger.Info("Starting CommandCode Bridge", map[string]any{
		"version":     version.Version,
		"cli_version": version.GetCommandCodeVersion(),
	})

	// Generate or load fingerprint
	var fp *config.Fingerprint
	if cfg.Fingerprint == nil || cfg.Fingerprint.Thumbmark == "" {
		logger.Info("Generating new fingerprint", nil)
		fpData, err := fingerprint.Generate()
		if err != nil {
			logger.Error("Failed to generate fingerprint", map[string]any{
				"error": err.Error(),
			})
			os.Exit(1)
		}

		fp = &config.Fingerprint{
			Thumbmark: fpData.Thumbmark,
			Components: config.FingerprintComponents{
				MachineIdHash:    fpData.Components.MachineIdHash,
				MacHashes:        fpData.Components.MacHashes,
				OsUserHash:       fpData.Components.OsUserHash,
				HostnameHash:     fpData.Components.HostnameHash,
				GitEmailHash:     fpData.Components.GitEmailHash,
				Platform:         fpData.Components.Platform,
				Arch:             fpData.Components.Arch,
				OsRelease:        fpData.Components.OsRelease,
				CpuModel:         fpData.Components.CpuModel,
				CpuCount:         fpData.Components.CpuCount,
				MemGiB:           fpData.Components.MemGiB,
				IsContainer:      fpData.Components.IsContainer,
				Timezone:         fpData.Components.Timezone,
				Runtime:          fpData.Components.Runtime,
				CollectorVersion: fpData.Components.CollectorVersion,
			},
		}

		// Save fingerprint to config
		cfg.Fingerprint = fp
		if err := cfg.Save(*configPath); err != nil {
			logger.Warn("Failed to save fingerprint to config", map[string]any{
				"error": err.Error(),
			})
		}
	} else {
		fp = cfg.Fingerprint
		logger.Info("Using existing fingerprint", nil)
	}

	// Initialize session store
	sessionStore := session.NewStore(12*time.Hour, 1*time.Hour)
	sessionStore.StartCleanup(5 * time.Minute)
	defer sessionStore.Stop()

	// Initialize HTTP client
	apiClient := client.New(cfg.APIBase, cfg.ProjectSlug, logger)

	// Initialize init manager for fingerprint/lifecycle events
	initManager := client.NewInitManager(cfg.APIBase, cfg.ProjectSlug, logger)

	// Initialize model manager
	modelManager := models.NewManager(cfg.APIBase, cfg.UseProviderModels, cfg.ModelRefreshInterval, logger)

	// Create handler dependencies
	deps := &http.HandlerDependencies{
		Config:       cfg,
		Logger:       logger,
		SessionStore: sessionStore,
		Client:       apiClient,
		ModelManager: modelManager,
		InitManager:  initManager,
		Version:      version.Version,
	}

	// Create and start HTTP server
	server := http.New(cfg, logger, deps)

	// Start model refresh goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go modelManager.StartRefresh(ctx, "")

	// Start CommandCode version auto-refresh
	go version.StartAutoRefresh(ctx, logger)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		logger.Error("Server error", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down", map[string]any{
			"signal": sig.String(),
		})
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", map[string]any{
			"error": err.Error(),
		})
	}

	logger.Info("Server stopped", nil)
}
