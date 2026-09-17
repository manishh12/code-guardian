package metrics

import "testing"

func TestPaidEquivalent(t *testing.T) {
	s := &Store{}
	s.cfg.OpenAIInputUSDPer1M = 0.15
	s.cfg.OpenAIOutputUSDPer1M = 0.60
	got := s.PaidEquivalent(1_000_000, 1_000_000)
	if got < 0.74 || got > 0.76 {
		t.Fatalf("got %f", got)
	}
}
