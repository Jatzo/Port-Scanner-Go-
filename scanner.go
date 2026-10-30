package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Status is what a check found. Failed is kept apart from Closed so that a check that could
// not run is never reported as a port that is shut.
type Status int

const (
	Open Status = iota
	Closed
	Failed
)

type Result struct {
	Port   int
	Status Status
	Err    error // set only when Status is Failed
}

// Windows reports a refused connection as 10061, which Go does not map to syscall.ECONNREFUSED.
const wsaConnRefused = syscall.Errno(10061)

func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, wsaConnRefused)
}

// ResolveHost turns a name into an IP address once, so the scan does not look the name up
// again for every port.
func ResolveHost(ctx context.Context, host string) (string, error) {
	if net.ParseIP(host) != nil {
		return host, nil
	}
	addresses, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil || len(addresses) == 0 {
		return "", fmt.Errorf("could not find the host %q. Check the spelling and your connection, or use an IP address instead", host)
	}
	return addresses[0], nil
}

// Scan checks every port on the host, with at most `workers` checks running at once.
// It returns the results sorted by port. If ctx is cancelled it returns what finished so
// far and ctx's error, so the caller can say the results are partial.
func Scan(ctx context.Context, ip string, ports []int, workers int, timeout time.Duration) ([]Result, error) {
	dialer := &net.Dialer{Timeout: timeout}

	// An unbuffered channel hands one port to one worker at a time, which is what keeps
	// the number of checks running at once equal to `workers`.
	jobs := make(chan int)
	results := make(chan Result)

	// The feeder sends ports to the workers, then closes jobs so their loops end.
	// Every send also watches ctx.Done(), otherwise it would block forever once the
	// workers had stopped listening.
	go func() {
		defer close(jobs)
		for _, port := range ports {
			select {
			case jobs <- port:
			case <-ctx.Done():
				return
			}
		}
	}()

	// The WaitGroup counts live workers. When it reaches zero nothing can send on results
	// any more, so it is safe to close it and let the collecting loop below finish.
	var workersRunning sync.WaitGroup
	for i := 0; i < workers; i++ {
		workersRunning.Add(1)
		go func() {
			defer workersRunning.Done()
			for port := range jobs {
				result, ok := probe(ctx, dialer, ip, port)
				if !ok {
					return
				}
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		workersRunning.Wait()
		close(results)
	}()

	var collected []Result
	for result := range results {
		collected = append(collected, result)
	}
	sort.Slice(collected, func(a, b int) bool { return collected[a].Port < collected[b].Port })

	if len(collected) < len(ports) {
		return collected, ctx.Err()
	}
	return collected, nil
}

// probe makes one connection attempt. Only a refusal or a timeout counts as Closed: those are
// the two ways a shut port answers. Any other error (for example "too many open files") means
// the check itself went wrong, so it is returned as Failed. The bool is false when the scan
// was cancelled, because a cancelled attempt says nothing about the port.
func probe(ctx context.Context, dialer *net.Dialer, ip string, port int) (Result, bool) {
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err == nil {
		connection.Close()
		return Result{Port: port, Status: Open}, true
	}
	if ctx.Err() != nil {
		return Result{}, false
	}
	var netErr net.Error
	if isRefused(err) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return Result{Port: port, Status: Closed}, true
	}
	return Result{Port: port, Status: Failed, Err: err}, true
}
