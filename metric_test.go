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

func TestMetric_startStopUnlessStopped(t *testing.T) {
	var m Metric
	m.Start()
	time.Sleep(50 * time.Millisecond)
	m.Stop()

	d1 := m.Duration
	time.Sleep(50 * time.Millisecond)
	m.StopUnlessStopped()
	d2 := m.Duration
	time.Sleep(50 * time.Millisecond)
	m.Stop()
	d3 := m.Duration

	if d1 != d2 {
		t.Fatal("duration should not have been reset")
	}
	if d1 == d3 {
		t.Fatal("duration should have been reset")
	}
}
