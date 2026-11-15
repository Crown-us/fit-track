// Package workout logs exercise sessions and estimates the energy they burn.
package workout

// Activity identifies a supported exercise type.
type Activity string

// Supported activities.
const (
	ActivityWalking     Activity = "walking"
	ActivityRunning     Activity = "running"
	ActivityCycling     Activity = "cycling"
	ActivityGymStrength Activity = "gym_strength"
	ActivitySkipping    Activity = "skipping"
	ActivitySwimming    Activity = "swimming"
	ActivityYoga        Activity = "yoga"
	ActivityBadminton   Activity = "badminton"
)

const minutesPerHour = 60.0

// metValues maps each activity to its MET (metabolic equivalent of task).
//
// Source: Ainsworth BE, et al. "2011 Compendium of Physical Activities: a
// second update of codes and MET values." Med Sci Sports Exerc.
// 2011;43(8):1575-1581. The Compendium code is noted beside each value. One
// representative intensity is used per activity; the estimate is therefore
// approximate by design.
var metValues = map[Activity]float64{
	ActivityWalking:     3.5,  // 17190: walking 2.8-3.2 mph, level, moderate pace
	ActivityRunning:     9.8,  // 12050: running 6 mph (10 min/mile)
	ActivityCycling:     7.5,  // 01015: bicycling, general
	ActivityGymStrength: 3.5,  // 02054: resistance training, multiple exercises, 8-15 reps
	ActivitySkipping:    11.8, // 15551: rope jumping, moderate pace, 100-120 skips/min
	ActivitySwimming:    5.8,  // 18240: swimming laps, freestyle, light or moderate effort
	ActivityYoga:        2.5,  // 02150: yoga, Hatha
	ActivityBadminton:   5.5,  // 15030: badminton, social singles and doubles, general
}

// ActivityInfo describes a supported activity for API clients.
type ActivityInfo struct {
	Activity Activity `json:"activity"`
	MET      float64  `json:"met"`
}

// Activities lists every supported activity in a stable order.
func Activities() []ActivityInfo {
	ordered := []Activity{
		ActivityWalking, ActivityRunning, ActivityCycling, ActivityGymStrength,
		ActivitySkipping, ActivitySwimming, ActivityYoga, ActivityBadminton,
	}
	infos := make([]ActivityInfo, 0, len(ordered))
	for _, activity := range ordered {
		infos = append(infos, ActivityInfo{Activity: activity, MET: metValues[activity]})
	}
	return infos
}

// MET returns the MET value of activity and whether it is supported.
func MET(activity Activity) (float64, bool) {
	met, ok := metValues[activity]
	return met, ok
}

// CaloriesBurned estimates energy expenditure in kcal as
// MET x body weight (kg) x duration (hours).
func CaloriesBurned(met, weightKg, durationMin float64) float64 {
	return met * weightKg * (durationMin / minutesPerHour)
}
