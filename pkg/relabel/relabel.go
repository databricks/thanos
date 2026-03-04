package relabel

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"github.com/thanos-io/thanos/pkg/unique"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

// Process returns a relabeled version of the given label set. The relabel configurations
// are applied in order of input.
// There are circumstances where Process will modify the input label.
// If you want to avoid issues with the input label set being modified, at the cost of
// higher memory usage, you can use lbls.Copy().
// If a label set is dropped, EmptyLabels and false is returned.
// Note! this is a clone of the Prometheus relabel.Process function,
// introduced for efficencey reasons.
func Process(lbls labelpb.Labels, cfgs ...*relabel.Config) (ret labelpb.Labels, keep bool) {
	lb := newBuilder(lbls)
	for _, cfg := range cfgs {
		if !relabelWithConfig(cfg, lb) {
			return labelpb.EmptyLabels(), false
		}
	}
	return lb.Labels(), true
}

func relabelWithConfig(cfg *relabel.Config, lb *builder) (keep bool) {
	var va [16]string
	values := va[:0]
	if len(cfg.SourceLabels) > cap(values) {
		values = make([]string, 0, len(cfg.SourceLabels))
	}
	for _, ln := range cfg.SourceLabels {
		values = append(values, lb.Get(string(ln)))
	}
	val := strings.Join(values, cfg.Separator)

	switch cfg.Action {
	case relabel.Drop:
		if cfg.Regex.MatchString(val) {
			return false
		}
	case relabel.Keep:
		if !cfg.Regex.MatchString(val) {
			return false
		}
	case relabel.DropEqual:
		if lb.Get(cfg.TargetLabel) == val {
			return false
		}
	case relabel.KeepEqual:
		if lb.Get(cfg.TargetLabel) != val {
			return false
		}
	case relabel.Replace:
		indexes := cfg.Regex.FindStringSubmatchIndex(val)
		// If there is no match no replacement must take place.
		if indexes == nil {
			break
		}
		target := model.LabelName(cfg.Regex.ExpandString([]byte{}, cfg.TargetLabel, val, indexes))
		if !target.IsValid() {
			break
		}
		res := cfg.Regex.ExpandString([]byte{}, cfg.Replacement, val, indexes)
		if len(res) == 0 {
			lb.Del(string(target))
			break
		}
		name := string(target)
		value := string(res)
		// Only intern and set if the value actually changed; avoids contention on unique's global map.
		if lb.Get(name) != value {
			name = unique.Make(name).Value()
			value = unique.Make(value).Value()
			lb.Set(name, value)
		}
	case relabel.Lowercase:
		value := strings.ToLower(val)
		// Only intern and set if the value actually changed; avoids contention on unique's global map.
		if lb.Get(cfg.TargetLabel) != value {
			value = unique.Make(value).Value()
			lb.Set(cfg.TargetLabel, value)
		}
	case relabel.Uppercase:
		value := strings.ToUpper(val)
		// Only intern and set if the value actually changed; avoids contention on unique's global map.
		if lb.Get(cfg.TargetLabel) != value {
			value = unique.Make(value).Value()
			lb.Set(cfg.TargetLabel, value)
		}
	case relabel.HashMod:
		hash := md5.Sum([]byte(val))
		// Use only the last 8 bytes of the hash to give the same result as earlier versions of this code.
		mod := binary.BigEndian.Uint64(hash[8:]) % cfg.Modulus
		value := strconv.FormatUint(mod, 10)
		// Only intern and set if the value actually changed; avoids contention on unique's global map.
		if lb.Get(cfg.TargetLabel) != value {
			value = unique.Make(value).Value()
			lb.Set(cfg.TargetLabel, value)
		}
	case relabel.LabelMap:
		lb.Range(func(l *labelpb.Label) {
			if cfg.Regex.MatchString(l.Name) {
				name := cfg.Regex.ReplaceAllString(l.Name, cfg.Replacement)
				// Only intern and set if the value actually changed; avoids contention on unique's global map.
				if name != l.Name {
					name = unique.Make(name).Value()
					lb.Set(name, l.Value)
				}
			}
		})
	case relabel.LabelDrop:
		lb.Range(func(l *labelpb.Label) {
			if cfg.Regex.MatchString(l.Name) {
				lb.Del(l.Name)
			}
		})
	case relabel.LabelKeep:
		lb.Range(func(l *labelpb.Label) {
			if !cfg.Regex.MatchString(l.Name) {
				lb.Del(l.Name)
			}
		})
	default:
		panic(fmt.Errorf("relabel: unknown relabel action type %q", cfg.Action))
	}

	return true
}
