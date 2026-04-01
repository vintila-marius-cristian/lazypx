package api

import "time"

// Centralized timeout constants for API and TUI operations.
const (
	TimeoutQuick     = 10 * time.Second  // Simple reads: GetNodes, Ping, task logs
	TimeoutStandard  = 30 * time.Second  // Cache refresh, cluster data fetch
	TimeoutLong      = 60 * time.Second  // State-changing operations: start/stop/reboot/delete
	TimeoutMigration = 120 * time.Second // VM migration
)
