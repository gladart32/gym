package exercises_test

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

type refJSON struct {
	ID   uint16 `json:"id"`
	Name string `json:"name"`
}

type mediaJSON struct {
	GifURL       string  `json:"gif_url"`
	ThumbnailURL string  `json:"thumbnail_url"`
	Attribution  *string `json:"attribution"`
}

type itemJSON struct {
	ID           uint16    `json:"id"`
	Name         string    `json:"name"`
	BodyPart     refJSON   `json:"body_part"`
	Equipment    refJSON   `json:"equipment"`
	TargetMuscle refJSON   `json:"target_muscle"`
	TrackingType string    `json:"tracking_type"`
	IsFavorite   bool      `json:"is_favorite"`
	Media        mediaJSON `json:"media"`
}

type listJSON struct {
	Items   []itemJSON `json:"items"`
	Total   int64      `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"per_page"`
	Pages   int        `json:"pages"`
}

type detailJSON struct {
	ID               uint16      `json:"id"`
	Name             string      `json:"name"`
	Instructions     *string     `json:"instructions"`
	Steps            []string    `json:"steps"`
	BodyPart         refJSON     `json:"body_part"`
	Equipment        refJSON     `json:"equipment"`
	TargetMuscle     refJSON     `json:"target_muscle"`
	SecondaryMuscles []refJSON   `json:"secondary_muscles"`
	TrackingType     string      `json:"tracking_type"`
	IsFavorite       bool        `json:"is_favorite"`
	Media            []mediaItem `json:"media"`
}

type mediaItem struct {
	Type        string  `json:"type"`
	URL         string  `json:"url"`
	Width       *uint16 `json:"width"`
	Height      *uint16 `json:"height"`
	Attribution *string `json:"attribution"`
}

type filtersJSON struct {
	BodyParts     []refJSON `json:"body_parts"`
	Equipment     []refJSON `json:"equipment"`
	Muscles       []refJSON `json:"muscles"`
	TrackingTypes []struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	} `json:"tracking_types"`
}

type fixture struct {
	chestID, waistID           uint16
	barbellID, bodyWeightID    uint16
	pectoralsID, absID         uint16
	benchID, pushupID, plankID uint32
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

	cleanCatalog(t, a)
	fx := seed(t, a)
	return router, a, fx
}

