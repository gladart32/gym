package stats_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/app"
	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/server"
	"github.com/gladart32/gym/internal/testutil"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

type apiEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

const testPassword = "test-pass-123"

func setup(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()

	a := testutil.Start(t)
	gin.SetMode(gin.TestMode)
	router := server.NewRouter(a.Log, a.Config,
		handler.NewHealth(a.Log, a.DB, a.Redis),
		handler.NewTasks(a.Log, a.Tasks),
		handler.NewAuth(a.Log, a.Config, a.Auth, a.Tasks),
		a.Auth,
		handler.NewProfile(a.Log, a.Profile),
		a.Profile,
		handler.NewExercises(a.Log, a.Exercises),
		handler.NewPrograms(a.Log, a.Programs),
		a.Programs,
		handler.NewSchedule(a.Log, a.Schedule),
		a.Schedule,
		handler.NewWorkouts(a.Log, a.Workouts),
		a.Workouts,
		handler.NewStats(a.Log, a.Stats),
		a.Stats,
	)

	clean(t, a)
	return router, a
}

func clean(t *testing.T, a *app.App) {
	t.Helper()
	for _, q := range []string{
		`DELETE FROM workout_sets`,
		`DELETE FROM workout_exercises`,
		`DELETE FROM workouts`,
		`DELETE FROM user_schedule_rules`,
		`DELETE FROM user_programs`,
		`DELETE FROM program_schedule_rules`,
		`DELETE FROM program_sets`,
		`DELETE FROM program_exercises`,
		`DELETE FROM program_days`,
		`DELETE FROM programs`,
		`DELETE FROM exercise_media`,
		`DELETE FROM exercise_translations`,
		`DELETE FROM exercises`,
	} {
		if _, err := a.DB.Exec(q); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
}

func do(t *testing.T, router *gin.Engine, method, path, bearer string, body any) (*httptest.ResponseRecorder, apiEnvelope) {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var env apiEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec, env
}

// registerAndLogin creates a verified user (UTC timezone) and returns
// the access token and the user id.
func registerAndLogin(t *testing.T, router *gin.Engine, a *app.App) (string, uint32) {
	t.Helper()

	email := fmt.Sprintf("st%d@gym.test", time.Now().UnixNano())
	body := map[string]string{"email": email, "password": testPassword, "timezone": "UTC"}
	rec, _ := do(t, router, http.MethodPost, "/api/v1/auth/register", "", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var userID uint32
	if err := a.DB.Get(&userID, `SELECT id FROM users WHERE email = ?`, email); err != nil {
		t.Fatalf("select user: %v", err)
	}
	ctx := context.Background()
	verToken, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenEmailVerify)
	if err != nil {
		t.Fatalf("create email token: %v", err)
	}
	if err := a.Auth.VerifyEmail(ctx, verToken); err != nil {
		t.Fatalf("verify email: %v", err)
	}

	rec, env := do(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email":    email,
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", rec.Code)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(env.Data, &tokens); err != nil {
		t.Fatalf("unmarshal tokens: %v", err)
	}
	return tokens.AccessToken, userID
}

// seedExercise inserts a weight_reps exercise and returns its id.
func seedExercise(t *testing.T, a *app.App) uint32 {
	t.Helper()

	var id uint32
	err := a.DB.Get(&id, `SELECT id FROM exercises WHERE external_id = 'st-0001'`)
	if err == nil {
		return id
	}

	insertRef := func(table, code, nameEn, nameRu string) uint16 {
		t.Helper()
		// Справочники не чистятся между тестами/прогонами — INSERT IGNORE.
		if _, err := a.DB.Exec(fmt.Sprintf("INSERT IGNORE INTO %s (code, name_en, name_ru) VALUES (?, ?, ?)", table), code, nameEn, nameRu); err != nil {
			t.Fatalf("seed %s: %v", table, err)
		}
		var id uint16
		if err := a.DB.Get(&id, fmt.Sprintf("SELECT id FROM %s WHERE code = ?", table), code); err != nil {
			t.Fatalf("seed %s id: %v", table, err)
		}
		return id
	}
	chestID := insertRef("body_parts", "st-chest", "Chest", "Грудь")
	barbellID := insertRef("equipment", "st-barbell", "Barbell", "Штанга")
	pectoralsID := insertRef("muscles", "st-pectorals", "Pectorals", "Грудные")

	res, err := a.DB.Exec(`
		INSERT INTO exercises (source, external_id, body_part_id, equipment_id, target_muscle_id, tracking_type)
		VALUES ('manual', 'st-0001', ?, ?, ?, 'weight_reps')`, chestID, barbellID, pectoralsID)
	if err != nil {
		t.Fatalf("seed exercise: %v", err)
	}
	id64, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed exercise id: %v", err)
	}
	if _, err := a.DB.Exec(`
		INSERT INTO exercise_translations (exercise_id, lang, name) VALUES (?, 'en', 'Bench Press')`, id64); err != nil {
		t.Fatalf("seed translation: %v", err)
	}
	return uint32(id64)
}

