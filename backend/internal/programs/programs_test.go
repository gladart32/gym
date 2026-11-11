package programs_test

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

type setJSON struct {
	ID          uint32   `json:"id"`
	SetNumber   uint8    `json:"set_number"`
	Reps        *uint16  `json:"reps"`
	WeightKg    *float64 `json:"weight_kg"`
	DurationSec *uint32  `json:"duration_sec"`
	DistanceM   *uint32  `json:"distance_m"`
	RestSec     *uint16  `json:"rest_sec"`
}

type exerciseJSON struct {
	ID           uint32    `json:"id"`
	ExerciseID   uint16    `json:"exercise_id"`
	Name         string    `json:"name"`
	Equipment    string    `json:"equipment"`
	TrackingType string    `json:"tracking_type"`
	Position     uint8     `json:"position"`
	RestAfterSec *uint16   `json:"rest_after_sec"`
	ThumbnailURL string    `json:"thumbnail_url"`
	GifURL       string    `json:"gif_url"`
	Sets         []setJSON `json:"sets"`
}

type dayJSON struct {
	ID        uint32         `json:"id"`
	Position  uint8          `json:"position"`
	Name      string         `json:"name"`
	Exercises []exerciseJSON `json:"exercises"`
}

type programJSON struct {
	ID           uint32    `json:"id"`
	OwnerID      *uint32   `json:"owner_id"`
	CopiedFromID *uint32   `json:"copied_from_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	AuthorName   *string   `json:"author_name"`
	Level        *string   `json:"level"`
	IsPublished  bool      `json:"is_published"`
	IsMine       bool      `json:"is_mine"`
	IsReady      bool      `json:"is_ready"`
	CanEdit      bool      `json:"can_edit"`
	Days         []dayJSON `json:"days"`
}

type programItemJSON struct {
	ID             uint32  `json:"id"`
	Name           string  `json:"name"`
	Level          *string `json:"level"`
	IsPublished    bool    `json:"is_published"`
	DaysCount      int     `json:"days_count"`
	ExercisesCount int     `json:"exercises_count"`
	IsMine         bool    `json:"is_mine"`
	IsReady        bool    `json:"is_ready"`
}

type programListJSON struct {
	Items   []programItemJSON `json:"items"`
	Total   int64             `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Pages   int               `json:"pages"`
}

type fixture struct {
	benchID, plankID uint32
}

const testPassword = "test-pass-123"

func setup(t *testing.T) (*gin.Engine, *app.App, fixture) {
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
	)

	clean(t, a)
	fx := seed(t, a)
	return router, a, fx
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
		`DELETE FROM exercise_favorites`,
		`DELETE FROM exercise_media`,
		`DELETE FROM exercise_translations`,
		`DELETE FROM exercise_secondary_muscles`,
		`DELETE FROM exercises`,
		`DELETE FROM user_equipment`,
		`DELETE FROM body_parts`,
		`DELETE FROM equipment`,
		`DELETE FROM muscles`,
	} {
		if _, err := a.DB.Exec(q); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
}

func seed(t *testing.T, a *app.App) fixture {
	t.Helper()

	chestID := insertRef(t, a, "body_parts", "chest", "Chest", "Грудь")
	waistID := insertRef(t, a, "body_parts", "waist", "Waist", "Пресс")
	barbellID := insertRef(t, a, "equipment", "barbell", "Barbell", "Штанга")
	bodyWeightID := insertRef(t, a, "equipment", "body weight", "Body weight", "Собственный вес")
	pectoralsID := insertRef(t, a, "muscles", "pectorals", "Pectorals", "Грудные")
	absID := insertRef(t, a, "muscles", "abs", "Abs", "Пресс")

	benchID := insertExercise(t, a, "0001", chestID, barbellID, pectoralsID, "weight_reps")
	plankID := insertExercise(t, a, "0002", waistID, bodyWeightID, absID, "time")

	insertTranslation(t, a, benchID, "ru", "Жим штанги лёжа")
	insertTranslation(t, a, benchID, "en", "Barbell Bench Press")
	insertTranslation(t, a, plankID, "ru", "Планка")
	insertTranslation(t, a, plankID, "en", "Plank")

	insertMedia(t, a, benchID, "thumbnail", "images/0001-t.jpg")

	return fixture{benchID: benchID, plankID: plankID}
}

