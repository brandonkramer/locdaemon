// Package locdaemon is the root module for local JSON-RPC daemon helpers.
//
// Subpackages:
//   - layout: svcroot layout paths and cross-platform ipc.Addr resolution
//   - runtime: lock, listen, accept loop, observe HTTP
//   - client: dial, spawn, JSON-RPC calls, observe reads
//   - observe: versioned observe-channel topic names
//   - runenv: child process environment and run-id verification helpers
package locdaemon
