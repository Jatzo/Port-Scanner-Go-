package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"time"
)

func main() {
	// Ctrl+C cancels this context, and every part of the scan watches it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run does the whole job and returns the exit code: 0 for a clean scan, 1 when the scan was
// stopped or some ports could not be checked, 2 when the options were wrong. Results go to
// stdout and everything else to stderr, so the results can be piped on their own.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("port-scanner", flag.ContinueOnError)
	flags.SetOutput(stderr)
	host := flags.String("host", "127.0.0.1", "host to scan, a name or an IP address")
	portList := flags.String("ports", "1-1024", "ports to scan, such as 80,443 or 1-1024 or 80,8000-8100")
	workers := flags.Int("workers", 100, "how many ports to check at once")
	timeout := flags.Duration("timeout", 500*time.Millisecond, "how long to wait on each port")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// Without this, "port-scanner 80" would quietly scan the default ports instead.
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "Did not expect %q. Options start with a dash, such as -ports 80.\n", flags.Arg(0))
		return 2
	}
	if *workers < 1 {
		fmt.Fprintln(stderr, "The number of workers must be 1 or more. Try -workers 100.")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "The timeout must be longer than zero. Try -timeout 500ms.")
		return 2
	}
	ports, err := ParsePorts(*portList)
	if err != nil {
		fmt.Fprintf(stderr, "Could not read -ports: %v.\n", err)
		return 2
	}
	ip, err := ResolveHost(ctx, *host)
	if err != nil {
		fmt.Fprintf(stderr, "%v.\n", err)
		return 2
	}

	warnAboutPermission(stderr, ip)
	fmt.Fprintf(stderr, "Scanning %d ports on %s, %d at a time.\n", len(ports), ip, *workers)

	results, scanErr := Scan(ctx, ip, ports, *workers, *timeout)

	var open, closed, failed int
	var firstFailure error
	for _, result := range results {
		switch result.Status {
		case Open:
			open++
			fmt.Fprintf(stdout, "%d open\n", result.Port)
		case Closed:
			closed++
		case Failed:
			failed++
			if firstFailure == nil {
				firstFailure = result.Err
			}
		}
	}

	fmt.Fprintf(stderr, "%d open, %d closed, %d could not be checked.\n", open, closed, failed)
	code := 0
	if failed > 0 {
		fmt.Fprintf(stderr, "The first error was: %v\nIf it mentions too many open files, lower -workers and try again.\n", firstFailure)
		code = 1
	}
	if scanErr != nil {
		fmt.Fprintf(stderr, "The scan was stopped after %d of %d ports, so these results are partial.\n", len(results), len(ports))
		code = 1
	}
	return code
}

// warnAboutPermission reminds the person to scan only what they are allowed to scan, and
// says so more firmly when the address is outside their own machine and home network.
func warnAboutPermission(stderr io.Writer, ip string) {
	address := net.ParseIP(ip)
	if address != nil && (address.IsLoopback() || address.IsPrivate()) {
		fmt.Fprintln(stderr, "Only scan hosts you own or have permission to scan.")
		return
	}
	fmt.Fprintf(stderr, "%s is not on your own machine or a private network. Only carry on if you own it or have written permission to scan it. Press Ctrl+C to stop.\n", ip)
}
