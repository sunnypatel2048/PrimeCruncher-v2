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
	// Track job counts per worker
	workerJobs map[int64]int
	workerMu   sync.Mutex
	workerID   int64
}

// NewConsolidator creates a new consolidator.
func NewConsolidator() *Consolidator {
	c := &Consolidator{
		results:    make(chan *pb.SubmitResultRequest, 100),
		closed:     make(chan struct{}),
		workerDone: make(chan struct{}),
		workerJobs: make(map[int64]int),
	}
	c.wg.Add(1)
	go c.aggregate()
	return c
}

// RegisterWorker increments the worker count and assigns an ID.
func (c *Consolidator) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*pb.RegisterWorkerResponse, error) {
	workerID := atomic.AddInt64(&c.workerID, 1)
	atomic.AddInt32(&c.workers, 1)
	slog.Info("Worker registered", "worker_id", workerID, "total_workers", atomic.LoadInt32(&c.workers))
	return &pb.RegisterWorkerResponse{WorkerId: workerID}, nil
}

// DeregisterWorker decrements the worker count.
func (c *Consolidator) DeregisterWorker(ctx context.Context, req *pb.DeregisterWorkerRequest) (*pb.DeregisterWorkerResponse, error) {
	currentWorkers := atomic.AddInt32(&c.workers, -1)
	slog.Info("Worker deregistered", "worker_id", req.WorkerId, "total_workers", currentWorkers)
	if currentWorkers <= 0 {
		slog.Info("All workers deregistered, closing workerDone")
		select {
		case <-c.closed:
			// Already closed, no action needed
		default:
			close(c.workerDone)
		}
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
		// Increment job count for the worker
		c.workerMu.Lock()
		c.workerJobs[req.WorkerId]++
		c.workerMu.Unlock()
		return &pb.SubmitResultResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, context.Canceled
	}
}

// aggregate processes results until all workers are done.
func (c *Consolidator) aggregate() {
	defer c.wg.Done()
	for {
		select {
		case result := <-c.results:
			if result != nil {
				atomic.AddUint64(&c.total, uint64(result.PrimeCount))
				slog.Info("Received result", "pathname", result.Pathname, "start", result.Start, "length", result.Length, "primes", result.PrimeCount, "worker_id", result.WorkerId)
			}
		case <-c.workerDone:
			slog.Info("All workers done, shutting down consolidator")
			close(c.closed)
			return
		}
	}
}

// GetTotal returns the total prime count.
func (c *Consolidator) GetTotal() uint64 {
	return atomic.LoadUint64(&c.total)
}

// GetJobCounts returns the number of jobs completed by each worker.
func (c *Consolidator) GetJobCounts() []int {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	counts := make([]int, 0, len(c.workerJobs))
	for _, count := range c.workerJobs {
		counts = append(counts, count)
	}
	return counts
}

// Wait waits for the consolidator to finish.
func (c *Consolidator) Wait() {
	c.wg.Wait()
}
