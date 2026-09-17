package rag

import "testing"

func TestEmbedTextIsNormalizedAndStable(t *testing.T) {
	a := EmbedText("flag hardcoded secrets and SQL injection")
	b := EmbedText("flag hardcoded secrets and SQL injection")
	if len(a) != Dims {
		t.Fatalf("dims=%d", len(a))
	}
	var sum float64
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("embedding is not stable")
		}
		sum += float64(a[i]) * float64(a[i])
	}
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("expected unit vector, got %f", sum)
	}
}

func TestEmbedTextDiffersByTopic(t *testing.T) {
	sec := EmbedText("SQL injection hardcoded password exec user input")
	style := EmbedText("prefer explicit types keep functions small")
	var dot float64
	for i := range sec {
		dot += float64(sec[i] * style[i])
	}
	if dot > 0.95 {
		t.Fatalf("unrelated texts should not be almost identical, cosine=%f", dot)
	}
}

func TestVectorLiteral(t *testing.T) {
	got := VectorLiteral([]float32{0.5, -0.25})
	if got != "[0.500000,-0.250000]" {
		t.Fatalf("got %s", got)
	}
}
