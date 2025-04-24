package service

import (
	"io"
	"os"

	pb "github.com/sunnypatel2048/primecruncher-v2/internal/proto"
)

// FileServer streams file segments.
type FileServer struct {
	pb.UnimplementedFileServerServiceServer
}

// NewFileServer creates a new FileServer instance.
func NewFileServer() *FileServer {
	return &FileServer{}
}

// FetchSegment streams a file segment in chunks.
func (s *FileServer) FetchSegment(req *pb.FetchSegmentRequest, stream pb.FileServerService_FetchSegmentServer) error {
	file, err := os.Open(req.Pathname)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(req.Start, io.SeekStart)
	if err != nil {
		return err
	}

	chunkSize := req.ChunkSize
	remaining := req.Length
	buf := make([]byte, chunkSize)

	for remaining > 0 {
		readSize := chunkSize
		if remaining < chunkSize {
			readSize = remaining
			buf = buf[:readSize]
		}
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if err := stream.Send(&pb.FetchSegmentResponse{Chunk: buf[:n]}); err != nil {
			return err
		}

		remaining -= int64(n)
	}
	return nil
}
