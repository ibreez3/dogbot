package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openclaw/go-openclaw/internal/config"
	"github.com/openclaw/go-openclaw/pkg/gateway"
	"github.com/spf13/cobra"
)

var (
	gateway *gateway.Gateway
)

// Execute starts the OpenClaw gateway
func Execute() {
	// Load configuration
	config, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create gateway
	gw := gateway.New(config.Gateway.Addr)

	// Start gateway
	log.Printf("🚀 Starting OpenClaw Gateway v%s", config.Gateway.Version)
	log.Printf("🌐 Listening on %s", config.Gateway.Addr)

	if err := gw.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start gateway: %v", err)
	}

	log.Printf("✅ Gateway started successfully")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	log.Println("🛑 Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := gw.Stop(shutdownCtx); err != nil {
		log.Printf("⚠️  Gateway shutdown error: %v", err)
	}

	log.Println("✅ Gateway stopped")

	os.Exit(0)
}

// GetGateway returns the current gateway instance
func GetGateway() *gateway.Gateway {
	return gw
}

// ExecuteWithGateway starts with a specific gateway instance
func ExecuteWithGateway(gw *gateway.Gateway) {
	gateway = gw

	log.Printf("🚀 Starting OpenClaw Gateway")

	if err := gw.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start gateway: %v", err)
	}

	log.Printf("✅ Gateway started successfully")
}

// GetGatewayStats returns gateway statistics
func GetGatewayStats() gateway.GatewayStats {
	if gateway == nil {
		return gateway.GatewayStats{
			Running:  false,
			Version:  "0.0.1",
		}
	}

	return gateway.GetStats()
}
