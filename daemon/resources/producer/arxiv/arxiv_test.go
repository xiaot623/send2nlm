package producer

import "testing"

func TestArxivProducerMatchesArxivURLs(t *testing.T) {
	producer := &ArxivProducer{}

	tests := []string{
		"https://arxiv.org/abs/2605.29801",
		"https://arxiv.org/pdf/2605.29801v1",
	}

	for _, rawURL := range tests {
		if !producer.Match(rawURL) {
			t.Fatalf("expected %s to match", rawURL)
		}
	}

	if producer.Match("https://huggingface.co/papers/2605.29801") {
		t.Fatal("expected Hugging Face paper URL not to match arxiv producer")
	}
}

func TestArxivIDFromURL(t *testing.T) {
	tests := map[string]string{
		"https://arxiv.org/abs/2605.29801":   "2605.29801",
		"https://arxiv.org/pdf/2605.29801v1": "2605.29801",
	}

	for rawURL, want := range tests {
		got, err := arxivIDFromURL(rawURL)
		if err != nil {
			t.Fatalf("arxivIDFromURL(%q) returned error: %v", rawURL, err)
		}
		if got != want {
			t.Fatalf("arxivIDFromURL(%q) = %q, want %q", rawURL, got, want)
		}
	}
}
