# Port Scanner

A simple project coded in Go to practise understanding it.

It is a small concurrent TCP port scanner. Give it a host and a range of ports, and it tells you which ports accept a connection. The aim is to learn how Go handles goroutines, channels, contexts and tests, so the code carries comments that explain why each piece is there.

## What it will do

- Scan a host for open TCP ports, many at once, using a fixed pool of worker goroutines.
- Take a port list such as `80,443,8000-8100`, or a range such as `1-1024`.
- Stop cleanly on Ctrl+C and say that the results are partial.
- Report a failed check as a failure, never as a closed port.

## What it does not do

It does not scan UDP, read service banners or try to identify software. It checks whether a connection is accepted, and nothing more.

## Use it on your own machines only

Only scan hosts you own, or have written permission to scan. A scan of someone else's network can be against the law and is likely to be logged. The default target is `127.0.0.1`, which is your own computer.

## Status

Early. The usage section will be added once the scanner works.
