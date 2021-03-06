package procstats

import (
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/stats"
)

func TestGoMetrics(t *testing.T) {
	h := &handler{}
	e := stats.NewEngine("")
	e.Register(h)

	gostats := NewGoMetricsWith(e)
	gostats.Collect()

	if len(h.metrics) == 0 {
		t.Error("no metrics were reported by the stats collector")
	}

	for _, m := range h.metrics {
		switch {
		case strings.HasPrefix(m.Name, "go.runtime."):
		case strings.HasPrefix(m.Name, "go.memstats."):
		default:
			t.Error("invalid metric name:", m.Name)
		}
	}
}

func TestGoMetricsMock(t *testing.T) {
	now := time.Now()

	h := &handler{}
	e := stats.NewEngine("")
	e.Register(h)

	gostats := NewGoMetricsWith(e)
	gostats.gc.NumGC = 1
	gostats.gc.Pause = []time.Duration{time.Microsecond}
	gostats.gc.PauseEnd = []time.Time{now.Add(-time.Second)}
	gostats.updateMemStats(time.Now())

	if len(h.metrics) == 0 {
		t.Error("no metrics were reported by the stats collector")
	}

	for _, m := range h.metrics {
		switch {
		case strings.HasPrefix(m.Name, "go.runtime."):
		case strings.HasPrefix(m.Name, "go.memstats."):
		default:
			t.Error("invalid metric name:", m.Name)
		}
	}
}

func TestStripOutdatedGCPauses(t *testing.T) {
	now := time.Now()

	gc := &debug.GCStats{}
	gc.LastGC = now.Add(-time.Second) // 1s ago
	gc.NumGC = 10                     // 10th GC pass
	gc.PauseTotal = time.Millisecond  // 1ms pauses total
	gc.Pause = []time.Duration{
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
		100 * time.Microsecond,
	}
	gc.PauseEnd = []time.Time{
		now.Add(-time.Second).Add(-100 * time.Microsecond),
		now.Add(-time.Second).Add(-200 * time.Microsecond),
		now.Add(-time.Second).Add(-300 * time.Microsecond),
		now.Add(-time.Second).Add(-400 * time.Microsecond),
		now.Add(-time.Second).Add(-500 * time.Microsecond),
		now.Add(-time.Second).Add(-600 * time.Microsecond),
		now.Add(-time.Second).Add(-700 * time.Microsecond),
		now.Add(-time.Second).Add(-800 * time.Microsecond),
		now.Add(-time.Second).Add(-900 * time.Microsecond),
		now.Add(-time.Second).Add(-1 * time.Millisecond),
	}

	gc.PauseQuantiles = nil // ???

	stripOutdatedGCPauses(gc, 8) // last observed the 8th pass

	if !reflect.DeepEqual(gc, &debug.GCStats{
		LastGC:     now.Add(-time.Second),
		NumGC:      10,
		PauseTotal: time.Millisecond,
		Pause: []time.Duration{
			100 * time.Microsecond,
			100 * time.Microsecond,
		},
		PauseEnd: []time.Time{
			now.Add(-time.Second).Add(-100 * time.Microsecond),
			now.Add(-time.Second).Add(-200 * time.Microsecond),
		},
	}) {
		t.Errorf("invalid gc stats after stripping outdated pauses:\n%#v", *gc)
	}
}
