package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/sunnypatel2048/primecruncher-v2/internal/config"
	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
	"github.com/sunnypatel2048/primecruncher-v2/internal/service"
	"google.golang.org/grpc"
)

func main() {
	dataPath := flag.String("data", "", "Path to the data file")
	n := flag.Int64("N", 64*1024, "Segment size in bytes")
	c := flag.Int64("C", 1024, "Chunk size in bytes")
	configPath := flag.String("config", "./primes_config.txt", "Path to config file")
	flag.Parse()

	if *dataPath == "" || *configPath == "" {
		fmt.Println("Usage: dispatcher --data <datafile> --N <segment_size> --C <chunk_size> --config <configfile>")
		os.Exit(1)
	}

	if *n%8 != 0 || *c%8 != 0 {
		fmt.Println("N and C must be divisible by 8")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start dispatcher
	dispatcher, err := service.NewDispatcher(*dataPath, *n)
	if err != nil {
		slog.Error("Failed to create dispatcher", "error", err)
		os.Exit(1)
	}
	dispatcherLis, err := net.Listen("tcp", cfg.Dispatcher)
	if err != nil {
		slog.Error("Failed to listen for dispatcher", "error", err)
		os.Exit(1)
	}
	dispatcherServer := grpc.NewServer()
	pb.RegisterDispatcherServiceServer(dispatcherServer, dispatcher)

	// Start consolidator
	consolidator := service.NewConsolidator()
	consolidatorLis, err := net.Listen("tcp", cfg.Consolidator)
	if err != nil {
		slog.Error("Failed to listen for consolidator", "error", err)
		os.Exit(1)
	}
	consolidatorServer := grpc.NewServer()
	pb.RegisterConsolidatorServiceServer(consolidatorServer, consolidator)

	// Start servers
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		slog.Info("Starting dispatcher server", "address", cfg.Dispatcher)
		if err := dispatcherServer.Serve(dispatcherLis); err != nil {
			slog.Error("Dispatcher server failed", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		slog.Info("Starting consolidator server", "address", cfg.Consolidator)
		if err := consolidatorServer.Serve(consolidatorLis); err != nil {
			slog.Error("Consolidator server failed", "error", err)
		}
	}()

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		slog.Info("Received shutdown signal")
		dispatcherServer.GracefulStop()
		consolidatorServer.GracefulStop()
		cancel()
	}()

	// Wait for completion
	slog.Info("Waiting for dispatcher to finish")
	dispatcher.Wait()
	slog.Info("Waiting for consolidator to finish")
	consolidator.Wait()
	slog.Info("All tasks completed, printing total")
	fmt.Printf("Total primes: %d\n", consolidator.GetTotal())

	// Shutdown
	dispatcherServer.GracefulStop()
	consolidatorServer.GracefulStop()
	wg.Wait()
}
