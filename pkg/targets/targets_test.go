// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package targets

import (
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/targets/targetspb"
)

func TestDedupTargets(t *testing.T) {
	for _, tc := range []struct {
		name          string
		targets, want *targetspb.TargetDiscovery
		replicaLabels []string
	}{
		{
			name:    "nil slice",
			targets: nil,
			want:    nil,
		},
		{
			name:          "dropped",
			replicaLabels: []string{"replica"},
			targets: &targetspb.TargetDiscovery{
				DroppedTargets: []*targetspb.DroppedTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:80",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
					},
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:80",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
					},
				},
			},
			want: &targetspb.TargetDiscovery{
				DroppedTargets: []*targetspb.DroppedTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:80",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
						),
					},
				},
			},
		},
		{
			name:          "active simple",
			replicaLabels: []string{"replica"},
			targets: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
					},
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
					},
				},
			},
			want: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
					},
				},
			},
		},
		{
			name:          "active unhealth first",
			replicaLabels: []string{"replica"},
			targets: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
					},
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_DOWN,
					},
				},
			},
			want: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_DOWN,
					},
				},
			},
		},
		{
			name:          "active latest scrape first",
			replicaLabels: []string{"replica"},
			targets: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "0",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
						LastScrape: timestamppb.New(time.Unix(1, 0)),
					},
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
							"replica", "1",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
						LastScrape: timestamppb.New(time.Unix(2, 0)),
					},
				},
			},
			want: &targetspb.TargetDiscovery{
				ActiveTargets: []*targetspb.ActiveTarget{
					{
						DiscoveredLabels: labelpb.LabelSetFromStrings(
							"__address__", "localhost:9090",
							"__metrics_path__", "/metrics",
							"__scheme__", "http",
							"job", "myself",
							"prometheus", "ha",
						),
						Labels: labelpb.LabelSetFromStrings(
							"instance", "localhost:9090",
							"job", "myself",
							"prometheus", "ha",
						),
						ScrapePool: "myself",
						ScrapeUrl:  "http://localhost:9090/metrics",
						Health:     targetspb.TargetHealth_UP,
						LastScrape: timestamppb.New(time.Unix(2, 0)),
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replicaLabels := make(map[string]struct{})
			for _, lbl := range tc.replicaLabels {
				replicaLabels[lbl] = struct{}{}
			}
			testutil.Equals(t, tc.want, dedupTargets(tc.targets, replicaLabels))
		})
	}
}
