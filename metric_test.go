package servertiming

import (
	"testing"
	"time"
)

func TestMetric_startStop(t *testing.T) {
	var m Metric
	m.Start()
	time.Sleep(50 * time.Millisecond)
	m.Stop()

	actual := m.Duration
	if actual == 0 {
		t.Fatal("duration should be set")
	}
	if actual > 100*time.Millisecond {
		t.Fatal("expected duration to be within 100ms")
	}
	if actual < 30*time.Millisecond {
		t.Fatal("expected duration to be more than 30ms")
	}
}

func TestMetric_stopNoStart(t *testing.T) {
	var m Metric
	m.Stop()

	actual := m.Duration
	if actual != 0 {
		t.Fatal("duration should not be set")
	}
}
