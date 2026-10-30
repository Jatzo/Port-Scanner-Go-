package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	minPort = 1
	maxPort = 65535
)

// ParsePorts turns a list such as "80,443,8000-8100" into a sorted list of ports with no
// duplicates. It rejects anything it cannot read, so a typo never quietly scans fewer ports
// than the person meant to scan.
func ParsePorts(spec string) ([]int, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("the port list is empty. Give ports such as 80,443 or a range such as 1-1024")
	}

	// A map is Go's set: adding the same port twice leaves one entry.
	seen := make(map[int]bool)
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("the port list has an empty entry. Check for a stray comma in %q", spec)
		}
		first, last, err := parseItem(item)
		if err != nil {
			return nil, err
		}
		for port := first; port <= last; port++ {
			seen[port] = true
		}
	}

	// Map order is random in Go, so sort to give the same scan every time.
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}

// parseItem reads one entry: either a single port ("80") or a range ("8000-8100").
func parseItem(item string) (int, int, error) {
	low, high, isRange := strings.Cut(item, "-")
	first, err := parsePort(low)
	if err != nil {
		return 0, 0, err
	}
	if !isRange {
		return first, first, nil
	}
	last, err := parsePort(high)
	if err != nil {
		return 0, 0, err
	}
	if last < first {
		return 0, 0, fmt.Errorf("the range %q runs backwards. Put the lower port first", item)
	}
	return first, last, nil
}

func parsePort(text string) (int, error) {
	text = strings.TrimSpace(text)
	port, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("%q is not a port number. Use whole numbers from %d to %d", text, minPort, maxPort)
	}
	if port < minPort || port > maxPort {
		return 0, fmt.Errorf("port %d is out of range. Use %d to %d", port, minPort, maxPort)
	}
	return port, nil
}
