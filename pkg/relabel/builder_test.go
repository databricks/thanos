package relabel

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

func TestBuilder(t *testing.T) {
	for i, tc := range []struct {
		base labelpb.Labels
		del  []string
		set  [][2]string
		want labelpb.Labels
	}{
		{
			base: labelpb.FromStrings("aaa", "111"),
			want: labelpb.FromStrings("aaa", "111"),
		},
		{
			base: nil,
			set:  [][2]string{{"aaa", "444"}, {"bbb", "555"}, {"ccc", "666"}},
			want: labelpb.FromStrings("aaa", "444", "bbb", "555", "ccc", "666"),
		},
		{
			base: labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"),
			set:  [][2]string{{"aaa", "444"}, {"bbb", "555"}, {"ccc", "666"}},
			want: labelpb.FromStrings("aaa", "444", "bbb", "555", "ccc", "666"),
		},
		{
			base: labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"),
			del:  []string{"bbb"},
			want: labelpb.FromStrings("aaa", "111", "ccc", "333"),
		},
		{
			set:  [][2]string{{"aaa", "111"}, {"bbb", "222"}, {"ccc", "333"}},
			del:  []string{"bbb"},
			want: labelpb.FromStrings("aaa", "111", "ccc", "333"),
		},
		{
			base: labelpb.FromStrings("aaa", "111"),
			set:  [][2]string{{"bbb", "222"}},
			want: labelpb.FromStrings("aaa", "111", "bbb", "222"),
		},
		{
			base: labelpb.FromStrings("aaa", "111"),
			set:  [][2]string{{"bbb", "222"}, {"bbb", "333"}},
			want: labelpb.FromStrings("aaa", "111", "bbb", "333"),
		},
		{
			base: labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"),
			del:  []string{"bbb"},
			set:  [][2]string{{"ddd", "444"}},
			want: labelpb.FromStrings("aaa", "111", "ccc", "333", "ddd", "444"),
		},
		{
			base: labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"),
			set:  [][2]string{{"bbb", ""}},
			want: labelpb.FromStrings("aaa", "111", "ccc", "333"),
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			b := newBuilder(tc.base)
			for _, s := range tc.set {
				b.Set(s[0], s[1])
			}
			b.Del(tc.del...)
			require.True(t, labelpb.Equal(tc.want, b.Labels()),
				"expected %s but got %s", tc.want, b.Labels())
		})
	}

	t.Run("set_after_del", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111"))
		b.Del("bbb")
		b.Set("bbb", "222")
		require.True(t, labelpb.Equal(labelpb.FromStrings("aaa", "111", "bbb", "222"), b.Labels()))
		require.Equal(t, "222", b.Get("bbb"))
	})

	t.Run("del_nonexistent", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111"))
		b.Del("zzz")
		require.True(t, labelpb.Equal(labelpb.FromStrings("aaa", "111"), b.Labels()))
	})

	t.Run("no_modifications_returns_base", func(t *testing.T) {
		base := labelpb.FromStrings("aaa", "111")
		b := newBuilder(base)
		got := b.Labels()
		require.True(t, labelpb.Equal(base, got))
		require.True(t, &base[0] == &got[0])
	})
}

func TestBuilderGet(t *testing.T) {
	base := labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333")
	b := newBuilder(base)

	require.Equal(t, "111", b.Get("aaa"))
	require.Equal(t, "222", b.Get("bbb"))
	require.Equal(t, "", b.Get("zzz"))

	b.Set("ddd", "444")
	require.Equal(t, "444", b.Get("ddd"))

	b.Set("bbb", "999")
	require.Equal(t, "999", b.Get("bbb"))

	b.Del("aaa")
	require.Equal(t, "", b.Get("aaa"))
}

func TestBuilderRange(t *testing.T) {
	t.Run("iterates_effective_set", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"))
		b.Del("bbb")
		b.Set("ddd", "444")

		got := map[string]string{}
		b.Range(func(l *labelpb.Label) {
			got[l.Name] = l.Value
		})
		require.Equal(t, map[string]string{
			"aaa": "111", "ccc": "333", "ddd": "444",
		}, got)
	})

	t.Run("set_override_visible", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111"))
		b.Set("aaa", "999")

		got := map[string]string{}
		b.Range(func(l *labelpb.Label) {
			got[l.Name] = l.Value
		})
		require.Equal(t, map[string]string{"aaa": "999"}, got)
	})

	t.Run("del_during_range", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111", "bbb", "222", "ccc", "333"))
		b.Range(func(l *labelpb.Label) {
			if l.Name == "bbb" {
				b.Del(l.Name)
			}
		})
		want := labelpb.FromStrings("aaa", "111", "ccc", "333")
		require.True(t, labelpb.Equal(want, b.Labels()),
			"expected %s but got %s", want, b.Labels())
	})

	t.Run("set_during_range_not_visited", func(t *testing.T) {
		b := newBuilder(labelpb.FromStrings("aaa", "111"))
		var visited []string
		b.Range(func(l *labelpb.Label) {
			visited = append(visited, l.Name)
			if l.Name == "aaa" {
				b.Set("zzz", "999")
			}
		})
		require.Equal(t, []string{"aaa"}, visited)
		require.Equal(t, "999", b.Get("zzz"))
	})

	t.Run("empty_builder", func(t *testing.T) {
		b := newBuilder(nil)
		count := 0
		b.Range(func(l *labelpb.Label) { count++ })
		require.Equal(t, 0, count)
	})
}

var benchmarkBuilderResult labelpb.Labels

func BenchmarkBuilder(b *testing.B) {
	benchLabels := [][2]string{
		{"job", "node"},
		{"instance", "123.123.1.211:9090"},
		{"path", "/api/v1/namespaces/<namespace>/deployments/<name>"},
		{"method", "GET"},
		{"namespace", "system"},
		{"status", "500"},
		{"prometheus", "prometheus-core-1"},
		{"datacenter", "eu-west-1"},
		{"pod_name", "abcdef-99999-defee"},
	}

	b.Run("set_from_empty", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bldr := newBuilder(nil)
			for _, l := range benchLabels {
				bldr.Set(l[0], l[1])
			}
			benchmarkBuilderResult = bldr.Labels()
		}
	})

	b.Run("del_and_set", func(b *testing.B) {
		base := labelpb.FromStrings(
			"datacenter", "eu-west-1",
			"instance", "123.123.1.211:9090",
			"job", "node",
			"method", "GET",
			"namespace", "system",
			"path", "/api/v1/namespaces/<namespace>/deployments/<name>",
			"pod_name", "abcdef-99999-defee",
			"prometheus", "prometheus-core-1",
			"status", "500",
		)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bldr := newBuilder(base)
			bldr.Del("method", "status")
			bldr.Set("new_label", "new_value")
			benchmarkBuilderResult = bldr.Labels()
		}
	})
}
