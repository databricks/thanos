package infopb

import (
	"math"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

func NewTSDBInfo(mint, maxt int64, lbls labelpb.Labels) *TSDBInfo {
	return &TSDBInfo{
		Labels: &labelpb.LabelSet{
			Labels: lbls,
		},
		MinTime: mint,
		MaxTime: maxt,
	}
}

type TSDBInfos []*TSDBInfo

func (infos TSDBInfos) MaxT() int64 {
	var maxt int64 = math.MinInt64
	for _, info := range infos {
		if info.MaxTime > maxt {
			maxt = info.MaxTime
		}
	}
	return maxt
}

func (infos TSDBInfos) LabelSets() []labelpb.Labels {
	lsets := make([]labelpb.Labels, len(infos))
	for i, info := range infos {
		lsets[i] = info.GetLabels().GetLabels()
	}
	return lsets
}
