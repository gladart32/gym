package main

import (
	"regexp"
	"strings"
)

const (
	trackWeightReps   = "weight_reps"
	trackReps         = "reps"
	trackTime         = "time"
	trackDistanceTime = "distance_time"
)

// Эвристика определения tracking_type по названию, категории и оборудованию.
// Это черновая разметка: ошибки правит администратор в БД, и при повторном
// импорте без -update они не затираются.
var (
	distanceRe = regexp.MustCompile(`\b(run|walk|walking|bike|elliptical|ergometer|stepmill|treadmill|sprints|cross trainer|skierg)\b`)

	timeRe = regexp.MustCompile(strings.Join([]string{
		`stretch`, `\bpose\b`, `yoga`,
		`^weighted front plank$`, `incline side plank`, `^side bridge v\. 2$`,
		`planche`, `front lever`, `back lever`, `l-sit`, `v-sit`, `maltese`, `\bflag\b`, `^handstand$`,
		`march sit`, `isometric`, `jump rope`, `battling ropes`, `balance board`, `circles`,
	}, "|"))

	// оборудование без внешнего веса: учитываем повторы, вес необязателен
	repsEquipment = map[string]bool{
		"body weight": true, "assisted": true, "band": true, "resistance band": true,
		"stability ball": true, "bosu ball": true, "roller": true, "wheel roller": true,
		"rope": true, "tire": true, "hammer": true,
	}

	// ручные исправления там, где эвристика ошибается
	trackingOverrides = map[string]string{
		"0003": trackReps,         // air bike — это скручивания «велосипед», не велотренажёр
		"3637": trackReps,         // wheel run — ролик для пресса
		"3638": trackReps,         // push to run
		"1312": trackReps,         // medicine ball chest push with run release — толчки медбола
		"3642": trackReps,         // weighted stretch lunge — выпады
		"2133": trackDistanceTime, // farmers walk — дистанция/время (+вес)
	}
)

func trackingType(e rawExercise) string {
	if t, ok := trackingOverrides[e.ID]; ok {
		return t
	}
	n := strings.ToLower(fixName(e.Name))
	isPush := strings.Contains(n, "push-up") || strings.Contains(n, "push up")

	if e.Category != "waist" && !strings.Contains(n, "lunge") && distanceRe.MatchString(n) {
		return trackDistanceTime
	}
	if !isPush && !strings.HasSuffix(n, " reps") && timeRe.MatchString(n) {
		return trackTime
	}
	if repsEquipment[e.Equipment] {
		return trackReps
	}
	return trackWeightReps
}
