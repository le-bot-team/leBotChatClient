package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create application instance
	app := NewApp()

	// Start application
	if err := app.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for exit signal or application to finish
	doneCh := make(chan struct{})
	go func() {
		app.Wait()
		close(doneCh)
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received exit signal: %v", sig)
	case <-doneCh:
		log.Println("Application terminated voluntarily")
	}

	// Graceful shutdown
	if err := app.Stop(); err != nil {
		log.Printf("Failed to shut down application: %v", err)
		os.Exit(1)
	}
}
