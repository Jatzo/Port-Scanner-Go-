# Learning notes

These are my notes for learning Go. Each section takes one idea the scanner uses and explains it with a snippet from the code. Anyone else learning Go is welcome to use them.

Every snippet comes from the code, so I can open the file next to these notes and find it. Where a snippet has `...`, lines have been left out to keep it short.

My reading order is `ports.go`, then `scanner.go`, then `main.go`, with [A Tour of Go](https://go.dev/tour) alongside.

## The picture first

```
feeder goroutine --port--> [jobs] --port--> workers --Result--> [results] --> Scan collects them
                                               |                                    ^
                                      probe() dials the port        closed when the wait group reaches 0
```

One goroutine feeds ports into a channel. A fixed number of workers take ports out, try to connect, and put the answer on a second channel. `Scan` reads the answers until that channel closes.

## 1. Channels: pipes between goroutines

From `scanner.go`:

```go
jobs := make(chan int)
results := make(chan Result)
```

A channel is a pipe. One goroutine puts a value in and another takes it out. `chan int` carries ports. `chan Result` carries answers.

Neither pipe has a size, so each is unbuffered: a send waits until something is ready to receive. That wait is what limits the work. Only `workers` goroutines are receiving, so only `workers` ports are being checked at once.

## 2. Goroutines: `go` runs a function alongside the rest

```go
go func() {
	workersRunning.Wait()
	close(results)
}()
```

Putting `go` in front of a call starts it in the background and carries on at once. `func() { ... }()` is an anonymous function that is written and called in one step. Goroutines are cheap, so 100 of them costs very little.

## 3. Ranging over a channel: the worker loop

```go
for port := range jobs {
	result, ok := probe(ctx, dialer, ip, port)
	...
}
```

This loop takes one port at a time from `jobs` and sleeps when the pipe is empty. It ends only when the pipe has been closed and emptied. That is why the feeder closes `jobs` when it is done: closing is the signal that there is no more work.

## 4. `defer`: run this when the function ends

```go
go func() {
	defer close(jobs)
	for _, port := range ports {
		...
	}
}()
```

`defer` schedules a call for the moment the function returns, by any route: the loop finishing, or the early `return` when the scan is cancelled. The pipe is always closed, because every way out goes through the `defer`.

## 5. `sync.WaitGroup`: count the workers, wait for zero

```go
workersRunning.Add(1)         // before starting a worker
defer workersRunning.Done()   // inside the worker, when it ends
workersRunning.Wait()         // blocks until the count is 0
```

Closing a channel that something is still sending on makes the program panic. The wait group is the counter that says when every sender has finished. When it reaches zero, `results` is closed, and the `for result := range results` loop in `Scan` ends.

## 6. `select` and `ctx.Done()`: wait on several things, take the first

```go
select {
case jobs <- port:
case <-ctx.Done():
	return
}
```

`select` waits on more than one channel operation and runs whichever is ready first. Here the port is either handed to a worker, or the scan is cancelled with Ctrl+C.

`ctx.Done()` is a channel that closes when the context is cancelled. Every `select` that watches it wakes up at once. Without that second case, a feeder whose workers had all stopped would wait forever, and the goroutine would be left behind. That is called a goroutine leak.

## 7. Errors are values, and interfaces are about behaviour

```go
var netErr net.Error
if isRefused(err) || (errors.As(err, &netErr) && netErr.Timeout()) {
```

Go has no exceptions. A function returns an `err` and the caller looks at it.

`errors.As` asks: is anything in this error's chain a `net.Error`? `net.Error` is an interface, which means any type with a `Timeout()` method qualifies. The code never needs to know the exact type. `errors.Is` asks a different question: is this one specific error somewhere in the chain? `isRefused` uses it to spot a refused connection.

## 8. A map as a set, and why the result is sorted

From `ports.go`:

```go
seen := make(map[int]bool)
seen[port] = true
```

Go has no set type, so a map with `true` values does the job. Adding the same port twice leaves one key. Map order is random in Go, so `ParsePorts` sorts the ports afterwards, and the scan is the same every time.

## Exercises for me

Each one teaches a single thing. I run `go test ./...` after each change.

1. **See the workers work.** Run `-workers 1`, then `-workers 100`, on the same ports. On my own machine the gap is small, because a closed port refuses at once. It grows when each port has to wait for a timeout.
2. **Break the feeder on purpose.** Delete the `case <-ctx.Done()` from the feeder's `select` and run the tests. They still pass, because the scan returns anyway. A goroutine is left stuck, and nothing in the tests notices. Write a test that catches it, for example by comparing `runtime.NumGoroutine()` before and after.
3. **Make `results` buffered.** Change it to `make(chan Result, 100)`. Nothing visible changes. Work out why, and what a buffer is for.
4. **Add a `-verbose` flag** that also prints the closed ports. Start in `run` in `main.go`.
5. **Add a `-json` flag** that prints the results as JSON. The `encoding/json` package does most of the work.
6. **Write the worker pool again from scratch** in a new file, using only the picture at the top of this page, then compare it with `scanner.go`.

## Where the tests are

The tests are part of the lesson. Their names read as sentences about behaviour, such as `TestAClosedPortIsReportedAsClosed`. They open real listeners on my own machine, so they need no network.
