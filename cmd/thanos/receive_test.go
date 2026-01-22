package main

import (
	"testing"

	"github.com/efficientgo/core/testutil"
)

func TestReceiveConfigValidateReplicaGroupQuorum(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		replicaGroup string
		quorum       int
		expectErr    bool
	}{
		{
			name:         "both unset",
			replicaGroup: "",
			quorum:       0,
			expectErr:    false,
		},
		{
			name:         "both set",
			replicaGroup: "rg",
			quorum:       1,
			expectErr:    false,
		},
		{
			name:         "only quorum set",
			replicaGroup: "",
			quorum:       1,
			expectErr:    true,
		},
		{
			name:         "only replica-group set (quorum default)",
			replicaGroup: "rg",
			quorum:       0,
			expectErr:    true,
		},
		{
			name:         "replica-group set with negative quorum",
			replicaGroup: "rg",
			quorum:       -1,
			expectErr:    true,
		},
		{
			name:         "negative quorum without replica-group",
			replicaGroup: "",
			quorum:       -1,
			expectErr:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := &receiveConfig{
				replicaGroup: tc.replicaGroup,
				quorum:       tc.quorum,
			}
			err := conf.validate()
			if tc.expectErr {
				testutil.NotOk(t, err)
				return
			}
			testutil.Ok(t, err)
		})
	}
}
