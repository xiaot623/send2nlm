package producer

import "testing"

func TestHFPaperProducerMatchesOnlyHFPaperURLs(t *testing.T) {
	producer := &HFPaperProducer{}

	if !producer.Match("https://huggingface.co/papers/2605.29801") {
		t.Fatal("expected Hugging Face paper URL to match")
	}
	if producer.Match("https://arxiv.org/abs/2605.29801") {
		t.Fatal("expected arXiv URL not to match hf-paper producer")
	}
}

func TestHFPaperIDFromURL(t *testing.T) {
	got, err := hfPaperIDFromURL("https://huggingface.co/papers/2605.29801")
	if err != nil {
		t.Fatalf("hfPaperIDFromURL returned error: %v", err)
	}
	if got != "2605.29801" {
		t.Fatalf("hfPaperIDFromURL = %q, want %q", got, "2605.29801")
	}
}
