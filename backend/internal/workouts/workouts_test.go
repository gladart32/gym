package workouts_test

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

const testPassword = "test-pass-123"

type apiEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type setJSON struct {
	ID                 uint32   `json:"id"`
	SetNumber          uint8    `json:"set_number"`
	PlannedReps        *uint16  `json:"planned_reps"`
	PlannedWeightKg    *float64 `json:"planned_weight_kg"`
	PlannedDurationSec *uint32  `json:"planned_duration_sec"`
	PlannedDistanceM   *uint32  `json:"planned_distance_m"`
	PlannedRestSec     *uint16  `json:"planned_rest_sec"`
	Reps               *uint16  `json:"reps"`
	WeightKg           *float64 `json:"weight_kg"`
	DurationSec        *uint32  `json:"duration_sec"`
	DistanceM          *uint32  `json:"distance_m"`
	CompletedAt        *string  `json:"completed_at"`
}

type exerciseJSON struct {
	ID                uint32    `json:"id"`
	ExerciseID        uint16    `json:"exercise_id"`
	ProgramExerciseID *uint32   `json:"program_exercise_id"`
	Replaced          bool      `json:"replaced"`
	Name              string    `json:"name"`
	Equipment         string    `json:"equipment"`
	TrackingType      string    `json:"tracking_type"`
	Position          uint8     `json:"position"`
	RestAfterSec      *uint16   `json:"rest_after_sec"`
	IsSkipped         bool      `json:"is_skipped"`
	ThumbnailURL      string    `json:"thumbnail_url"`
	GifURL            string    `json:"gif_url"`
	Sets              []setJSON `json:"sets"`
}

