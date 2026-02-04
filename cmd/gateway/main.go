package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openclaw/go-openclaw/internal/config"
	"github.com/openclaw/go-openclaw/pkg/gateway"
)

var (
	gw *gateway.Gateway
)

// Execute starts the OpenClaw gateway
func Execute() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create gateway
	gw = gateway.New(cfg.GetAddr())

	// Start gateway
	log.Printf("🚀 Starting OpenClaw Gateway v0.0.1")
	log.Printf("🌐 Listening on %s", cfg.GetAddr())

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
func ExecuteWithGateway(g *gateway.Gateway) {
	gw = g

	log.Printf("🚀 Starting OpenClaw Gateway")

	if err := gw.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start gateway: %v", err)
	}

	log.Printf("✅ Gateway started successfully")
}

// GetGatewayState returns gateway state
func GetGatewayState() *gateway.GatewayState {
	if gw == nil {
		return &gateway.GatewayState{
			Running: false,
			Version: "0.0.1",
		}
	}

	return gw.GetState()
}

// main is the entry point for the gateway application
func main() {
	Execute()
}
