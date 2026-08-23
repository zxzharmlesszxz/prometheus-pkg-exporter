package pkg

import (
	"context"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/apt"
	"github.com/zxzharmlesszxz/prometheus-pkg-exporter/internal/pkgcheck"
)

func NewDefaultSnapshotEngine() featurekit.SnapshotEngine[Snapshot] {
	engine, err := newSnapshotEngine(NewDefaultConfig())
	if err != nil {
		panic(err)
	}
	return engine
}

func NewSnapshotEngine(ctx featurekit.CollectorContext[Config]) (featurekit.SnapshotEngine[Snapshot], error) {
	return newSnapshotEngine(ctx.Config)
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.pkg.AttemptTime,
		Success:     snapshot.pkg.Success,
	}
}

func newSnapshotEngine(config Config) (featurekit.SnapshotEngine[Snapshot], error) {
	checker := pkgcheck.NewChecker(
		apt.CommandSource{
			StatusDatabasePath: config.StatusDatabasePath,
			APTListsPath:       config.APTListsPath,
		},
		config.CommandTimeout,
		config.RebootRequiredPath,
	)

	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		return Snapshot{
			pkg: checker.Snapshot(ctx, now),
		}
	}), nil
}
