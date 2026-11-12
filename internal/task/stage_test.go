package task

import "testing"

func TestNormalizeStagesDedupesAndOrdersValues(t *testing.T) {
	got := NormalizeStages([]string{" Publish ", "implement", "PUBLISH", "", "verify"})
	want := []string{"publish", "implement", "verify"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// A nil declaration and an empty declaration mean different things: nil keeps
// the behaviour that existed before stages, while an empty list states that no
// stage is implied. Conflating them would silently change older tasks.
func TestStagesDeclaredDistinguishesNilFromEmpty(t *testing.T) {
	if StagesDeclared(nil) {
		t.Fatal("nil must mean not declared")
	}
	if !StagesDeclared([]string{}) {
		t.Fatal("empty list must mean declared with no stages")
	}
	if RequiresPublish(nil) || RequiresPublish([]string{}) {
		t.Fatal("publish must never be implied")
	}
	if !RequiresPublish([]string{StageImplement, StagePublish}) {
		t.Fatal("declared publish must be honoured")
	}
}

func TestInvalidStagesReportsUnknownValues(t *testing.T) {
	invalid := InvalidStages([]string{"implement", "deploy", "publish"})
	if len(invalid) != 1 || invalid[0] != "deploy" {
		t.Fatalf("got %v, want [deploy]", invalid)
	}
}

func TestDescribeStagesUsesOwnerLanguage(t *testing.T) {
	if got := DescribeStages(nil); got != "" {
		t.Fatalf("undeclared stages must describe nothing, got %q", got)
	}
	if got := DescribeStages([]string{StageImplement, StageVerify}); got != "kerjakan perubahannya, pastikan hasilnya benar" {
		t.Fatalf("unexpected description: %q", got)
	}
	if got := DescribeStages([]string{}); got != "tanpa tahap lanjutan" {
		t.Fatalf("empty declaration description: %q", got)
	}
}