func insertRef(t *testing.T, a *app.App, table, code, nameEn, nameRu string) uint16 {
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

func insertExercise(t *testing.T, a *app.App, extID string, bodyPartID, equipmentID, muscleID uint16, tracking string) uint32 {
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

func insertTranslation(t *testing.T, a *app.App, exerciseID uint32, lang, name string) {
	t.Helper()
	if _, err := a.DB.Exec(`
		INSERT INTO exercise_translations (exercise_id, lang, name)
		VALUES (?, ?, ?)`, exerciseID, lang, name); err != nil {
		t.Fatalf("seed translation: %v", err)
	}
}

func insertMedia(t *testing.T, a *app.App, exerciseID uint32, typ, path string) {
	t.Helper()
	if _, err := a.DB.Exec(`
		INSERT INTO exercise_media (exercise_id, type, path, width, height, source)
		VALUES (?, ?, ?, 180, 180, 'gymvisual')`, exerciseID, typ, path); err != nil {
		t.Fatalf("seed media: %v", err)
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

// doCode issues a request and returns only the HTTP status code.
func doCode(t *testing.T, router *gin.Engine, method, path, bearer, acceptLang string, body any) int {
	t.Helper()
	rec, _ := do(t, router, method, path, bearer, acceptLang, body)
	return rec.Code
}

// registerAndLogin creates a verified user and returns (access token, user id).
func registerAndLogin(t *testing.T, router *gin.Engine, a *app.App) (string, uint32) {
	t.Helper()
	email := fmt.Sprintf("pg%d@gym.test", time.Now().UnixNano())
	return registerLoginEmail(t, router, a, email)
}

// promoteAndLogin registers a user, grants the admin role and logs in
// (the role is embedded into the access token at login time).
func promoteAndLogin(t *testing.T, router *gin.Engine, a *app.App) (string, uint32) {
	t.Helper()
	email := fmt.Sprintf("ad%d@gym.test", time.Now().UnixNano())
	_, userID := registerLoginEmail(t, router, a, email)
	if _, err := a.DB.Exec(`UPDATE users SET role = 'admin' WHERE id = ?`, userID); err != nil {
		t.Fatalf("promote to admin: %v", err)
	}
	rec, env := do(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email":    email,
		"password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(env.Data, &tokens); err != nil {
		t.Fatalf("unmarshal tokens: %v", err)
	}
	return tokens.AccessToken, userID
}

func registerLoginEmail(t *testing.T, router *gin.Engine, a *app.App, email string) (string, uint32) {
	t.Helper()

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
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(env.Data, &tokens); err != nil {
		t.Fatalf("unmarshal tokens: %v", err)
	}
	return tokens.AccessToken, userID
}

// benchDay is a reusable day input with the seeded bench press.
func benchDay(exerciseID uint32) map[string]any {
	return map[string]any{
		"name": "День A: грудь",
		"exercises": []map[string]any{
			{
				"exercise_id":    exerciseID,
				"rest_after_sec": 60,
				"sets": []map[string]any{
					{"set_number": 1, "reps": 12, "weight_kg": 40, "rest_sec": 90},
					{"set_number": 2, "reps": 10, "weight_kg": 45, "rest_sec": 90},
					{"set_number": 3, "reps": 8, "weight_kg": 50, "rest_sec": 120},
				},
			},
		},
	}
}

func createProgram(t *testing.T, router *gin.Engine, bearer string, body map[string]any) (int, apiEnvelope, programJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodPost, "/api/v1/programs", bearer, "", body)
	var out programJSON
	if env.Data != nil {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			t.Fatalf("unmarshal created program: %v", err)
		}
	}
	return rec.Code, env, out
}

func getDetail(t *testing.T, router *gin.Engine, bearer string, id uint32, acceptLang string) (int, apiEnvelope, programJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/programs/%d", id), bearer, acceptLang, nil)
	var out programJSON
	if env.Data != nil {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			t.Fatalf("unmarshal detail: %v", err)
		}
	}
	return rec.Code, env, out
}

func TestProgramsRequireAuth(t *testing.T) {
	router, _, _ := setup(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/programs"},
		{http.MethodGet, "/api/v1/programs/1"},
		{http.MethodPost, "/api/v1/programs"},
		{http.MethodPut, "/api/v1/programs/1"},
		{http.MethodDelete, "/api/v1/programs/1"},
		{http.MethodPost, "/api/v1/programs/1/copy"},
	} {
		rec, _ := do(t, router, tc.method, tc.path, "", "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestCreatePersonalProgramAndDetail(t *testing.T) {
	router, a, fx := setup(t)
	bearer, userID := registerAndLogin(t, router, a)

	body := map[string]any{
		"name":        "Моя программа",
		"description": "Три дня в неделю",
		"level":       "beginner",
		"days":        []map[string]any{benchDay(fx.benchID)},
	}
	code, env, out := createProgram(t, router, bearer, body)
	if code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", code, env.Message)
	}
	if out.ID == 0 || !out.IsMine || out.IsReady || !out.CanEdit || out.IsPublished {
		t.Fatalf("unexpected created program: %+v", out)
	}
	if out.OwnerID == nil || *out.OwnerID != userID {
		t.Fatalf("unexpected owner: %+v", out.OwnerID)
	}
	if len(out.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(out.Days))
	}
	day := out.Days[0]
	if day.Name != "День A: грудь" || day.Position != 1 {
		t.Fatalf("unexpected day: %+v", day)
	}
	if len(day.Exercises) != 1 || day.Exercises[0].Name != "Жим штанги лёжа" {
		t.Fatalf("unexpected exercises: %+v", day.Exercises)
	}
	ex := day.Exercises[0]
	if ex.ExerciseID != uint16(fx.benchID) || ex.TrackingType != "weight_reps" ||
		ex.Equipment != "Штанга" || ex.RestAfterSec == nil || *ex.RestAfterSec != 60 ||
		ex.ThumbnailURL != "/media/images/0001-t.jpg" {
		t.Fatalf("unexpected exercise row: %+v", ex)
	}
	if len(ex.Sets) != 3 || ex.Sets[0].Reps == nil || *ex.Sets[0].Reps != 12 ||
		ex.Sets[2].WeightKg == nil || *ex.Sets[2].WeightKg != 50 {
		t.Fatalf("unexpected sets: %+v", ex.Sets)
	}

	// Re-read via GET: the same tree.
	code, _, detail := getDetail(t, router, bearer, out.ID, "")
	if code != http.StatusOK || detail.ID != out.ID || len(detail.Days) != 1 {
		t.Fatalf("detail mismatch: code=%d detail=%+v", code, detail)
	}

	// English locale.
	code, _, en := getDetail(t, router, bearer, out.ID, "en-US,en;q=0.9")
	if code != http.StatusOK || en.Days[0].Exercises[0].Name != "Barbell Bench Press" ||
		en.Days[0].Exercises[0].Equipment != "Barbell" {
		t.Fatalf("unexpected en detail: %+v", en.Days)
	}

	// List: exactly one own program with correct counters.
	rec, env := do(t, router, http.MethodGet, "/api/v1/programs", bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}
	var list programListJSON
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("unexpected list: %+v", list)
	}
	item := list.Items[0]
	if item.ID != out.ID || !item.IsMine || item.IsReady ||
		item.DaysCount != 1 || item.ExercisesCount != 1 {
		t.Fatalf("unexpected list item: %+v", item)
	}
}

