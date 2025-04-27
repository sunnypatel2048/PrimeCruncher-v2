package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"math/rand"
	"time"

	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
	"github.com/sunnypatel2048/primecruncher-v2/internal/utils"
)

// Worker processes jobs from the dispatcher.
type Worker struct {
	dispatcherClient   pb.DispatcherServiceClient
	consolidatorClient pb.ConsolidatorServiceClient
	fileServerClient   pb.FileServerServiceClient
	chunkSize          int64
}

// NewWorker creates a new worker.
func NewWorker(dispatcherClient pb.DispatcherServiceClient, consolidatorClient pb.ConsolidatorServiceClient, fileServerClient pb.FileServerServiceClient, chunkSize int64) *Worker {
	return &Worker{
		dispatcherClient:   dispatcherClient,
		consolidatorClient: consolidatorClient,
		fileServerClient:   fileServerClient,
		chunkSize:          chunkSize,
	}
}

// Run starts the worker process.
func (w *Worker) Run(ctx context.Context) error {
	time.Sleep(time.Duration(400+rand.Intn(200)) * time.Millisecond)

	for {
		// Get job from dispatcher
		resp, err := w.dispatcherClient.GetJob(ctx, &pb.GetJobRequest{})
		if err != nil {
			return err
		}
		if resp.Done {
			return nil
		}

		// Fetch segment from fileserver
		stream, err := w.fileServerClient.FetchSegment(ctx, &pb.FetchSegmentRequest{
			Pathname:  resp.Pathname,
			Start:     resp.Start,
			Length:    resp.Length,
			ChunkSize: w.chunkSize,
		})
		if err != nil {
			slog.Error("Failed to fetch segment", "error", err)
			continue
		}

		// Process chunks and count primes
		primeCount := int64(0)
		for {
			chunkResp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.Error("Error receiving chunk", "error", err)
				break
			}
			reader := bytes.NewReader(chunkResp.Chunk)
			for {
				var num uint64
				if err := binary.Read(reader, binary.LittleEndian, &num); err != nil {
					if err == io.EOF {
						break
					}
					slog.Error("Error decoding number", "error", err)
					break
				}
				if utils.IsPrime(num) {
					primeCount++
				}
			}
		}

		// Submit result to consolidator
		_, err = w.consolidatorClient.SubmitResult(ctx, &pb.SubmitResultRequest{
			Pathname:   resp.Pathname,
			Start:      resp.Start,
			Length:     resp.Length,
			PrimeCount: primeCount,
		})
		if err != nil {
			slog.Error("Failed to submit result", "error", err)
			continue
		}
		slog.Info("Processed job", "pathname", resp.Pathname, "start", resp.Start, "length", resp.Length, "primes", primeCount)
	}
}
