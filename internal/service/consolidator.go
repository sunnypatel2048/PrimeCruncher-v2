package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
)

// Consolidator aggregates prime counts.
type Consolidator struct {
	pb.UnimplementedConsolidatorServiceServer
	total      uint64
	workers    int32
	results    chan *pb.SubmitResultRequest
	closed     chan struct{}
	wg         sync.WaitGroup
	workerDone chan struct{}
}

// NewConsolidator creates a new consolidator.
func NewConsolidator() *Consolidator {
	c := &Consolidator{
		results:    make(chan *pb.SubmitResultRequest, 100),
		closed:     make(chan struct{}),
		workerDone: make(chan struct{}),
	}
	c.wg.Add(1)
	go c.aggregate()
	return c
}

// RegisterWorker increments the worker count.
func (c *Consolidator) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*pb.RegisterWorkerResponse, error) {
	atomic.AddInt32(&c.workers, 1)
	slog.Info("Worker registered", "total_workers", atomic.LoadInt32(&c.workers))
	return &pb.RegisterWorkerResponse{}, nil
}

// DeregisterWorker decrements the worker count.
func (c *Consolidator) DeregisterWorker(ctx context.Context, req *pb.DeregisterWorkerRequest) (*pb.DeregisterWorkerResponse, error) {
	currentWorkers := atomic.AddInt32(&c.workers, -1)
	slog.Info("Worker deregistered", "total_workers", currentWorkers)
	if currentWorkers == 0 {
		slog.Info("All workers deregistered, closing workerDone")
		close(c.workerDone)
	}
	return &pb.DeregisterWorkerResponse{}, nil
}

// SubmitResult receives a worker's result.
func (c *Consolidator) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	// Ignore empty results
	if req.Pathname == "" && req.Start == 0 && req.Length == 0 && req.PrimeCount == 0 {
		return &pb.SubmitResultResponse{}, nil
	}
	select {
	case c.results <- req:
		return &pb.SubmitResultResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, context.Canceled
	}
}

// aggregate processes results.
func (c *Consolidator) aggregate() {
	defer c.wg.Done()
	timeout := time.After(30 * time.Second) // Timeout if no activity
	for {
		select {
		case result := <-c.results:
			if result != nil {
				atomic.AddUint64(&c.total, uint64(result.PrimeCount))
				slog.Info("Received result", "pathname", result.Pathname, "start", result.Start, "length", result.Length, "primes", result.PrimeCount)
			}
		case <-c.workerDone:
			slog.Info("All workers done, shutting down consolidator")
			close(c.closed)
			return
		case <-timeout:
			slog.Warn("Consolidator timed out waiting for results")
			close(c.closed)
			return
		}
	}
}

// GetTotal returns the total prime count.
func (c *Consolidator) GetTotal() uint64 {
	return atomic.LoadUint64(&c.total)
}

// Wait waits for the consolidator to finish.
func (c *Consolidator) Wait() {
	c.wg.Wait()
}
