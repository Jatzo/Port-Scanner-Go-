package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

// listenOnFreePort opens a real listener on the local machine and returns its port. The
// listener closes when the test ends.
func listenOnFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open a local listener: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	return listener.Addr().(*net.TCPAddr).Port
}

// freePortWithNothingListening finds a port that was free a moment ago and leaves it unused.
func freePortWithNothingListening(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open a local listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func scanLocal(t *testing.T, ports []int, workers int) []Result {
	t.Helper()
	results, err := Scan(context.Background(), "127.0.0.1", ports, workers, time.Second)
	if err != nil {
		t.Fatalf("Scan returned an error: %v", err)
	}
	return results
}

func TestAnOpenPortIsReportedAsOpen(t *testing.T) {
	port := listenOnFreePort(t)

	results := scanLocal(t, []int{port}, 1)

	if len(results) != 1 || results[0].Status != Open {
		t.Errorf("got %+v, want port %d open", results, port)
	}
}

func TestAClosedPortIsReportedAsClosed(t *testing.T) {
	port := freePortWithNothingListening(t)

	results := scanLocal(t, []int{port}, 1)

	if len(results) != 1 || results[0].Status != Closed {
		t.Errorf("got %+v, want port %d closed", results, port)
	}
}

func TestManyWorkersGiveTheSameResultsAsOne(t *testing.T) {
	ports := []int{
		listenOnFreePort(t),
		freePortWithNothingListening(t),
		listenOnFreePort(t),
		freePortWithNothingListening(t),
		listenOnFreePort(t),
	}

	one := scanLocal(t, ports, 1)
	many := scanLocal(t, ports, 50)

	if len(one) != len(ports) {
		t.Fatalf("one worker checked %d of %d ports", len(one), len(ports))
	}
	if !reflect.DeepEqual(one, many) {
		t.Errorf("one worker gave %+v but 50 workers gave %+v", one, many)
	}
}

func TestResultsComeBackSortedByPort(t *testing.T) {
	ports := []int{
		freePortWithNothingListening(t),
		freePortWithNothingListening(t),
		freePortWithNothingListening(t),
	}

	results := scanLocal(t, ports, 3)

	for i := 1; i < len(results); i++ {
		if results[i-1].Port > results[i].Port {
			t.Fatalf("results are out of order: %+v", results)
		}
	}
}

func TestACancelledScanSaysItIsPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ports := make([]int, 200)
	for i := range ports {
		ports[i] = i + 1
	}

	results, err := Scan(ctx, "127.0.0.1", ports, 10, time.Second)

	if err == nil {
		t.Errorf("Scan returned no error after cancel, with %d of %d results", len(results), len(ports))
	}
	if len(results) != 0 {
		t.Errorf("a cancelled attempt was counted as a result: %+v", results)
	}
}

func TestAHostThatDoesNotExistGivesAClearError(t *testing.T) {
	_, err := ResolveHost(context.Background(), "no-such-host.invalid")

	if err == nil {
		t.Fatal("ResolveHost found an address for a name that cannot exist")
	}
	if !strings.Contains(err.Error(), "no-such-host.invalid") {
		t.Errorf("error %q does not name the host", err)
	}
}

func TestAnIPAddressIsUsedAsItIs(t *testing.T) {
	got, err := ResolveHost(context.Background(), "127.0.0.1")

	if err != nil || got != "127.0.0.1" {
		t.Errorf("ResolveHost(\"127.0.0.1\") = %q, %v", got, err)
	}
}

func runWithArgs(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestTheProgramListsAnOpenPortOnStdout(t *testing.T) {
	port := listenOnFreePort(t)

	code, stdout, stderr := runWithArgs("-ports", fmt.Sprint(port))

	if code != 0 {
		t.Errorf("exit code %d, want 0. stderr: %s", code, stderr)
	}
	if want := fmt.Sprintf("%d open\n", port); stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestTheProgramRefusesOptionsThatCannotWork(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{"a stray word is refused", []string{"80"}, "Did not expect"},
		{"zero workers is refused", []string{"-workers", "0"}, "workers"},
		{"a zero timeout is refused", []string{"-timeout", "0s"}, "timeout"},
		{"a bad port list is refused", []string{"-ports", "90-80"}, "backwards"},
		{"an unknown host is refused", []string{"-host", "no-such-host.invalid"}, "could not find the host"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, stdout, stderr := runWithArgs(c.args...)

			if code != 2 {
				t.Errorf("exit code %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing", stdout)
			}
			if !strings.Contains(stderr, c.wantMessage) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, c.wantMessage)
			}
		})
	}
}
