package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/sunnypatel2048/primecruncher-v2/internal/config"
	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
	"github.com/sunnypatel2048/primecruncher-v2/internal/service"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "./primes_config.txt", "Path to the configuration file")
	flag.Parse()

	if *configPath == "" {
		slog.Error("Configuration file path is required")
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", cfg.FileServer)
	if err != nil {
		slog.Error("Failed to listen", "error", err)
		os.Exit(1)
	}

	server := grpc.NewServer()
	pb.RegisterFileServerServiceServer(server, service.NewFileServer())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		slog.Info("Starting fileserver", "address", cfg.FileServer)
		if err := server.Serve(lis); err != nil {
			slog.Error("Fileserver failed", "error", err)
		}
	}()

	<-sigChan
	slog.Info("Received shutdown signal")
	server.GracefulStop()
}
