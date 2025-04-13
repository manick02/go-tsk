package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mshan/go-tsk/internal/config"
	"github.com/mshan/go-tsk/internal/scheduler"
)

func main() {
	// Create configuration
	cfg := config.DefaultConfig()

	// Create email poller
	poller := scheduler.NewEmailPoller(cfg)

	// Create context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Start polling
	log.Println("Starting email poller...")
	if err := poller.Start(ctx); err != nil {
		log.Printf("Poller stopped with error: %v", err)
		os.Exit(1)
	}
}
