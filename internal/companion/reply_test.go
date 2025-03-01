package companion

import "testing"

func TestPlanReply(t *testing.T) {
	plan := PlanReply("先给结论。然后补充说明！最后提醒")
	if plan.Conclusion != "先给结论" {
		t.Fatalf("unexpected conclusion: %q", plan.Conclusion)
	}
	if len(plan.Segments) != 3 {
		t.Fatalf("unexpected segments: %+v", plan.Segments)
	}
}

func TestAnalyzeEmotion(t *testing.T) {
	spec := AnalyzeEmotion("注意，这里有风险，需要先确认。")
	if spec.Emotion != "alert" {
		t.Fatalf("unexpected emotion: %+v", spec)
	}
}