// Regression: Detail used a global exercise index against per-day slices,
// panicking ("index out of range") on programs with more than one day.
func TestCreateMultiDayProgramDetail(t *testing.T) {
	router, a, fx := setup(t)
	bearer, userID := registerAndLogin(t, router, a)

	body := map[string]any{
		"name":  "Домашняя",
		"level": "beginner",
		"days": []map[string]any{
			{
				"name": "Понедельник",
				"exercises": []map[string]any{
					{
						"exercise_id":    fx.benchID,
						"rest_after_sec": 60,
						"sets": []map[string]any{
							{"set_number": 1, "reps": 12, "weight_kg": 40},
							{"set_number": 2, "reps": 10, "weight_kg": 45},
						},
					},
					{
						"exercise_id": fx.plankID,
						"sets": []map[string]any{
							{"set_number": 1, "duration_sec": 60},
						},
					},
				},
			},
			{
				"name": "Среда",
				"exercises": []map[string]any{
					{
						"exercise_id": fx.plankID,
						"sets": []map[string]any{
							{"set_number": 1, "duration_sec": 45},
							{"set_number": 2, "duration_sec": 60},
						},
					},
				},
			},
		},
	}

	code, env, out := createProgram(t, router, bearer, body)
	if code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", code, env.Message)
	}
	if out.OwnerID == nil || *out.OwnerID != userID || !out.IsMine {
		t.Fatalf("unexpected created program: %+v", out)
	}

	assertMultiDayTree := func(tag string, p programJSON) {
		t.Helper()
		if len(p.Days) != 2 {
			t.Fatalf("%s: expected 2 days, got %d", tag, len(p.Days))
		}
		day1, day2 := p.Days[0], p.Days[1]
		if day1.Name != "Понедельник" || day2.Name != "Среда" {
			t.Fatalf("%s: unexpected day names: %q, %q", tag, day1.Name, day2.Name)
		}
		if len(day1.Exercises) != 2 || len(day2.Exercises) != 1 {
			t.Fatalf("%s: unexpected exercise counts: %+v", tag, p.Days)
		}
		if day1.Exercises[0].ExerciseID != uint16(fx.benchID) ||
			day1.Exercises[1].ExerciseID != uint16(fx.plankID) ||
			day2.Exercises[0].ExerciseID != uint16(fx.plankID) {
			t.Fatalf("%s: exercises in wrong days: %+v", tag, p.Days)
		}
		if len(day1.Exercises[0].Sets) != 2 ||
			len(day1.Exercises[1].Sets) != 1 ||
			len(day2.Exercises[0].Sets) != 2 {
			t.Fatalf("%s: sets attached to wrong exercises: %+v", tag, p.Days)
		}
		if *day1.Exercises[0].Sets[1].WeightKg != 45 ||
			*day2.Exercises[0].Sets[1].DurationSec != 60 {
			t.Fatalf("%s: unexpected set values: %+v", tag, p.Days)
		}
	}

	assertMultiDayTree("create", out)

	// Re-read via GET: sets stay attached to the right day/exercise.
	code, _, detail := getDetail(t, router, bearer, out.ID, "")
	if code != http.StatusOK {
		t.Fatalf("detail: expected 200, got %d", code)
	}
	assertMultiDayTree("detail", detail)
}

