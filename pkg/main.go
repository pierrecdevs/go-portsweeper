package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func parsePorts(singlePort int, portRange string) ([]int, error) {
	if singlePort != 0 && portRange != "" {
		return nil, errors.New("use either, '-port' or '-ports', not both.")
	}

	if singlePort != 0 {
		if singlePort < 1 || singlePort > 65535 {
			return nil, errors.New("port must be between 1 and 65535")
		}

		return []int{singlePort}, nil
	}

	if portRange == "" {
		return nil, errors.New("specify '-port' or '-ports'")
	}

	parts := strings.Split(portRange, "-")
	if len(parts) > 2 {
		return nil, errors.New("invalid port range: use START-END")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid start port: %w", err)
	}

	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])

		if err != nil {
			return nil, fmt.Errorf("invalid end port: %w", err)
		}
	}

	if start < 1 || end > 65535 || start > end {
		return nil, errors.New("range must satisfy 1 <= START <= END <= 65535")
	}

	ports := make([]int, 0, end-start+1)
	for port := start; port <= end; port++ {
		ports = append(ports, port)
	}

	return ports, nil
}
func main() {
	host := flag.String("host", "127.0.0.1", "target hostname or IP")
	singlePort := flag.Int("port", 0, "scan a single TCP port")
	portRange := flag.String("ports", "", "range of ports, e.g 80-443")
	workers := flag.Int("workers", 200, "maximum concurrent workers (default: 200)")
	timeout := flag.Duration("timeout", 800*time.Millisecond, "per-connection timeout (default: 8sec)")
	showClosed := flag.Bool("show closed", false, "display closed and filtered ports")
	flag.Parse()

	if *singlePort == 0 && *portRange == "" {
		*portRange = "1-1024"
	}

	ports, err := parsePorts(*singlePort, *portRange)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	cfg := Config{
		Host:    *host,
		Ports:   *&ports,
		Workers: *workers,
		Timeout: *timeout,
	}

	scanner, err := NewPortSweeper(cfg)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	fmt.Printf(
		"PortSweeper | host=%s | ports=%d | workers=%d | timeout=%s\n\n",
		cfg.Host,
		len(cfg.Ports),
		cfg.Workers,
		cfg.Timeout,
	)

	var results []PortResult
	for result := range scanner.Scan(ctx) {
		results = append(results, result)

		if result.Status == StatusOpen || *showClosed {
			fmt.Printf(
				"%-16s %-6d %-18s %s\n",
				result.Host,
				result.Port,
				result.Status,
				result.Duration,
			)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Port < results[j].Port
	})

	openCount, closedCount, filteredCount := 0, 0, 0
	for _, result := range results {
		switch result.Status {
		case StatusOpen:
			openCount++
		case StatusClosed:
			closedCount++

		default:
			filteredCount++
		}
	}

	fmt.Println("\n-- Summary--")
	fmt.Printf("Completed: %d / %d\n", len(results), len(cfg.Ports))
	fmt.Printf("Open: %d\n", openCount)
	fmt.Printf("Closed: %d\n", closedCount)
	fmt.Printf("Filtered/Unknown: %d\n", filteredCount)

	if ctx.Err() != nil {
		fmt.Println("Scan interrupted.")
	}
}
