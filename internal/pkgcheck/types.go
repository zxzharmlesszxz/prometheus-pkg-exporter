package pkgcheck

import (
	"context"
	"time"

	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
)

type Snapshot struct {
	AttemptTime               time.Time
	Success                   bool
	Err                       error
	Stats                     apt.Stats
	RebootRequired            bool
	CollectionDurationSeconds float64
}

type Snapshotter interface {
	Snapshot(context.Context) (Snapshot, error)
}
