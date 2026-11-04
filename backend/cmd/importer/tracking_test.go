package main

import "testing"

func TestTrackingType(t *testing.T) {
	cases := []struct {
		e    rawExercise
		want string
	}{
		{rawExercise{Name: "barbell bench press", Equipment: "barbell", Category: "chest"}, trackWeightReps},
		{rawExercise{Name: "lever leg extension", Equipment: "leverage machine", Category: "upper legs"}, trackWeightReps},
		{rawExercise{Name: "push-up", Equipment: "body weight", Category: "chest"}, trackReps},
		{rawExercise{Name: "full planche push-up", Equipment: "body weight", Category: "chest"}, trackReps},
		{rawExercise{Name: "front lever reps", Equipment: "body weight", Category: "back"}, trackReps},
		{rawExercise{Name: "weighted pull-up", Equipment: "weighted", Category: "back"}, trackWeightReps},
		{rawExercise{Name: "hamstring stretch", Equipment: "body weight", Category: "upper legs"}, trackTime},
		{rawExercise{Name: "weighted front plank", Equipment: "weighted", Category: "waist"}, trackTime},
		{rawExercise{Name: "front lever", Equipment: "body weight", Category: "back"}, trackTime},
		{rawExercise{Name: "run", Equipment: "body weight", Category: "cardio"}, trackDistanceTime},
		{rawExercise{Name: "walking on incline treadmill", Equipment: "body weight", Category: "cardio"}, trackDistanceTime},
		{rawExercise{Name: "walking lunge", Equipment: "body weight", Category: "upper legs"}, trackReps},
		{rawExercise{ID: "0003", Name: "air bike", Equipment: "body weight", Category: "waist"}, trackReps},
		{rawExercise{Name: "sled 45в° leg press", Equipment: "sled machine", Category: "upper legs"}, trackWeightReps},
	}
	for _, c := range cases {
		if got := trackingType(c.e); got != c.want {
			t.Errorf("%q: got %s, want %s", c.e.Name, got, c.want)
		}
	}
}

func TestFixName(t *testing.T) {
	if got := fixName("sled 45в° leg press"); got != "sled 45° leg press" {
		t.Fatalf("got %q", got)
	}
}

func TestCanonicalMuscles(t *testing.T) {
	s := &sourceData{Muscles: map[string]muscleTr{
		"abs":         {Canonical: "abs"},
		"abdominals":  {Canonical: "abs"},
		"hip flexors": {Canonical: "hip flexors"},
		"lower back":  {Canonical: "lower back"},
	}}
	target, sec := s.canonicalMuscles(rawExercise{
		Target: "abs", MuscleGroup: "hip flexors",
		SecondaryMuscles: []string{"hip flexors", "abdominals", "lower back"},
	})
	if target != "abs" || len(sec) != 2 || sec[0] != "hip flexors" || sec[1] != "lower back" {
		t.Fatalf("got %s %v", target, sec)
	}
}
