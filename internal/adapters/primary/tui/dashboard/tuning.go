package dashboard

import "time"

const (
	BranchCacheRefreshInterval = 1 * time.Hour

	BranchCacheStaleThreshold = 5 * time.Minute
)
