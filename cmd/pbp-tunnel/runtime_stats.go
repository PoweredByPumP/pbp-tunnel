package main

import (
	"log"
	"runtime"
	"time"
)

// logRuntimeStatsEvery helps spot goroutine and memory leaks in long-running tunnels.
func logRuntimeStatsEvery(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		log.Printf("DEBUG: Goroutines count: %d, Memory used: %d KB", runtime.NumGoroutine(), memory.Alloc/1024)
	}
}
