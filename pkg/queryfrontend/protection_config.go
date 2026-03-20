// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"
	"crypto/sha256"
	"os"
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// ProtectionConfig is the top-level structure of the protection config file.
type ProtectionConfig struct {
	Rules []RuleConfig `yaml:"rules"`
}

// RuleConfig is the config for a single protection rule.
type RuleConfig struct {
	Name       string            `yaml:"name"`
	Protection string            `yaml:"protection"`
	Action     string            `yaml:"action"`
	Actor      string            `yaml:"actor"`
	Enabled    bool              `yaml:"enabled"`
	Args       map[string]string `yaml:"args"`
}

// loadRulesFromFile reads a YAML config file and constructs a list of Rules.
func loadRulesFromFile(path string) ([]*Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read protection config file")
	}

	var cfg ProtectionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Wrap(err, "parse protection config")
	}

	rules := make([]*Rule, 0, len(cfg.Rules))
	for _, rc := range cfg.Rules {
		rule, err := buildRule(rc)
		if err != nil {
			return nil, errors.Wrapf(err, "build rule %q", rc.Name)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func buildRule(rc RuleConfig) (*Rule, error) {
	factory, err := LookupProtection(rc.Protection)
	if err != nil {
		return nil, err
	}

	protection, err := factory(rc.Args)
	if err != nil {
		return nil, errors.Wrap(err, "construct protection")
	}

	action, err := parseAction(rc.Action)
	if err != nil {
		return nil, err
	}

	if rc.Name == "" {
		return nil, errors.New("name is required")
	}
	if rc.Actor == "" {
		return nil, errors.New("actor is required")
	}
	actorRegex, err := regexp.Compile(rc.Actor)
	if err != nil {
		return nil, errors.Wrapf(err, "compile actor regex %q", rc.Actor)
	}

	return NewRule(rc.Name, protection, action, actorRegex, rc.Enabled), nil
}

func parseAction(s string) (RuleAction, error) {
	switch s {
	case "log":
		return RuleActionLog, nil
	case "block":
		return RuleActionBlock, nil
	default:
		return RuleActionLog, errors.Errorf("unknown action %q, must be log or block", s)
	}
}

// WatchConfig watches the protection config file for changes and reloads the engine.
// It polls the file at the given interval, detecting changes via SHA256 checksum.
// Blocks until ctx is canceled.
func WatchConfig(ctx context.Context, engine *ProtectionEngine, path string, logger log.Logger, interval time.Duration) error {
	if err := reloadRules(engine, path, logger); err != nil {
		return errors.Wrap(err, "load initial protection config")
	}

	var previousChecksum [sha256.Size]byte

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
			data, err := os.ReadFile(path)
			if err != nil {
				level.Error(logger).Log("msg", "failed to read protection config", "path", path, "err", err)
				continue
			}
			checksum := sha256.Sum256(data)
			if checksum == previousChecksum {
				continue
			}
			previousChecksum = checksum
			if err := reloadRules(engine, path, logger); err != nil {
				level.Error(logger).Log("msg", "failed to reload protection config", "path", path, "err", err)
			}
		}
	}
}

func reloadRules(engine *ProtectionEngine, path string, logger log.Logger) error {
	rules, err := loadRulesFromFile(path)
	if err != nil {
		return errors.Wrap(err, "load protection rules")
	}
	engine.UpdateRules(rules)
	level.Info(logger).Log("msg", "protection config reloaded", "path", path, "rules", len(rules))
	return nil
}
