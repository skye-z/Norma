package runtime

import "testing"

func TestQualityControllerSearchTuning(t *testing.T) {
	q := NewQualityController(true, 1200, 3)
	q.SetDegradedEFSearch(18)
	q.SetDegradedRetrieverFactors(1, 2)
	q.SetDegradedAudioChunkSize(2048)
	q.SetDegradedLLMModel("qwen2.5:3b")
	q.SetDegradedTTSVoice("zh_CN-huayan-x_low")
	q.ObserveLLMFirstToken(1500)

	tuning := q.SearchTuning(6)
	if tuning.TopK != 3 {
		t.Fatalf("expected topK 3, got %d", tuning.TopK)
	}
	if tuning.VectorEFSearch != 18 {
		t.Fatalf("expected ef 18, got %d", tuning.VectorEFSearch)
	}
	if tuning.RetrieverExtraFactor != 1 || tuning.RetrieverFuseFactor != 2 {
		t.Fatalf("unexpected tuning: %+v", tuning)
	}
	if q.AudioChunkSize(8192) != 2048 {
		t.Fatalf("expected degraded audio chunk size")
	}
	if q.LLMModel("qwen2.5:7b") != "qwen2.5:3b" {
		t.Fatalf("expected degraded llm model")
	}
	if q.TTSVoice("zh_CN-huayan-medium") != "zh_CN-huayan-x_low" {
		t.Fatalf("expected degraded tts voice")
	}
}

func TestQualityControllerDefaultTuning(t *testing.T) {
	q := NewQualityController(true, 1200, 4)
	tuning := q.SearchTuning(6)
	if tuning.TopK != 6 {
		t.Fatalf("expected default topK, got %d", tuning.TopK)
	}
	if tuning.RetrieverExtraFactor != 2 || tuning.RetrieverFuseFactor != 3 {
		t.Fatalf("unexpected default tuning: %+v", tuning)
	}
}