func TestListVisibilityAndScopes(t *testing.T) {
	router, a, fx := setup(t)
	user1, _ := registerAndLogin(t, router, a)
	user2, _ := registerAndLogin(t, router, a)
	admin, _ := promoteAndLogin(t, router, a)

	// user1 personal program.
	code, _, personal := createProgram(t, router, user1, map[string]any{
		"name": "Личная база",
		"days": []map[string]any{benchDay(fx.benchID)},
	})
	if code != http.StatusOK {
		t.Fatalf("create personal: %d", code)
	}

	// admin ready published program.
	code, _, ready := createProgram(t, router, admin, map[string]any{
		"name":         "Старт Gym",
		"level":        "intermediate",
		"is_published": true,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	if code != http.StatusOK {
		t.Fatalf("create ready: %d", code)
	}
	if !ready.IsReady || !ready.IsPublished || ready.IsMine || ready.OwnerID != nil {
		t.Fatalf("unexpected ready program: %+v", ready)
	}

	// user2 sees only the ready program.
	rec, env := do(t, router, http.MethodGet, "/api/v1/programs", user2, "", nil)
	var list programListJSON
	_ = json.Unmarshal(env.Data, &list)
	if rec.Code != http.StatusOK || list.Total != 1 || list.Items[0].ID != ready.ID {
		t.Fatalf("user2 list: code=%d list=%+v", rec.Code, list)
	}
	if _, _, d := getDetail(t, router, user2, personal.ID, ""); d.ID != 0 {
		t.Fatalf("user2 must not see foreign personal program")
	}

	// Scopes.
	rec, env = do(t, router, http.MethodGet, "/api/v1/programs?scope=mine", user1, "", nil)
	_ = json.Unmarshal(env.Data, &list)
	if list.Total != 1 || list.Items[0].ID != personal.ID {
		t.Fatalf("scope=mine: %+v", list)
	}

	rec, env = do(t, router, http.MethodGet, "/api/v1/programs?scope=ready", user1, "", nil)
	_ = json.Unmarshal(env.Data, &list)
	if list.Total != 1 || list.Items[0].ID != ready.ID || list.Items[0].IsMine {
		t.Fatalf("scope=ready: %+v", list)
	}

	// Search by name and level filter.
	rec, env = do(t, router, http.MethodGet, "/api/v1/programs?q=%D0%9B%D0%B8%D1%87%D0%BD%D0%B0%D1%8F", user1, "", nil)
	_ = json.Unmarshal(env.Data, &list)
	if list.Total != 1 || list.Items[0].ID != personal.ID {
		t.Fatalf("q filter: %+v", list)
	}

	rec, env = do(t, router, http.MethodGet, "/api/v1/programs?level=intermediate", user2, "", nil)
	_ = json.Unmarshal(env.Data, &list)
	if list.Total != 1 || list.Items[0].ID != ready.ID {
		t.Fatalf("level filter: %+v", list)
	}

	// Invalid params.
	for _, q := range []string{"?scope=bogus", "?level=bogus", "?sort=bogus", "?page=0", "?per_page=0"} {
		rec, _ = do(t, router, http.MethodGet, "/api/v1/programs"+q, user1, "", nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", q, rec.Code)
		}
	}

	// Draft (unpublished) ready program is hidden from users, visible to admin.
	rec, _ = do(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", ready.ID), admin, "", map[string]any{
		"name":         "Старт Gym",
		"level":        "intermediate",
		"is_published": false,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("unpublish: %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/programs/%d", ready.ID), user2, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden draft detail: expected 404, got %d", rec.Code)
	}
	rec, env = do(t, router, http.MethodGet, "/api/v1/programs?scope=ready", admin, "", nil)
	_ = json.Unmarshal(env.Data, &list)
	if rec.Code != http.StatusOK || list.Total != 1 || list.Items[0].ID != ready.ID {
		t.Fatalf("admin must still see the draft: code=%d list=%+v", rec.Code, list)
	}
}

func TestUpdatePermissionsAndStructureReplacement(t *testing.T) {
	router, a, fx := setup(t)
	user1, _ := registerAndLogin(t, router, a)
	user2, _ := registerAndLogin(t, router, a)
	admin, _ := promoteAndLogin(t, router, a)

	code, _, own := createProgram(t, router, user1, map[string]any{
		"name": "До замены",
		"days": []map[string]any{
			benchDay(fx.benchID),
			{"name": "День B: пусто", "exercises": []map[string]any{}},
		},
	})
	if code != http.StatusOK {
		t.Fatalf("create: %d", code)
	}

	// Foreign user cannot update.
	code = doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", own.ID), user2, "", map[string]any{
		"name": "Взлом",
		"days": []map[string]any{},
	})
	if code != http.StatusForbidden {
		t.Fatalf("foreign update: expected 403, got %d", code)
	}

	// Owner replaces the whole structure.
	code = doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", own.ID), user1, "", map[string]any{
		"name":  "После замены",
		"level": "advanced",
		"days": []map[string]any{
			{
				"name": "День A: грудь и планка",
				"exercises": []map[string]any{
					{"exercise_id": fx.benchID, "sets": []map[string]any{{"set_number": 1, "reps": 5, "weight_kg": 60}}},
					{"exercise_id": fx.plankID, "rest_after_sec": 30, "sets": []map[string]any{
						{"set_number": 1, "duration_sec": 60},
						{"set_number": 2, "duration_sec": 45},
					}},
				},
			},
		},
	})
	if code != http.StatusOK {
		t.Fatalf("update: %d", code)
	}
	_, _, updated := getDetail(t, router, user1, own.ID, "")
	if updated.Name != "После замены" || updated.Level == nil || *updated.Level != "advanced" {
		t.Fatalf("unexpected updated header: %+v", updated)
	}
	if len(updated.Days) != 1 || len(updated.Days[0].Exercises) != 2 {
		t.Fatalf("structure was not replaced: %+v", updated.Days)
	}
	bench := updated.Days[0].Exercises[0]
	if bench.Sets[0].Reps == nil || *bench.Sets[0].Reps != 5 {
		t.Fatalf("unexpected bench set: %+v", bench)
	}
	plank := updated.Days[0].Exercises[1]
	if plank.TrackingType != "time" || len(plank.Sets) != 2 || plank.Sets[1].DurationSec == nil || *plank.Sets[1].DurationSec != 45 {
		t.Fatalf("unexpected plank sets: %+v", plank)
	}

	// Ready program: users cannot update, admin can.
	code, _, ready := createProgram(t, router, admin, map[string]any{
		"name":         "Готовая",
		"is_published": true,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	if code != http.StatusOK {
		t.Fatalf("create ready: %d", code)
	}
	code = doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", ready.ID), user1, "", map[string]any{
		"name": "Взлом",
		"days": []map[string]any{},
	})
	if code != http.StatusForbidden {
		t.Fatalf("user update ready: expected 403, got %d", code)
	}
	code = doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", ready.ID), admin, "", map[string]any{
		"name":         "Готовая v2",
		"is_published": true,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	if code != http.StatusOK {
		t.Fatalf("admin update ready: %d", code)
	}
	_, _, readyV2 := getDetail(t, router, admin, ready.ID, "")
	if readyV2.Name != "Готовая v2" || !readyV2.IsReady {
		t.Fatalf("unexpected ready after update: %+v", readyV2)
	}
}

