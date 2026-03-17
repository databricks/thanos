// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package targets

import (
	"net/url"
	"strings"

	"github.com/thanos-io/thanos/pkg/promclient"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/targets/targetspb"
)

// Prometheus implements targetspb.Targets gRPC that allows to fetch targets from Prometheus HTTP api/v1/targets endpoint.
type Prometheus struct {
	targetspb.UnimplementedTargetsServer

	base   *url.URL
	client *promclient.Client

	extLabels func() labelpb.Labels
}

// NewPrometheus creates new targets.Prometheus.
func NewPrometheus(base *url.URL, client *promclient.Client, extLabels func() labelpb.Labels) *Prometheus {
	return &Prometheus{
		base:      base,
		client:    client,
		extLabels: extLabels,
	}
}

// Targets returns all specified targets from Prometheus.
func (p *Prometheus) Targets(r *targetspb.TargetsRequest, s targetspb.Targets_TargetsServer) error {
	var stateTargets string
	if r.State != targetspb.TargetsRequest_ANY {
		stateTargets = strings.ToLower(r.State.String())
	}
	targets, err := p.client.TargetsInGRPC(s.Context(), p.base, stateTargets)
	if err != nil {
		return err
	}

	// Prometheus does not add external labels, so we need to add on our own.
	enrichTargetsWithExtLabels(targets, p.extLabels())

	if err := s.Send(&targetspb.TargetsResponse{Result: &targetspb.TargetsResponse_Targets{Targets: targets}}); err != nil {
		return err
	}

	return nil
}

func enrichTargetsWithExtLabels(targets *targetspb.TargetDiscovery, ext labelpb.Labels) {
	for i, target := range targets.ActiveTargets {
		target.DiscoveredLabels = &labelpb.LabelSet{Labels: labelpb.ExtendSortedLabels(target.DiscoveredLabels.GetLabels(), ext)}
		target.Labels = &labelpb.LabelSet{Labels: labelpb.ExtendSortedLabels(target.Labels.GetLabels(), ext)}
		targets.ActiveTargets[i] = target
	}

	for i, target := range targets.DroppedTargets {
		target.DiscoveredLabels = &labelpb.LabelSet{Labels: labelpb.ExtendSortedLabels(target.DiscoveredLabels.GetLabels(), ext)}
		targets.DroppedTargets[i] = target
	}
}
