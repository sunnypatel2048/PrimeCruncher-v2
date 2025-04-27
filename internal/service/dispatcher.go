package service

import (
	"context"
	"os"
	"sync"

	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
)

// Dispatcher manages job distribution.
type Dispatcher struct {
	pb.UnimplementedDispatcherServiceServer
	pathname string
	fileSize int64
	n        int64
	jobs     chan *pb.GetJobResponse
	wg       sync.WaitGroup
}

// NewDispatcher creates a new Dispatcher instance.
func NewDispatcher(pathname string, n int64) (*Dispatcher, error) {
	file, err := os.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := info.Size()
	numJobs := (fileSize + n - 1) / n
	d := &Dispatcher{
		pathname: pathname,
		fileSize: fileSize,
		n:        n,
		jobs:     make(chan *pb.GetJobResponse, numJobs),
	}
	d.wg.Add(1)
	go d.generateJobs()
	return d, nil
}

// generateJobs creates and queues jobs.
func (d *Dispatcher) generateJobs() {
	defer d.wg.Done()
	for start := int64(0); start < d.fileSize; start += d.n {
		length := d.n
		if start+length > d.fileSize {
			length = d.fileSize - start
		}
		d.jobs <- &pb.GetJobResponse{
			Pathname: d.pathname,
			Start:    start,
			Length:   length,
			Done:     false,
		}
	}
	close(d.jobs)
}

// GetJob serves a job to a worker.
func (d *Dispatcher) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.GetJobResponse, error) {
	select {
	case job, ok := <-d.jobs:
		if !ok {
			return &pb.GetJobResponse{Done: true}, nil
		}
		return job, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Wait waits for the dispatcher to finish.
func (d *Dispatcher) Wait() {
	d.wg.Wait()
}
