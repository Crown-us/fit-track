package workout

import (
	"math"
	"testing"
)

const tolerance = 1e-9

func TestCaloriesBurned(t *testing.T) {
	tests := []struct {
		name        string
		met         float64
		weightKg    float64
		durationMin float64
		want        float64
	}{
		{name: "one hour", met: 9.8, weightKg: 70, durationMin: 60, want: 686},
		{name: "half hour", met: 3.5, weightKg: 60, durationMin: 30, want: 105},
		{name: "ninety minutes", met: 7.5, weightKg: 80, durationMin: 90, want: 900},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CaloriesBurned(tt.met, tt.weightKg, tt.durationMin)
			if math.Abs(got-tt.want) > tolerance {
				t.Errorf("CaloriesBurned() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMET(t *testing.T) {
	if met, ok := MET(ActivityRunning); !ok || met != 9.8 {
		t.Errorf("MET(running) = %v, %v; want 9.8, true", met, ok)
	}
	if _, ok := MET("teleporting"); ok {
		t.Error("MET() reported an unknown activity as supported")
	}
}

func TestActivitiesCoversTable(t *testing.T) {
	infos := Activities()
	if len(infos) != len(metValues) {
		t.Fatalf("Activities() lists %d activities, table has %d", len(infos), len(metValues))
	}
	for _, info := range infos {
		if info.MET <= 0 {
			t.Errorf("activity %q has no MET value", info.Activity)
		}
	}
}