func cleanCatalog(t *testing.T, a *app.App) {
	t.Helper()
	for _, q := range []string{
		// program/workout tables reference exercises — cleared first.
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
			t.Fatalf("clean catalog: %v", err)
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
	pushupID := insertExercise(t, a, "0002", chestID, bodyWeightID, pectoralsID, "reps")
	plankID := insertExercise(t, a, "0003", waistID, bodyWeightID, absID, "time")

	instructions := "Лягте на скамью и жмите штангу вверх."
	steps := `["Лягте на скамью","Снимите штангу","Опустите на грудь","Выжмите вверх"]`
	insertTranslation(t, a, benchID, "ru", "Жим штанги лёжа", &instructions, &steps)
	insertTranslation(t, a, benchID, "en", "Barbell Bench Press", nil, nil)
	insertTranslation(t, a, pushupID, "ru", "Отжимания", nil, nil)
	insertTranslation(t, a, pushupID, "en", "Push-up", nil, nil)
	insertTranslation(t, a, plankID, "ru", "Планка", nil, nil)
	insertTranslation(t, a, plankID, "en", "Plank", nil, nil)

	insertMedia(t, a, benchID, "gif", "videos/0001-t.gif")
	insertMedia(t, a, benchID, "thumbnail", "images/0001-t.jpg")
	insertMedia(t, a, pushupID, "gif", "videos/0002-t.gif")
	insertMedia(t, a, pushupID, "thumbnail", "images/0002-t.jpg")
	insertMedia(t, a, plankID, "gif", "videos/0003-t.gif")

	if _, err := a.DB.Exec(`INSERT INTO exercise_secondary_muscles (exercise_id, muscle_id) VALUES (?, ?)`, benchID, absID); err != nil {
		t.Fatalf("seed secondary muscle: %v", err)
	}

	return fixture{
		chestID: chestID, waistID: waistID,
		barbellID: barbellID, bodyWeightID: bodyWeightID,
		pectoralsID: pectoralsID, absID: absID,
		benchID: benchID, pushupID: pushupID, plankID: plankID,
	}
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

func insertTranslation(t *testing.T, a *app.App, exerciseID uint32, lang, name string, instructions, steps *string) {
	t.Helper()
	if _, err := a.DB.Exec(`
		INSERT INTO exercise_translations (exercise_id, lang, name, instructions, steps)
		VALUES (?, ?, ?, ?, CAST(NULLIF(?, '') AS JSON))`, exerciseID, lang, name, instructions, steps); err != nil {
		t.Fatalf("seed translation: %v", err)
	}
}

func insertMedia(t *testing.T, a *app.App, exerciseID uint32, typ, path string) {
	t.Helper()
	attrib := "© Gym visual — https://gymvisual.com/"
	if _, err := a.DB.Exec(`
		INSERT INTO exercise_media (exercise_id, type, path, width, height, source, attribution)
		VALUES (?, ?, ?, 180, 180, 'gymvisual', ?)`, exerciseID, typ, path, attrib); err != nil {
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

func registerAndLogin(t *testing.T, router *gin.Engine, a *app.App) (string, uint32) {
	t.Helper()

	email := fmt.Sprintf("ex%d@gym.test", time.Now().UnixNano())
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
	token, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenEmailVerify)
	if err != nil {
		t.Fatalf("create email token: %v", err)
	}
	if err := a.Auth.VerifyEmail(ctx, token); err != nil {
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

func getList(t *testing.T, router *gin.Engine, bearer, query, acceptLang string) listJSON {
	t.Helper()
	rec, env := do(t, router, http.MethodGet, "/api/v1/exercises"+query, bearer, acceptLang, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %s: expected 200, got %d: %s", query, rec.Code, rec.Body.String())
	}
	var out listJSON
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	return out
}

func names(items []itemJSON) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func contains(items []itemJSON, name string) bool {
	for _, it := range items {
		if it.Name == name {
			return true
		}
	}
	return false
}

func TestListRequiresAuth(t *testing.T) {
	router, _, _ := setup(t)
	rec, _ := do(t, router, http.MethodGet, "/api/v1/exercises", "", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListDefaultsAndPagination(t *testing.T) {
	router, a, _ := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	out := getList(t, router, bearer, "", "")
	if out.Total != 3 || len(out.Items) != 3 || out.Page != 1 || out.PerPage != 20 || out.Pages != 1 {
		t.Fatalf("unexpected defaults: %+v", out)
	}
	if out.Items[0].Name != "Жим штанги лёжа" {
		t.Fatalf("expected ru name asc default sort, got %v", names(out.Items))
	}

	bench := out.Items[0]
	if bench.BodyPart.Name != "Грудь" || bench.Equipment.Name != "Штанга" || bench.TargetMuscle.Name != "Грудные" {
		t.Fatalf("unexpected refs: %+v", bench)
	}
	if bench.TrackingType != "weight_reps" {
		t.Fatalf("unexpected tracking type: %s", bench.TrackingType)
	}
	if bench.Media.GifURL != "/media/videos/0001-t.gif" ||
		bench.Media.ThumbnailURL != "/media/images/0001-t.jpg" ||
		bench.Media.Attribution == nil || *bench.Media.Attribution == "" {
		t.Fatalf("unexpected media: %+v", bench.Media)
	}

	out = getList(t, router, bearer, "?per_page=2&page=2", "")
	if out.Total != 3 || out.Pages != 2 || len(out.Items) != 1 || out.Page != 2 {
		t.Fatalf("unexpected page 2: %+v", out)
	}

	out = getList(t, router, bearer, "?sort=newest", "")
	if out.Items[0].Name != "Планка" {
		t.Fatalf("expected newest first, got %v", names(out.Items))
	}

	out = getList(t, router, bearer, "?order=desc", "")
	if out.Items[0].Name != "Планка" {
		t.Fatalf("expected desc name sort, got %v", names(out.Items))
	}

	en := getList(t, router, bearer, "", "en-US,en;q=0.9")
	if en.Items[0].BodyPart.Name != "Chest" || en.Items[0].Equipment.Name != "Barbell" {
		t.Fatalf("expected en refs, got %+v", en.Items[0])
	}
}

func TestListFilters(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	out := getList(t, router, bearer, fmt.Sprintf("?body_part_ids=%d", fx.chestID), "")
	if out.Total != 2 || !contains(out.Items, "Жим штанги лёжа") || !contains(out.Items, "Отжимания") {
		t.Fatalf("body part filter: %+v", out)
	}

	out = getList(t, router, bearer, fmt.Sprintf("?equipment_ids=%d", fx.barbellID), "")
	if out.Total != 1 || out.Items[0].Name != "Жим штанги лёжа" {
		t.Fatalf("equipment filter: %+v", out)
	}

	out = getList(t, router, bearer, "?tracking_types=time", "")
	if out.Total != 1 || out.Items[0].Name != "Планка" {
		t.Fatalf("tracking type filter: %+v", out)
	}

	out = getList(t, router, bearer, fmt.Sprintf("?muscle_ids=%d", fx.absID), "")
	if out.Total != 1 || out.Items[0].Name != "Планка" {
		t.Fatalf("target muscle filter: %+v", out)
	}

	out = getList(t, router, bearer, fmt.Sprintf("?body_part_ids=%d&equipment_ids=%d", fx.chestID, fx.barbellID), "")
	if out.Total != 1 || out.Items[0].Name != "Жим штанги лёжа" {
		t.Fatalf("combined filter: %+v", out)
	}

	rec, _ := do(t, router, http.MethodGet, "/api/v1/exercises?body_part_ids=abc", bearer, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid ids: expected 400, got %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodGet, "/api/v1/exercises?tracking_types=bogus", bearer, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid tracking type: expected 400, got %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodGet, "/api/v1/exercises?sort=bogus", bearer, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid sort: expected 400, got %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodGet, "/api/v1/exercises?per_page=abc", bearer, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid per_page: expected 400, got %d", rec.Code)
	}
}

func TestListSearch(t *testing.T) {
	router, a, _ := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	out := getList(t, router, bearer, "?q=%D1%88%D1%82%D0%B0%D0%BD%D0%B3", "")
	if out.Total != 1 || out.Items[0].Name != "Жим штанги лёжа" {
		t.Fatalf("fulltext search: %+v", out)
	}

	out = getList(t, router, bearer, "?q=%D1%88", "")
	if out.Total != 1 || out.Items[0].Name != "Жим штанги лёжа" {
		t.Fatalf("like search: %+v", out)
	}

	out = getList(t, router, bearer, "?q=bench", "en-US,en;q=0.9")
	if out.Total != 1 || out.Items[0].Name != "Barbell Bench Press" {
		t.Fatalf("en search: %+v", out)
	}

	out = getList(t, router, bearer, "?q=%D1%83%D0%BD%D0%B8%D0%B2%D0%B5%D1%80%D1%81", "")
	if out.Total != 0 {
		t.Fatalf("no match search: %+v", out)
	}
}

func TestListMyEquipment(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	out := getList(t, router, bearer, "?my_equipment=1", "")
	if out.Total != 2 || !contains(out.Items, "Отжимания") || !contains(out.Items, "Планка") {
		t.Fatalf("body weight only expected: %v", names(out.Items))
	}

	rec, _ := do(t, router, http.MethodPut, "/api/v1/profile/equipment", bearer, "", map[string]any{
		"equipment_ids": []uint16{fx.barbellID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace equipment: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	out = getList(t, router, bearer, "?my_equipment=1", "")
	if out.Total != 3 {
		t.Fatalf("selected equipment must be included: %v", names(out.Items))
	}
}

func TestDetail(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	rec, env := do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/exercises/%d", fx.benchID), bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var d detailJSON
	if err := json.Unmarshal(env.Data, &d); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if d.Name != "Жим штанги лёжа" || d.Instructions == nil || len(d.Steps) != 4 || d.Steps[0] != "Лягте на скамью" {
		t.Fatalf("unexpected detail text: %+v", d)
	}
	if d.Equipment.Name != "Штанга" || d.TrackingType != "weight_reps" {
		t.Fatalf("unexpected detail refs: %+v", d)
	}
	if len(d.SecondaryMuscles) != 1 || d.SecondaryMuscles[0].Name != "Пресс" {
		t.Fatalf("unexpected secondary muscles: %+v", d.SecondaryMuscles)
	}
	if len(d.Media) != 2 {
		t.Fatalf("expected 2 media items, got %d", len(d.Media))
	}
	for _, m := range d.Media {
		if len(m.URL) == 0 || m.URL[0] != '/' || m.Width == nil || *m.Width != 180 {
			t.Fatalf("unexpected media item: %+v", m)
		}
	}

	rec, _ = do(t, router, http.MethodGet, "/api/v1/exercises/999999", bearer, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown exercise: expected 404, got %d", rec.Code)
	}

	if _, err := a.DB.Exec(`UPDATE exercises SET is_active = 0 WHERE id = ?`, fx.benchID); err != nil {
		t.Fatalf("deactivate exercise: %v", err)
	}
	rec, _ = do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/exercises/%d", fx.benchID), bearer, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("inactive exercise: expected 404, got %d", rec.Code)
	}
	out := getList(t, router, bearer, "", "")
	if out.Total != 2 {
		t.Fatalf("inactive exercise must be hidden from list: %v", names(out.Items))
	}
}

func TestFiltersEndpoint(t *testing.T) {
	router, a, fx := setup(t)
	bearer, _ := registerAndLogin(t, router, a)

	rec, env := do(t, router, http.MethodGet, "/api/v1/exercises/filters", bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filters: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var f filtersJSON
	if err := json.Unmarshal(env.Data, &f); err != nil {
		t.Fatalf("unmarshal filters: %v", err)
	}
	if len(f.BodyParts) != 2 || len(f.Equipment) != 2 || len(f.Muscles) != 2 {
		t.Fatalf("unexpected dictionaries: %+v", f)
	}
	if f.Equipment[0].ID != fx.barbellID && f.Equipment[1].ID != fx.barbellID {
		t.Fatalf("barbell missing from equipment: %+v", f.Equipment)
	}
	if len(f.TrackingTypes) != 4 || f.TrackingTypes[0].Value != "weight_reps" || f.TrackingTypes[0].Name != "Вес × повторы" {
		t.Fatalf("unexpected tracking types: %+v", f.TrackingTypes)
	}

	rec, env = do(t, router, http.MethodGet, "/api/v1/exercises/filters", bearer, "en-GB,en;q=0.9", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filters en: expected 200, got %d", rec.Code)
	}
	f = filtersJSON{}
	if err := json.Unmarshal(env.Data, &f); err != nil {
		t.Fatalf("unmarshal filters en: %v", err)
	}
	if f.BodyParts[0].Name != "Chest" && f.BodyParts[1].Name != "Chest" {
		t.Fatalf("expected en names, got %+v", f.BodyParts)
	}
}

func TestFavorites(t *testing.T) {
	router, a, fx := setup(t)
	bearer, userID := registerAndLogin(t, router, a)
	benchURL := fmt.Sprintf("/api/v1/exercises/%d/favorite", fx.benchID)

	// 401 без токена.
	rec, _ := do(t, router, http.MethodPost, benchURL, "", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("add favorite anon: expected 401, got %d", rec.Code)
	}
	rec, _ = do(t, router, http.MethodDelete, benchURL, "", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("remove favorite anon: expected 401, got %d", rec.Code)
	}

	// 404 на несуществующее упражнение.
	rec, _ = do(t, router, http.MethodPost, "/api/v1/exercises/999999/favorite", bearer, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown exercise: expected 404, got %d", rec.Code)
	}

	// 404 на неактивное упражнение.
	hiddenID := insertExercise(t, a, "0009", fx.chestID, fx.bodyWeightID, fx.pectoralsID, "reps")
	if _, err := a.DB.Exec(`UPDATE exercises SET is_active = 0 WHERE id = ?`, hiddenID); err != nil {
		t.Fatalf("deactivate exercise: %v", err)
	}
	rec, _ = do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/exercises/%d/favorite", hiddenID), bearer, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("inactive exercise: expected 404, got %d", rec.Code)
	}

	var fav struct {
		IsFavorite bool `json:"is_favorite"`
	}

	// Добавление.
	rec, env := do(t, router, http.MethodPost, benchURL, bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("add favorite: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(env.Data, &fav); err != nil || !fav.IsFavorite {
		t.Fatalf("add favorite response: %s %v", env.Data, err)
	}

	// Повторное добавление идемпотентно (INSERT IGNORE).
	rec, _ = do(t, router, http.MethodPost, benchURL, bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-add favorite: expected 200, got %d", rec.Code)
	}
	var count int
	if err := a.DB.Get(&count, `SELECT COUNT(*) FROM exercise_favorites WHERE user_id = ? AND exercise_id = ?`, userID, fx.benchID); err != nil || count != 1 {
		t.Fatalf("expected single favorite row, got %d (%v)", count, err)
	}

	// is_favorite в списке по умолчанию.
	out := getList(t, router, bearer, "", "")
	if out.Total != 3 {
		t.Fatalf("expected 3 items, got %d", out.Total)
	}
	for _, it := range out.Items {
		want := it.ID == uint16(fx.benchID)
		if it.IsFavorite != want {
			t.Fatalf("unexpected is_favorite for %s: %v", it.Name, it.IsFavorite)
		}
	}

	// Фильтр favorites_only.
	out = getList(t, router, bearer, "?favorites_only=1", "")
	if out.Total != 1 || len(out.Items) != 1 || out.Items[0].Name != "Жим штанги лёжа" || !out.Items[0].IsFavorite {
		t.Fatalf("favorites_only: %+v", out)
	}

	// is_favorite в детальной карточке.
	getDetail := func(id uint32) detailJSON {
		t.Helper()
		rec, env := do(t, router, http.MethodGet, fmt.Sprintf("/api/v1/exercises/%d", id), bearer, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("detail %d: expected 200, got %d", id, rec.Code)
		}
		var d detailJSON
		if err := json.Unmarshal(env.Data, &d); err != nil {
			t.Fatalf("unmarshal detail: %v", err)
		}
		return d
	}
	if d := getDetail(fx.benchID); !d.IsFavorite {
		t.Fatalf("bench detail: expected is_favorite")
	}
	if d := getDetail(fx.pushupID); d.IsFavorite {
		t.Fatalf("pushup detail: expected not favorite")
	}

	// Изоляция пользователей: у второго юзера избранного нет.
	other, _ := registerAndLogin(t, router, a)
	out = getList(t, router, other, "?favorites_only=1", "")
	if out.Total != 0 {
		t.Fatalf("favorites must be per-user, got %+v", out)
	}

	// Удаление и идемпотентность удаления.
	rec, env = do(t, router, http.MethodDelete, benchURL, bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove favorite: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(env.Data, &fav); err != nil || fav.IsFavorite {
		t.Fatalf("remove favorite response: %s %v", env.Data, err)
	}
	rec, _ = do(t, router, http.MethodDelete, benchURL, bearer, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-remove favorite: expected 200, got %d", rec.Code)
	}

	out = getList(t, router, bearer, "?favorites_only=1", "")
	if out.Total != 0 {
		t.Fatalf("expected empty favorites after delete: %+v", out)
	}
	if d := getDetail(fx.benchID); d.IsFavorite {
		t.Fatalf("bench detail after delete: expected not favorite")
	}
}
