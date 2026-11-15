package meal

import "testing"

func TestForGrams(t *testing.T) {
	per100g := Nutrition{Kcal: 180, ProteinG: 3, CarbsG: 39.8, FatG: 0.3}
	tests := []struct {
		name  string
		grams float64
		want  Nutrition
	}{
		{name: "reference portion", grams: 100, want: per100g},
		{name: "double portion", grams: 200, want: Nutrition{Kcal: 360, ProteinG: 6, CarbsG: 79.6, FatG: 0.6}},
		{name: "triple portion", grams: 300, want: Nutrition{Kcal: 540, ProteinG: 9, CarbsG: 119.4, FatG: 0.9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ForGrams(per100g, tt.grams).rounded(); got != tt.want {
				t.Errorf("ForGrams() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNutritionAdd(t *testing.T) {
	sum := Nutrition{Kcal: 100, ProteinG: 5, CarbsG: 10, FatG: 2}.
		add(Nutrition{Kcal: 50, ProteinG: 1, CarbsG: 4, FatG: 3})
	want := Nutrition{Kcal: 150, ProteinG: 6, CarbsG: 14, FatG: 5}
	if sum != want {
		t.Errorf("add() = %+v, want %+v", sum, want)
	}
}
