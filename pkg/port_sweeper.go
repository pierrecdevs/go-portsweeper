package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type PortStatus string

const (
	StatusOpen     PortStatus = "open"
	StatusClosed   PortStatus = "closed"
	StatusFiltered PortStatus = "filtered/unknown"
)

type PortResult struct {
	Host     string        `json: "host`
	Port     int           `json: "port"`
	Status   PortStatus    `json: "status"`
	Duration time.Duration `json: "duration"`
	Error    string        `json: "error:omitempty"`
}

type Config struct {
	Host    string
	Ports   []int
	Workers int
	Timeout time.Duration
}

type PortSweeper struct {
	config Config
}

func NewPortSweeper(cfg Config) (*PortSweeper, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("'host' cannot be empty.")
	}

	if len(cfg.Ports) == 0 {
		return nil, errors.New("at least one port is required.")
	}

	if cfg.Workers < 1 {
		return nil, errors.New("'workers' must be greater than zero.")
	}

	if cfg.Timeout <= 0 {
		return nil, errors.New("'timeout' must be greater than zero")
	}

	ports := make([]int, 0, len(cfg.Ports))
	seen := make(map[int]struct{}, len(cfg.Ports))

	for _, port := range cfg.Ports {
		if port < 1 || port >= 65535 {
			return nil, fmt.Errorf("invalid port %d", port)
		}

		if _, exists := seen[port]; exists {
			continue
		}

		seen[port] = struct{}{}
		ports = append(ports, port)
	}

	sort.Ints(ports)
	cfg.Ports = ports

	if cfg.Workers > len(ports) {
		cfg.Workers = len(ports)
	}

	return &PortSweeper{config: cfg}, nil
}

func (ps *PortSweeper) testPort(ctx context.Context, port int) PortResult {
	result := PortResult{
		Host:   ps.config.Host,
		Port:   port,
		Status: StatusFiltered,
	}

	address := net.JoinHostPort(
		ps.config.Host,
		strconv.Itoa(port),
	)

	start := time.Now()

	dialer := net.Dialer{
		Timeout: ps.config.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	result.Duration = time.Since(start)

	if err == nil {
		_ = conn.Close()
		result.Status = StatusOpen
		return result
	}

	var opError *net.OpError
	if errors.As(err, &opError) && opError.Err != nil {
		// connection refused
		// host refused connection?
		if errors.Is(opError.Err, syscall.ECONNREFUSED) {
			result.Status = StatusClosed
			return result
		}
	}

	result.Status = StatusFiltered
	return result
}

func (ps *PortSweeper) Scan(
	ctx context.Context,
) <-chan PortResult {
	jobs := make(chan int, ps.config.Workers)
	results := make(chan PortResult, ps.config.Workers)

	var workers sync.WaitGroup
	workers.Add(ps.config.Workers)

	for i := 0; i < ps.config.Workers; i++ {
		go func() {
			defer workers.Done()

			for port := range jobs {
				result := ps.testPort(ctx, port)
				results <- result
			}
		}()
	}

	// close jobs
	go func() {
		defer close(jobs)

		for _, port := range ps.config.Ports {
			select {
			case jobs <- port:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	return results
}