// selectProgram creates a personal program with one day (weekly mon/wed/fri
// by default), selects it and returns (user_program_id, program_day_id).
func selectProgram(t *testing.T, router *gin.Engine, bearer string, startDate string, weekdays ...string) (uint32, uint32) {
	t.Helper()

	if len(weekdays) == 0 {
		weekdays = []string{"mon", "wed", "fri"}
	}
	body := map[string]any{
		"name": "Stats Program",
		"days": []map[string]any{{
			"name": "День A",
			"schedule": map[string]any{
				"frequency":  "weekly",
				"interval_n": 1,
				"weekdays":   weekdays,
			},
			"exercises": []map[string]any{},
		}},
	}
	rec, env := do(t, router, http.MethodPost, "/api/v1/programs", bearer, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create program: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var program struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &program); err != nil {
		t.Fatalf("unmarshal program: %v", err)
	}

	rec, env = do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/programs/%d/select", program.ID), bearer, map[string]string{"start_date": startDate})
	if rec.Code != http.StatusOK {
		t.Fatalf("select program: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec, env = do(t, router, http.MethodGet, "/api/v1/schedule", bearer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("schedule: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var ups []struct {
		ID        uint32 `json:"id"`
		ProgramID uint32 `json:"program_id"`
	}
	if err := json.Unmarshal(env.Data, &ups); err != nil || len(ups) != 1 {
		t.Fatalf("schedule: expected 1 user program, got %d (%v)", len(ups), err)
	}

	rec, env = do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/programs/%d", ups[0].ProgramID), bearer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("program detail: expected 200, got %d", rec.Code)
	}
	var detail struct {
		Days []struct {
			ID uint32 `json:"id"`
		} `json:"days"`
	}
	if err := json.Unmarshal(env.Data, &detail); err != nil || len(detail.Days) != 1 {
		t.Fatalf("program detail: expected 1 day, got %d (%v)", len(detail.Days), err)
	}
	return ups[0].ID, detail.Days[0].ID
}

// insertWorkout inserts a completed (or cancelled) workout with one exercise
// and the given sets, at a historical started_at moment.
func insertWorkout(t *testing.T, a *app.App, userID uint32, userProgramID, programDayID uint32, name, status string, startedAt time.Time, sets []map[string]any) int64 {
	t.Helper()

	exerciseID := seedExercise(t, a)

	res, err := a.DB.Exec(`
		INSERT INTO workouts (user_id, user_program_id, program_day_id, name, status, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, userProgramID, programDayID, name, status, startedAt, startedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("insert workout: %v", err)
	}
	workoutID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("insert workout id: %v", err)
	}

	res, err = a.DB.Exec(`
		INSERT INTO workout_exercises (workout_id, exercise_id, position, is_skipped)
		VALUES (?, ?, 1, FALSE)`, workoutID, exerciseID)
	if err != nil {
		t.Fatalf("insert workout exercise: %v", err)
	}
	weID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("insert workout exercise id: %v", err)
	}

	for i, s := range sets {
		var completedAt any = startedAt.Add(time.Duration(5*i) * time.Minute)
		if done, ok := s["done"].(bool); ok && !done {
			completedAt = nil // подход не выполнен
		}
		if _, err := a.DB.Exec(`
			INSERT INTO workout_sets
				(workout_exercise_id, set_number, planned_reps, planned_weight_kg,
				 reps, weight_kg, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			weID, i+1,
			s["planned_reps"], s["planned_weight_kg"],
			s["reps"], s["weight_kg"],
			completedAt,
		); err != nil {
			t.Fatalf("insert workout set: %v", err)
		}
	}
	return workoutID
}

type streakJSON struct {
	Current   int `json:"current"`
	Scheduled int `json:"scheduled"`
	Completed int `json:"completed"`
}

type weekJSON struct {
	WeekStart     string  `json:"week_start"`
	Workouts      int     `json:"workouts"`
	SetsDone      int     `json:"sets_done"`
	TotalVolumeKg float64 `json:"total_volume_kg"`
}

type lastWorkoutJSON struct {
	Date          string  `json:"date"`
	Name          string  `json:"name"`
	DurationSec   int     `json:"duration_sec"`
	SetsDone      int     `json:"sets_done"`
	ExercisesDone int     `json:"exercises_done"`
	TotalVolumeKg float64 `json:"total_volume_kg"`
}

type dashboardJSON struct {
	WorkoutsMonth  int              `json:"workouts_month"`
	WorkoutsTotal  int              `json:"workouts_total"`
	TonnageMonthKg float64          `json:"tonnage_month_kg"`
	Streak         *streakJSON      `json:"streak"`
	WeeklyActivity []weekJSON       `json:"weekly_activity"`
	LastWorkout    *lastWorkoutJSON `json:"last_workout"`
}

func dashboard(t *testing.T, router *gin.Engine, bearer string) (*httptest.ResponseRecorder, apiEnvelope, *dashboardJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodGet, "/api/v1/stats/dashboard", bearer, nil)
	if env.Data == nil || string(env.Data) == "null" {
		return rec, env, nil
	}
	var out dashboardJSON
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("unmarshal dashboard: %v", err)
	}
	return rec, env, &out
}

