package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/api"
	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/executor"
	"github.com/pibot/pibot/internal/fileops"
	"github.com/pibot/pibot/internal/scheduler"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded from %s", *configPath)

	// Initialize AI manager with all providers
	aiManager := ai.NewManager(cfg)
	aiManager.RegisterProvider(ai.NewOpenAIProvider(cfg))
	aiManager.RegisterProvider(ai.NewAnthropicProvider(cfg))
	aiManager.RegisterProvider(ai.NewGoogleProvider(cfg))
	aiManager.RegisterProvider(ai.NewOllamaProvider(cfg))
	aiManager.RegisterProvider(ai.NewQwenProvider(cfg))

	log.Printf("Registered AI providers: %v", aiManager.ListProviders())

	// Initialize command executor
	exec := executor.NewExecutor(cfg)

	// Initialize file operations
	fileOps := fileops.NewFileOps(cfg)
	if err := fileOps.EnsureBaseDirectory(); err != nil {
		log.Fatalf("Failed to create base directory: %v", err)
	}
	log.Printf("File operations base directory: %s", fileOps.GetBaseDirectory())

	// Initialize scheduler (chat session wired inside api.NewServer)
	sched := scheduler.NewScheduler(exec)

	// Create API server (also wires chat session into the scheduler)
	server := api.NewServer(cfg, aiManager, exec, fileOps, sched)
	server.StartWebSocketHub()

	// Start scheduler with a cancellable context
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go sched.Start(schedCtx)

	// Configure HTTP server
	serverCfg := cfg.GetServer()
	addr := fmt.Sprintf("%s:%d", serverCfg.Host, serverCfg.Port)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting PiBot server on http://%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop scheduler first
	schedCancel()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
