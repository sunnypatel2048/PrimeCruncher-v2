package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

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

// SubmitResult receives a worker's result.
func (c *Consolidator) SubmitResult(ctx context.Context, req *pb.SubmitResultRequest) (*pb.SubmitResultResponse, error) {
	select {
	case c.results <- req:
		return &pb.SubmitResultResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, context.Canceled
	}
}

// RegisterWorker increments the worker count.
func (c *Consolidator) RegisterWorker() {
	atomic.AddInt32(&c.workers, 1)
}

// DeregisterWorker decrements the worker count.
func (c *Consolidator) DeregisterWorker() {
	if atomic.AddInt32(&c.workers, -1) == 0 {
		close(c.workerDone)
	}
}

// aggregate processes results.
func (c *Consolidator) aggregate() {
	defer c.wg.Done()
	for {
		select {
		case result := <-c.results:
			atomic.AddUint64(&c.total, uint64(result.PrimeCount))
			slog.Info("Received result", "pathname", result.Pathname, "start", result.Start, "length", result.Length, "primes", result.PrimeCount)
		case <-c.workerDone:
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