// utcMonday returns the Monday of the current UTC week (00:00 UTC).
func utcMonday() time.Time {
	now := time.Now().UTC()
	offset := (int(now.Weekday()) + 6) % 7
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, -offset)
}

// expectedSlots returns the number of mon/wed/fri slots in [start..yesterday].
func expectedSlots(start, yesterday time.Time) int {
	n := 0
	for d := start; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
		switch d.Weekday() {
		case time.Monday, time.Wednesday, time.Friday:
			n++
		}
	}
	return n
}

// TestDashboardEmpty verifies the payload of a fresh user.
func TestDashboardEmpty(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	rec, _, d := dashboard(t, router, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if d.WorkoutsMonth != 0 || d.WorkoutsTotal != 0 || d.TonnageMonthKg != 0 {
		t.Fatalf("expected zero counters, got %+v", d)
	}
	if d.Streak != nil {
		t.Fatalf("expected nil streak without programs, got %+v", d.Streak)
	}
	if d.LastWorkout != nil {
		t.Fatalf("expected nil last workout, got %+v", d.LastWorkout)
	}
	if len(d.WeeklyActivity) != 12 {
		t.Fatalf("expected 12 weeks, got %d", len(d.WeeklyActivity))
	}
	monday := utcMonday()
	for i, w := range d.WeeklyActivity {
		want := monday.AddDate(0, 0, -7*(11-i)).Format("2006-01-02")
		if w.WeekStart != want || w.Workouts != 0 || w.SetsDone != 0 || w.TotalVolumeKg != 0 {
			t.Fatalf("week %d: expected %s all-zero, got %+v", i, want, w)
		}
	}
}

// TestDashboardCounters verifies month/total counters, tonnage, weekly
// activity buckets and the last workout summary.
func TestDashboardCounters(t *testing.T) {
	router, a := setup(t)
	bearer, userID := registerAndLogin(t, router, a)

	monday := utcMonday()
	upid, pdid := selectProgram(t, router, bearer, monday.AddDate(0, 0, -7).Format("2006-01-02"), "mon")

	heavy := []map[string]any{
		{"planned_reps": 10, "planned_weight_kg": 40.0, "reps": 10, "weight_kg": 40.0}, // 400
		{"planned_reps": 8, "planned_weight_kg": 45.0, "reps": 8, "weight_kg": 45.0},   // 360
		{"planned_reps": 8, "planned_weight_kg": 45.0, "reps": nil, "weight_kg": nil, "done": false},
	}
	// Two workouts in the current month, one in the previous month.
	insertWorkout(t, a, userID, upid, pdid, "День A", "completed", monday.AddDate(0, 0, -6), heavy) // 760
	insertWorkout(t, a, userID, upid, pdid, "День A", "completed", monday.Add(time.Hour), heavy)    // 760
	prevMonth := time.Now().UTC().AddDate(0, -1, 0)
	insertWorkout(t, a, userID, upid, pdid, "День B", "completed", prevMonth, heavy) // 760, вне месяца
	// Cancelled workout does not count anywhere.
	insertWorkout(t, a, userID, upid, pdid, "День A", "cancelled", monday.AddDate(0, 0, -3), heavy)

	rec, _, d := dashboard(t, router, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	monthStart := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	wantMonth := 0
	for _, day := range []time.Time{monday.AddDate(0, 0, -6), monday.Add(time.Hour), prevMonth} {
		if !day.Before(monthStart) {
			wantMonth++
		}
	}
	if d.WorkoutsMonth != wantMonth {
		t.Fatalf("workouts_month: expected %d, got %d", wantMonth, d.WorkoutsMonth)
	}
	if d.WorkoutsTotal != 3 {
		t.Fatalf("workouts_total: expected 3, got %d", d.WorkoutsTotal)
	}
	if d.TonnageMonthKg != 760*float64(wantMonth) {
		t.Fatalf("tonnage_month: expected %v, got %v", 760*float64(wantMonth), d.TonnageMonthKg)
	}

	if d.LastWorkout == nil {
		t.Fatalf("expected last workout, got nil")
	}
	wantLastDate := monday.Format("2006-01-02")
	if d.LastWorkout.Date != wantLastDate || d.LastWorkout.Name != "День A" ||
		d.LastWorkout.SetsDone != 2 || d.LastWorkout.ExercisesDone != 1 ||
		d.LastWorkout.TotalVolumeKg != 760 || d.LastWorkout.DurationSec <= 0 {
		t.Fatalf("unexpected last workout: %+v", d.LastWorkout)
	}

	var totalWorkouts, totalSets int
	var totalVolume float64
	for _, w := range d.WeeklyActivity {
		totalWorkouts += w.Workouts
		totalSets += w.SetsDone
		totalVolume += w.TotalVolumeKg
	}
	if totalWorkouts != 3 || totalSets != 6 || totalVolume != 760*3 {
		t.Fatalf("weekly totals: expected 3/6/%v, got %d/%d/%v", 760*3, totalWorkouts, totalSets, totalVolume)
	}
}

// TestStreak verifies the schedule-based streak: consecutive closed slots.
func TestStreak(t *testing.T) {
	router, a := setup(t)
	bearer, userID := registerAndLogin(t, router, a)

	monday := utcMonday()
	// Программа стартовала 4 недели назад, слоты mon/wed/fri.
	start := monday.AddDate(0, 0, -28)
	upid, pdid := selectProgram(t, router, bearer, start.Format("2006-01-02"))

	today := monday.AddDate(0, 0, daysSinceMondayUTC())
	yesterday := today.AddDate(0, 0, -1)
	if yesterday.Before(start) {
		t.Skip("test run too early in the week for a 4-week window")
	}

	all := expectedSlots(start, yesterday)
	set := []map[string]any{{"planned_reps": 10, "planned_weight_kg": 40.0, "reps": 10, "weight_kg": 40.0}}

	// Пропуск: среда прошлой недели (или ближайшая прошедшая wed/fri/mon,
	// если сегодня рано). Идём назад от вчера и ищем первую "wed".
	miss := yesterday
	for miss.Weekday() != time.Wednesday {
		miss = miss.AddDate(0, 0, -1)
	}

	closed := 0
	for d := start; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
		switch d.Weekday() {
		case time.Monday, time.Wednesday, time.Friday:
			if d.Equal(miss) {
				continue // намеренный пропуск
			}
			insertWorkout(t, a, userID, upid, pdid, "День A", "completed", d.Add(time.Hour), set)
			closed++
		}
	}

	rec, _, d := dashboard(t, router, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if d.Streak == nil {
		t.Fatalf("expected streak, got nil")
	}
	if d.Streak.Scheduled != all || d.Streak.Completed != closed {
		t.Fatalf("streak scheduled/completed: expected %d/%d, got %d/%d", all, closed, d.Streak.Scheduled, d.Streak.Completed)
	}
	// Серия = слоты после пропуска: от miss (wed) до yesterday — thu, fri (+mon.., если сегодня позже).
	wantCurrent := expectedSlots(miss.AddDate(0, 0, 1), yesterday)
	if d.Streak.Current != wantCurrent {
		t.Fatalf("streak current: expected %d, got %d", wantCurrent, d.Streak.Current)
	}
}

// TestStreakCancelledBreaks verifies a cancelled workout counts as a miss.
func TestStreakCancelledBreaks(t *testing.T) {
	router, a := setup(t)
	bearer, userID := registerAndLogin(t, router, a)

	monday := utcMonday()
	start := monday.AddDate(0, 0, -28)
	upid, pdid := selectProgram(t, router, bearer, start.Format("2006-01-02"))

	today := monday.AddDate(0, 0, daysSinceMondayUTC())
	yesterday := today.AddDate(0, 0, -1)
	if yesterday.Before(start) {
		t.Skip("test run too early in the week for a 4-week window")
	}

	// Пропуск — отменённая тренировка в среду прошлой недели.
	miss := yesterday
	for miss.Weekday() != time.Wednesday {
		miss = miss.AddDate(0, 0, -1)
	}

	set := []map[string]any{{"planned_reps": 10, "planned_weight_kg": 40.0, "reps": 10, "weight_kg": 40.0}}
	for d := start; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
		switch d.Weekday() {
		case time.Monday, time.Wednesday, time.Friday:
			status := "completed"
			if d.Equal(miss) {
				status = "cancelled"
			}
			insertWorkout(t, a, userID, upid, pdid, "День A", status, d.Add(time.Hour), set)
		}
	}

	rec, _, d := dashboard(t, router, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	wantCurrent := expectedSlots(miss.AddDate(0, 0, 1), yesterday)
	if d.Streak == nil || d.Streak.Current != wantCurrent {
		t.Fatalf("streak current: expected %d, got %+v", wantCurrent, d.Streak)
	}
}

// TestDashboardIsolatedPerUser verifies the stats are scoped to the JWT user.
func TestDashboardIsolatedPerUser(t *testing.T) {
	router, a := setup(t)
	bearer1, userID1 := registerAndLogin(t, router, a)
	bearer2, _ := registerAndLogin(t, router, a)

	monday := utcMonday()
	upid, pdid := selectProgram(t, router, bearer1, monday.AddDate(0, 0, -7).Format("2006-01-02"), "mon")
	insertWorkout(t, a, userID1, upid, pdid, "День A", "completed", monday.Add(time.Hour),
		[]map[string]any{{"planned_reps": 10, "planned_weight_kg": 40.0, "reps": 10, "weight_kg": 40.0}})

	rec, _, d := dashboard(t, router, bearer2)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", rec.Code)
	}
	if d.WorkoutsTotal != 0 || d.Streak != nil || d.LastWorkout != nil {
		t.Fatalf("user2 must see nothing, got %+v", d)
	}
}

// daysSinceMondayUTC returns 0 for Monday .. 6 for Sunday (UTC today).
func daysSinceMondayUTC() int {
	offset := (int(time.Now().UTC().Weekday()) + 6) % 7
	return offset
}