func TestDeletePermissions(t *testing.T) {
	router, a, fx := setup(t)
	user1, _ := registerAndLogin(t, router, a)
	user2, _ := registerAndLogin(t, router, a)
	admin, _ := promoteAndLogin(t, router, a)

	code, _, personal := createProgram(t, router, user1, map[string]any{
		"name": "Личная",
		"days": []map[string]any{benchDay(fx.benchID)},
	})
	code, _, ready := createProgram(t, router, admin, map[string]any{
		"name":         "Готовая",
		"is_published": true,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})

	// Foreign user cannot delete anything.
	for _, id := range []uint32{personal.ID, ready.ID} {
		code = doCode(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/programs/%d", id), user2, "", nil)
		if code != http.StatusForbidden {
			t.Fatalf("foreign delete %d: expected 403, got %d", id, code)
		}
	}

	// Owner deletes the personal program.
	code = doCode(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/programs/%d", personal.ID), user1, "", nil)
	if code != http.StatusOK {
		t.Fatalf("delete personal: %d", code)
	}
	if c, _, _ := getDetail(t, router, user1, personal.ID, ""); c != http.StatusNotFound {
		t.Fatalf("deleted personal detail: expected 404, got %d", c)
	}

	// Only admin deletes the ready program.
	code = doCode(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/programs/%d", ready.ID), admin, "", nil)
	if code != http.StatusOK {
		t.Fatalf("delete ready: %d", code)
	}
	if c, _, _ := getDetail(t, router, user2, ready.ID, ""); c != http.StatusNotFound {
		t.Fatalf("deleted ready detail: expected 404, got %d", c)
	}

	// Unknown id -> 404.
	code = doCode(t, router, http.MethodDelete, "/api/v1/programs/4294967295", admin, "", nil)
	if code != http.StatusNotFound {
		t.Fatalf("delete unknown: expected 404, got %d", code)
	}
}

func TestCopyProgram(t *testing.T) {
	router, a, fx := setup(t)
	user1, _ := registerAndLogin(t, router, a)
	admin, _ := promoteAndLogin(t, router, a)

	code, _, ready := createProgram(t, router, admin, map[string]any{
		"name":         "Оригинал",
		"description":  "Описание",
		"level":        "beginner",
		"is_published": true,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	if code != http.StatusOK {
		t.Fatalf("create ready: %d", code)
	}

	// Copy as a personal program.
	rec, env := do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/programs/%d/copy", ready.ID), user1, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var copy programJSON
	if err := json.Unmarshal(env.Data, &copy); err != nil {
		t.Fatalf("unmarshal copy: %v", err)
	}
	if copy.ID == ready.ID || !copy.IsMine || copy.IsReady || !copy.CanEdit || copy.IsPublished {
		t.Fatalf("unexpected copy flags: %+v", copy)
	}
	if copy.CopiedFromID == nil || *copy.CopiedFromID != ready.ID {
		t.Fatalf("copied_from_id not set: %+v", copy.CopiedFromID)
	}
	if copy.Name != "Оригинал" || len(copy.Days) != 1 || len(copy.Days[0].Exercises) != 1 || len(copy.Days[0].Exercises[0].Sets) != 3 {
		t.Fatalf("copy structure mismatch: %+v", copy)
	}

	// The source stays intact and visible.
	_, _, src := getDetail(t, router, user1, ready.ID, "")
	if src.IsPublished != true || len(src.Days) != 1 {
		t.Fatalf("source changed by copy: %+v", src)
	}

	// Copying a hidden program -> 404. First make it a hidden draft.
	_, _ = do(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", ready.ID), admin, "", map[string]any{
		"name":         "Оригинал",
		"is_published": false,
		"days":         []map[string]any{benchDay(fx.benchID)},
	})
	rec, _ = do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/programs/%d/copy", ready.ID), user1, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("copy hidden: expected 404, got %d", rec.Code)
	}
}

func TestCreateValidation(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"empty name", map[string]any{"name": "   ", "days": []map[string]any{}}},
		{"long name", map[string]any{"name": string(make([]rune, 300)), "days": []map[string]any{}}},
		{"bad level", map[string]any{"name": "X", "level": "elite", "days": []map[string]any{}}},
		{"empty day name", map[string]any{"name": "X", "days": []map[string]any{{"name": "  "}}}},
		{"set_number 0", map[string]any{"name": "X", "days": []map[string]any{
			{"name": "D", "exercises": []map[string]any{
				{"exercise_id": fx.benchID, "sets": []map[string]any{{"set_number": 0}}},
			}},
		}}},
		{"duplicate set_number", map[string]any{"name": "X", "days": []map[string]any{
			{"name": "D", "exercises": []map[string]any{
				{"exercise_id": fx.benchID, "sets": []map[string]any{
					{"set_number": 1}, {"set_number": 1},
				}},
			}},
		}}},
		{"unknown exercise", map[string]any{"name": "X", "days": []map[string]any{
			{"name": "D", "exercises": []map[string]any{
				{"exercise_id": 99999, "sets": []map[string]any{{"set_number": 1}}},
			}},
		}}},
		{"bad weight", map[string]any{"name": "X", "days": []map[string]any{
			{"name": "D", "exercises": []map[string]any{
				{"exercise_id": fx.benchID, "sets": []map[string]any{{"set_number": 1, "weight_kg": 10000}}},
			}},
		}}},
	}

	for _, tc := range cases {
		code, env, _ := createProgram(t, router, bearer, tc.body)
		if code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d (%s)", tc.name, code, env.Message)
		}
	}

	// Malformed JSON body.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/programs", bytes.NewReader([]byte("{broken")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("broken json: expected 400, got %d", rec.Code)
	}

	// Regular user cannot publish.
	code, _, out := createProgram(t, router, bearer, map[string]any{
		"name":         "Не готовая",
		"is_published": true,
		"days":         []map[string]any{},
	})
	if code != http.StatusOK {
		t.Fatalf("user publish attempt: %d", code)
	}
	if out.IsPublished || out.IsReady || out.OwnerID == nil {
		t.Fatalf("user forced publish: %+v", out)
	}
}

func TestNegativeWeightAllowed(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	body := map[string]any{"name": "Помощь", "days": []map[string]any{
		{"name": "D", "exercises": []map[string]any{
			{"exercise_id": fx.benchID, "sets": []map[string]any{{"set_number": 1, "weight_kg": -30}}},
		}},
	}}
	code, _, out := createProgram(t, router, bearer, body)
	if code != http.StatusOK {
		t.Fatalf("negative weight: expected 200, got %d", code)
	}
	w := out.Days[0].Exercises[0].Sets[0].WeightKg
	if w == nil || *w != -30 {
		t.Fatalf("unexpected weight: %+v", w)
	}
}
