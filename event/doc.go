// Package event provides an abstraction for single worker multi threaded applications. Some applications are multi
// threaded by design (e.g Kademlia lookup), but having a sequential execution brings many benefits such as
// deterministic testing, easier debugging, sequential tracing, and sometimes even increased performance.
package event
