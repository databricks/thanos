// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package exemplars

import (
	"context"
	"net/url"

	"github.com/thanos-io/thanos/pkg/exemplars/exemplarspb"
	"github.com/thanos-io/thanos/pkg/promclient"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/tracing"
)

// Prometheus implements exemplarspb.Exemplars gRPC that allows to fetch exemplars from Prometheus.
type Prometheus struct {
	exemplarspb.UnimplementedExemplarsServer

	base   *url.URL
	client *promclient.Client

	extLabels func() labelpb.Labels
}

// NewPrometheus creates new exemplars.Prometheus.
func NewPrometheus(base *url.URL, client *promclient.Client, extLabels func() labelpb.Labels) *Prometheus {
	return &Prometheus{
		base:      base,
		client:    client,
		extLabels: extLabels,
	}
}

// Exemplars returns all specified exemplars from Prometheus.
func (p *Prometheus) Exemplars(r *exemplarspb.ExemplarsRequest, s exemplarspb.Exemplars_ExemplarsServer) error {
	exemplars, err := p.client.ExemplarsInGRPC(s.Context(), p.base, r.Query, r.Start, r.End)
	if err != nil {
		return err
	}

	// Prometheus does not add external labels, so we need to add on our own.
	ext := p.extLabels()
	for _, e := range exemplars {
		e.SeriesLabels = &labelpb.LabelSet{Labels: labelpb.ExtendSortedLabels(e.SeriesLabels.GetLabels(), ext)}

		var err error
		tracing.DoInSpan(s.Context(), "send_exemplars_response", func(_ context.Context) {
			err = s.Send(&exemplarspb.ExemplarsResponse{Result: &exemplarspb.ExemplarsResponse_Data{Data: e}})
		})
		if err != nil {
			return err
		}
	}
	return nil
}
