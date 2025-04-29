# Prime Cruncher - v2

## Introduction

Prime Cruncher - v2 is a distributed GoLang application that computes the number of prime 64-bit unsigned integers in a binary file. It employs a dispatcher-worker-consolidator architecture, with workers running as independent processes (potentially on different machines) communicating via gRPC over TCP. A dedicated fileserver streams file segments to workers, enabling efficient processing of large datasets. The project is designed for correctness, high concurrency, and minimal communication overhead.

## Features

* **Distributed Architecture:** Workers run as separate processes, communicating with a dispatcher, consolidator, and fileserver via gRPC.
* **Multi-threaded Workers:** Each worker process spawns ```M``` goroutines for parallel job processing.
* **Server-Side Streaming:** Fileserver streams file segments in chunks using gRPC.
* **Configuration:** Server addresses are specified in a simple ASCII configuration file (```primes_config.txt```).
* **Logging:** Structured logging with slog for debugging and monitoring.
* **Graceful Shutdown:** Handles interrupts (```Ctrl+C```) for clean termination.

## Prerequisites

* **Go:** Version 1.21 or later.
* **Protocol Buffers Compiler:** ```protoc``` for generating gRPC code.
* **gRPC Go Plugins:** ```protoc-gen-go``` and ```protoc-gen-go-grpc```.

Install dependencies:

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
```

## Configuration

Create a ```primes_config.txt``` file with server addresses in the format ```<server>``` ```<host>``` ```<port>```:

```text
dispatcher localhost:5001
consolidator localhost:5002
fileserver localhost:5003
```

You can modify the hostnames/ports for distributed setups across multiple machines.

## Generate Test Data

Generate a random binary data file for testing:

```bash
head -c <no-of-bytes> /dev/urandom > <file-name>
```

* **no-of-bytes:** The size of the file in bytes. Since the program processes 64-bit (8-byte) integers, this should ideally be a multiple of 8 for consistent results.
* **file-name:** The name of the output file (e.g., ```data.dat```).

## Building

1. **Generate gRPC Code:**

    ```bash
    protoc --go_out=. --go-grpc_out=. internal/proto/primecruncher-v2.proto
    ```

2. **Build the Programs:**

    ```bash
    go build -o dispatcher cmd/dispatcher/main.go
    go build -o fileserver cmd/fileserver/main.go
    go build -o worker cmd/worker/main.go
    ```

## Usage

Run the components in separate terminal sessions. The programs accept command-line flags for configuration.

1. **Start the Fileserver**

    ```bash
    ./fileserver --config primes_config.txt
    ```

    * ```--config```: Path to the configuration file.

2. **Start the Dispatcher**

    ```bash
    ./dispatcher --data data/1KB-one.dat --N 65536 --C 1024 --config primes_config.txt
    ```

    * ```--data```: Path to the input binary file.
    * ```--N```: Segment size in bytes (default: 64KB, must be divisible by 8).
    * ```--C```: Chunk size in bytes (default: 1KB, must be divisible by 8).
    * ```--config```: Path to the configuration file.

    The dispatcher runs both the dispatcher and consolidator as gRPC servers, listening on ports specified in ```primes_config.txt```.

3. **Start Worker(s)**

    ```bash
    ./worker --C 1024 --M 4 --config primes_config.txt
    ```

    * ```--C```: Chunk size in bytes (must match dispatcher’s ```C```, divisible by 8).
    * ```--M```: Number of worker threads (goroutines) per process (default: 1).
    * ```--config```: Path to the configuration file.

    You can start multiple worker processes to distribute the workload:

    ```bash
    ./worker --C 1024 --M 2 --config primes_config.txt
    ./worker --C 1024 --M 3 --config primes_config.txt
    ```

4. **Shutdown**

    Press ```Ctrl+C``` in each terminal to gracefully shut down the processes. The dispatcher will print the total number of primes found:

    ```text
    Total primes: <number>
    ```

## Design and Implementation

### Architecture

* **Dispatcher:** Partitions the input file into segments of ```N``` bytes and serves jobs to workers via unary gRPC calls (```GetJob```).
* **Workers:** Independent processes, each spawning ```M``` goroutines. Each goroutine:

  * Fetches jobs from the dispatcher.
  * Streams segment data from the fileserver in ```C```-byte chunks.
  * Counts prime 64-bit integers using encoding/binary for decoding.
  * Submits results to the consolidator.

* **Consolidator:** Aggregates prime counts from workers via unary gRPC calls (```SubmitResult```). Supports worker registration/deregistration (```RegisterWorker```, ```DeregisterWorker```).
* **Fileserver:** Streams file segments to workers using server-side streaming gRPC (```FetchSegment```).

### Key Features

* **Concurrency:** Workers run as separate processes, with ```M``` goroutines per process for intra-process parallelism. gRPC ensures low-latency communication.
* **Correctness:** No race conditions due to gRPC’s request-response model and atomic operations in the consolidator.
* **Efficiency:** Minimal data is communicated (job descriptors, result counts, file chunks). Buffered channels and streaming optimize performance.
* **Robustness:** Graceful shutdown handle errors and interrupts.

### Implementation Choices

* **gRPC:** Chosen for reliable, high-performance RPC communication across distributed processes.
* **Goroutines:** Used within workers for lightweight threading, mimicking the original project’s ```M``` worker threads.
* **Configuration:** Simple ASCII file (```primes_config.txt```) for flexible server address specification.
* **Logging:** Structured ```slog``` logs for debugging worker lifecycle, job processing, and errors.
