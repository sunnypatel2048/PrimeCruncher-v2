package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/sunnypatel2048/primecruncher-v2/internal/config"
	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
	"github.com/sunnypatel2048/primecruncher-v2/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	chunkSize := flag.Int64("C", 1024, "Chunk size in bytes")
	configPath := flag.String("config", "./primes_config.txt", "Path to config file")
	m := flag.Int64("M", 1, "Number of worker threads")
	flag.Parse()

	if *configPath == "" {
		slog.Error("Config file path required")
		os.Exit(1)
	}
	if *chunkSize%8 != 0 {
		slog.Error("Chunk size must be divisible by 8")
		os.Exit(1)
	}
	if *m <= 0 {
		slog.Error("Number of worker threads (M) must be positive")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to dispatcher
	dispatcherConn, err := grpc.Dial(cfg.Dispatcher, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect to dispatcher", "error", err)
		os.Exit(1)
	}
	defer dispatcherConn.Close()
	dispatcherClient := pb.NewDispatcherServiceClient(dispatcherConn)

	// Connect to consolidator
	consolidatorConn, err := grpc.Dial(cfg.Consolidator, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect to consolidator", "error", err)
		os.Exit(1)
	}
	defer consolidatorConn.Close()
	consolidatorClient := pb.NewConsolidatorServiceClient(consolidatorConn)

	// Connect to fileserver
	fileServerConn, err := grpc.Dial(cfg.FileServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("Failed to connect to fileserver", "error", err)
		os.Exit(1)
	}
	defer fileServerConn.Close()
	fileServerClient := pb.NewFileServerServiceClient(fileServerConn)

	// Register worker
	_, err = consolidatorClient.RegisterWorker(ctx, &pb.RegisterWorkerRequest{})
	if err != nil {
		slog.Error("Failed to register worker", "error", err)
		os.Exit(1)
	}
	slog.Info("Worker registered successfully")

	// Create worker instance
	worker := service.NewWorker(dispatcherClient, consolidatorClient, fileServerClient, *chunkSize)

	// Spawn M worker goroutines
	var wg sync.WaitGroup
	slog.Info("Starting worker process", "num_threads", *m)
	for i := int64(0); i < *m; i++ {
		wg.Add(1)
		go func(workerID int64) {
			defer wg.Done()
			// Register worker
			_, err := consolidatorClient.RegisterWorker(ctx, &pb.RegisterWorkerRequest{})
			if err != nil {
				slog.Error("Failed to register worker", "worker_id", workerID, "error", err)
				return
			}
			slog.Info("Worker registered successfully", "worker_id", workerID)

			// Run worker
			if err := worker.Run(ctx); err != nil {
				slog.Error("Worker failed", "worker_id", workerID, "error", err)
			}

			// Deregister worker
			if _, err := consolidatorClient.DeregisterWorker(ctx, &pb.DeregisterWorkerRequest{}); err != nil {
				slog.Error("Failed to deregister worker", "worker_id", workerID, "error", err)
			} else {
				slog.Info("Worker deregistered successfully", "worker_id", workerID)
			}
		}(i)
	}

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		slog.Info("Received shutdown signal")
		cancel()
	}()

	// Wait for all workers to complete
	wg.Wait()
	slog.Info("All worker threads completed")
}