type sessionJSON struct {
	ID            uint32         `json:"id"`
	UserProgramID *uint32        `json:"user_program_id"`
	ProgramDayID  *uint32        `json:"program_day_id"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	StartedAt     string         `json:"started_at"`
	FinishedAt    *string        `json:"finished_at"`
	Exercises     []exerciseJSON `json:"exercises"`
}

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
		`DELETE FROM equipment`,
		`DELETE FROM muscles`,
		`DELETE FROM body_parts`,
	} {
		if _, err := a.DB.Exec(q); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
}

func do(t *testing.T, router *gin.Engine, method, path, bearer, acceptLang string, body any) (*httptest.ResponseRecorder, apiEnvelope) {
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
	if acceptLang != "" {
		req.Header.Set("Accept-Language", acceptLang)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var env apiEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec, env
}

// registerAndLogin creates a verified user and returns (access token, user id).
func registerAndLogin(t *testing.T, router *gin.Engine, a *app.App) (string, uint32) {
	t.Helper()
	email := fmt.Sprintf("wk%d@gym.test", time.Now().UnixNano())

	rec, _ := do(t, router, http.MethodPost, "/api/v1/auth/register", "", "", map[string]string{
		"email":    email,
		"password": testPassword,
	})
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

	rec, env := do(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
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

// seedCatalog inserts two exercises: bench press (weight_reps) and plank (time).
func seedCatalog(t *testing.T, a *app.App) (benchID, plankID uint32) {
	t.Helper()

	insertRef := func(table, code, nameEn, nameRu string) uint16 {
		t.Helper()
		if _, err := a.DB.Exec(fmt.Sprintf("INSERT INTO %s (code, name_en, name_ru) VALUES (?, ?, ?)", table), code, nameEn, nameRu); err != nil {
			t.Fatalf("seed %s: %v", table, err)
		}
		var id uint16
		if err := a.DB.Get(&id, fmt.Sprintf("SELECT id FROM %s WHERE code = ?", table), code); err != nil {
			t.Fatalf("seed %s id: %v", table, err)
		}
		return id
	}
	chestID := insertRef("body_parts", "chest", "Chest", "Грудь")
	waistID := insertRef("body_parts", "waist", "Waist", "Пресс")
	barbellID := insertRef("equipment", "barbell", "Barbell", "Штанга")
	bodyWeightID := insertRef("equipment", "body weight", "Body weight", "Собственный вес")
	pectoralsID := insertRef("muscles", "pectorals", "Pectorals", "Грудные")
	absID := insertRef("muscles", "abs", "Abs", "Пресс")

	insertExercise := func(extID string, bodyPartID, equipmentID, muscleID uint16, tracking string) uint32 {
		t.Helper()
		res, err := a.DB.Exec(`
			INSERT INTO exercises (source, external_id, body_part_id, equipment_id, target_muscle_id, tracking_type)
			VALUES ('manual', ?, ?, ?, ?, ?)`, extID, bodyPartID, equipmentID, muscleID, tracking)
		if err != nil {
			t.Fatalf("seed exercise: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("seed exercise id: %v", err)
		}
		return uint32(id)
	}
	benchID = insertExercise("0001", chestID, barbellID, pectoralsID, "weight_reps")
	plankID = insertExercise("0002", waistID, bodyWeightID, absID, "time")

	insertTranslation := func(exerciseID uint32, lang, name string) {
		t.Helper()
		if _, err := a.DB.Exec(`
			INSERT INTO exercise_translations (exercise_id, lang, name)
			VALUES (?, ?, ?)`, exerciseID, lang, name); err != nil {
			t.Fatalf("seed translation: %v", err)
		}
	}
	insertTranslation(benchID, "ru", "Жим штанги лёжа")
	insertTranslation(benchID, "en", "Barbell Bench Press")
	insertTranslation(plankID, "ru", "Планка")
	insertTranslation(plankID, "en", "Plank")

	if _, err := a.DB.Exec(`
		INSERT INTO exercise_media (exercise_id, type, path, width, height, source)
		VALUES (?, 'thumbnail', 'images/0001-t.jpg', 180, 180, 'gymvisual')`, benchID); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	return benchID, plankID
}

// startDay creates a personal program with one day, selects it for the user
// and returns (user_program_id, program_day_id).
func startDay(t *testing.T, router *gin.Engine, bearer string, benchID, plankID uint32) (uint32, uint32) {
	t.Helper()

	exercises := []map[string]any{
		{
			"exercise_id":    benchID,
			"rest_after_sec": 60,
			"sets": []map[string]any{
				{"set_number": 1, "reps": 12, "weight_kg": 40, "rest_sec": 90},
				{"set_number": 2, "reps": 10, "weight_kg": 45, "rest_sec": 90},
			},
		},
	}
	if plankID != 0 {
		exercises = append(exercises, map[string]any{
			"exercise_id": plankID,
			"sets": []map[string]any{
				{"set_number": 1, "duration_sec": 60, "rest_sec": 30},
			},
		})
	}
	body := map[string]any{
		"name": "Push Power",
		"days": []map[string]any{{
			"name":      "День A: грудь",
			"exercises": exercises,
		}},
	}
	rec, env := do(t, router, http.MethodPost, "/api/v1/programs", bearer, "", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create program: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var program struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &program); err != nil {
		t.Fatalf("unmarshal program: %v", err)
	}

	rec, env = do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/programs/%d/select", program.ID), bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("select program: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// the active user program (created by select) and the day id from the program detail
	rec, env = do(t, router, http.MethodGet, "/api/v1/schedule", bearer, "", nil)
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

	rec, env = do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/programs/%d", ups[0].ProgramID), bearer, "", nil)
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

func start(t *testing.T, router *gin.Engine, bearer string, userProgramID, programDayID uint32) (*httptest.ResponseRecorder, apiEnvelope, sessionJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodPost, "/api/v1/workouts/start", bearer, "",
		map[string]uint32{"user_program_id": userProgramID, "program_day_id": programDayID})
	var session sessionJSON
	if env.Data != nil {
		if err := json.Unmarshal(env.Data, &session); err != nil {
			t.Fatalf("unmarshal session: %v", err)
		}
	}
	return rec, env, session
}

func active(t *testing.T, router *gin.Engine, bearer string) (*httptest.ResponseRecorder, apiEnvelope, *sessionJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodGet, "/api/v1/workouts/active", bearer, "", nil)
	if env.Data == nil || string(env.Data) == "null" {
		return rec, env, nil
	}
	var session sessionJSON
	if err := json.Unmarshal(env.Data, &session); err != nil {
		t.Fatalf("unmarshal active session: %v", err)
	}
	return rec, env, &session
}

// TestActiveEmpty verifies there is no active workout right after registration.
func TestActiveEmpty(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	rec, _, session := active(t, router, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("active: expected 200, got %d", rec.Code)
	}
	if session != nil {
		t.Fatalf("active: expected null data, got %+v", session)
	}
}

// TestStartSnapshotAndFact covers the full happy path: start (plan snapshot),
// set fact, complete — after completion there is no active workout.
func TestStartSnapshotAndFact(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)
	benchID, _ := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer, benchID, 0)

	rec, env, s := start(t, router, bearer, upID, dayID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start: expected 201, got %d: %s", rec.Code, env.Message)
	}
	if s.Status != "in_progress" || s.Name != "День A: грудь" {
		t.Fatalf("start: unexpected session header: %+v", s)
	}
	if s.UserProgramID == nil || *s.UserProgramID != upID || s.ProgramDayID == nil || *s.ProgramDayID != dayID {
		t.Fatalf("start: unexpected session links: %+v", s)
	}
	if len(s.Exercises) != 1 {
		t.Fatalf("start: expected 1 exercise, got %d", len(s.Exercises))
	}

	bench := s.Exercises[0]
	if bench.Name != "Жим штанги лёжа" || bench.TrackingType != "weight_reps" || bench.Equipment != "Штанга" {
		t.Fatalf("start: unexpected exercise info: %+v", bench)
	}
	if bench.RestAfterSec == nil || *bench.RestAfterSec != 60 {
		t.Fatalf("start: rest_after_sec snapshot lost: %+v", bench.RestAfterSec)
	}
	if bench.ThumbnailURL != "/media/images/0001-t.jpg" {
		t.Fatalf("start: unexpected thumbnail url: %q", bench.ThumbnailURL)
	}
	if len(bench.Sets) != 2 {
		t.Fatalf("start: expected 2 sets, got %d", len(bench.Sets))
	}
	first := bench.Sets[0]
	if first.PlannedReps == nil || *first.PlannedReps != 12 ||
		first.PlannedWeightKg == nil || *first.PlannedWeightKg != 40 ||
		first.PlannedRestSec == nil || *first.PlannedRestSec != 90 {
		t.Fatalf("start: planned snapshot lost: %+v", first)
	}
	if first.Reps != nil || first.CompletedAt != nil {
		t.Fatalf("start: fact must be empty: %+v", first)
	}

	// recoverable after "reload"
	if _, _, sActive := active(t, router, bearer); sActive == nil {
		t.Fatalf("active after start: expected data")
	}

	// set the fact of the first set
	patchBody := map[string]any{"reps": 11, "weight_kg": 42.5, "completed": true}
	rec, env = do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/sets/%d", first.ID), bearer, "", patchBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch set: expected 200, got %d: %s", rec.Code, env.Message)
	}

	_, _, s2 := active(t, router, bearer)
	got := s2.Exercises[0].Sets[0]
	if got.Reps == nil || *got.Reps != 11 || got.WeightKg == nil || *got.WeightKg != 42.5 {
		t.Fatalf("patch set: fact not stored: %+v", got)
	}
	if got.CompletedAt == nil {
		t.Fatalf("patch set: completed_at not set")
	}
	if got.PlannedReps == nil || *got.PlannedReps != 12 {
		t.Fatalf("patch set: planned must stay 12: %+v", got)
	}

	// uncheck clears completed_at
	rec, env = do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/sets/%d", first.ID), bearer, "",
		map[string]any{"completed": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("uncheck set: expected 200, got %d: %s", rec.Code, env.Message)
	}
	_, _, s3 := active(t, router, bearer)
	if s3.Exercises[0].Sets[0].CompletedAt != nil {
		t.Fatalf("uncheck set: completed_at must be null")
	}

	// complete
	rec, env = do(t, router, http.MethodPost, "/api/v1/workouts/complete", bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var status struct {
		Status   string  `json:"status"`
		Finished *string `json:"finished_at"`
	}
	if err := a.DB.Get(&status.Status, `SELECT status FROM workouts WHERE id = ?`, s.ID); err != nil {
		t.Fatalf("select workout: %v", err)
	}
	if status.Status != "completed" {
		t.Fatalf("complete: expected status completed, got %s", status.Status)
	}
	if _, _, s4 := active(t, router, bearer); s4 != nil {
		t.Fatalf("active after complete: expected null")
	}
}

// TestStartConflict verifies only one active workout per user.
func TestStartConflict(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)
	benchID, _ := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer, benchID, 0)

	if rec, _, _ := start(t, router, bearer, upID, dayID); rec.Code != http.StatusCreated {
		t.Fatalf("first start: expected 201, got %d", rec.Code)
	}
	rec, env, _ := start(t, router, bearer, upID, dayID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second start: expected 409, got %d: %s", rec.Code, env.Message)
	}
}

// TestStructuralChanges covers skip, replace, add set and add exercise.
func TestStructuralChanges(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)
	benchID, plankID := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer, benchID, plankID)

	_, _, s := start(t, router, bearer, upID, dayID)
	bench, plank := s.Exercises[0], s.Exercises[1]

	// skip plank
	rec, env := do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/exercises/%d", plank.ID), bearer, "",
		map[string]any{"is_skipped": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("skip exercise: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var skipped exerciseJSON
	_ = json.Unmarshal(env.Data, &skipped)
	if !skipped.IsSkipped {
		t.Fatalf("skip exercise: is_skipped not stored")
	}

	// replace bench with plank in the running workout
	rec, env = do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/exercises/%d", bench.ID), bearer, "",
		map[string]any{"exercise_id": plankID})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace exercise: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var replaced exerciseJSON
	_ = json.Unmarshal(env.Data, &replaced)
	if !replaced.Replaced || replaced.ExerciseID != uint16(plankID) || replaced.Name != "Планка" {
		t.Fatalf("replace exercise: unexpected result: %+v", replaced)
	}
	if replaced.TrackingType != "time" {
		t.Fatalf("replace exercise: tracking_type must follow the new exercise")
	}

	// add a third set to the replaced exercise
	rec, env = do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/workouts/exercises/%d/sets", bench.ID), bearer, "", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add set: expected 201, got %d: %s", rec.Code, env.Message)
	}
	var addedSet setJSON
	_ = json.Unmarshal(env.Data, &addedSet)
	if addedSet.SetNumber != 3 {
		t.Fatalf("add set: expected set_number 3, got %d", addedSet.SetNumber)
	}

	// add an unplanned exercise with one empty set
	rec, env = do(t, router, http.MethodPost, "/api/v1/workouts/exercises", bearer, "",
		map[string]any{"exercise_id": benchID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add exercise: expected 201, got %d: %s", rec.Code, env.Message)
	}
	var added exerciseJSON
	_ = json.Unmarshal(env.Data, &added)
	if added.ExerciseID != uint16(benchID) || added.ProgramExerciseID != nil || added.Position != 3 {
		t.Fatalf("add exercise: unexpected result: %+v", added)
	}
	if len(added.Sets) != 1 || added.Sets[0].SetNumber != 1 {
		t.Fatalf("add exercise: expected one empty set: %+v", added.Sets)
	}
}

// TestCancel verifies the workout can be aborted.
func TestCancel(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)
	benchID, _ := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer, benchID, 0)

	_, _, s := start(t, router, bearer, upID, dayID)
	rec, env := do(t, router, http.MethodPost, "/api/v1/workouts/cancel", bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var status string
	if err := a.DB.Get(&status, `SELECT status FROM workouts WHERE id = ?`, s.ID); err != nil {
		t.Fatalf("select workout: %v", err)
	}
	if status != "cancelled" {
		t.Fatalf("cancel: expected status cancelled, got %s", status)
	}
	if _, _, s2 := active(t, router, bearer); s2 != nil {
		t.Fatalf("active after cancel: expected null")
	}
}

// TestOwnership verifies cross-user access is rejected.
func TestOwnership(t *testing.T) {
	router, a := setup(t)
	bearer1, _ := registerAndLogin(t, router, a)
	bearer2, _ := registerAndLogin(t, router, a)
	benchID, _ := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer1, benchID, 0)

	_, _, s := start(t, router, bearer1, upID, dayID)
	setID := s.Exercises[0].Sets[0].ID

	// another user cannot patch the set
	rec, env := do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/sets/%d", setID), bearer2, "",
		map[string]any{"completed": true})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign patch set: expected 404, got %d: %s", rec.Code, env.Message)
	}

	// another user cannot skip the exercise
	rec, _ = do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/exercises/%d", s.Exercises[0].ID), bearer2, "",
		map[string]any{"is_skipped": true})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign patch exercise: expected 404, got %d", rec.Code)
	}

	// another user cannot start a workout from a foreign program
	rec, _, _ = start(t, router, bearer2, upID, dayID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign start: expected 404, got %d", rec.Code)
	}
}

// TestInvalidFact verifies fact validation bounds.
func TestInvalidFact(t *testing.T) {
	router, a := setup(t)
	bearer, _ := registerAndLogin(t, router, a)
	benchID, _ := seedCatalog(t, a)
	upID, dayID := startDay(t, router, bearer, benchID, 0)

	_, _, s := start(t, router, bearer, upID, dayID)
	setID := s.Exercises[0].Sets[0].ID

	rec, _ := do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/sets/%d", setID), bearer, "",
		map[string]any{"weight_kg": 9999})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid weight: expected 400, got %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/workouts/sets/%d", setID), bearer, "",
		map[string]any{"reps": 100000})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid reps: expected 400, got %d", rec.Code)
	}
}
