package schedule_test

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

type ruleJSON struct {
	ID            uint32   `json:"id"`
	UserProgramID uint32   `json:"user_program_id"`
	ProgramDayID  uint32   `json:"program_day_id"`
	DayName       string   `json:"day_name"`
	DayPosition   uint8    `json:"day_position"`
	Frequency     string   `json:"frequency"`
	IntervalN     uint8    `json:"interval_n"`
	Weekdays      []string `json:"weekdays"`
	OnceDate      *string  `json:"once_date"`
	IsEnabled     bool     `json:"is_enabled"`
}

type userProgramJSON struct {
	ID          uint32     `json:"id"`
	ProgramID   uint32     `json:"program_id"`
	ProgramName string     `json:"program_name"`
	StartDate   string     `json:"start_date"`
	IsActive    bool       `json:"is_active"`
	Rules       []ruleJSON `json:"rules"`
}

type planEntryJSON struct {
	UserProgramID uint32 `json:"user_program_id"`
	ProgramID     uint32 `json:"program_id"`
	ProgramName   string `json:"program_name"`
	ProgramDayID  uint32 `json:"program_day_id"`
	DayName       string `json:"day_name"`
	Position      uint8  `json:"position"`
}

type planDayJSON struct {
	Date    string          `json:"date"`
	Weekday string          `json:"weekday"`
	Entries []planEntryJSON `json:"entries"`
}

type weekPlanJSON struct {
	StartDate string        `json:"start_date"`
	EndDate   string        `json:"end_date"`
	Days      []planDayJSON `json:"days"`
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
	)

	clean(t, a)
	return router, a
}

func clean(t *testing.T, a *app.App) {
	t.Helper()
	for _, q := range []string{
		`DELETE FROM user_schedule_rules`,
		`DELETE FROM user_programs`,
		`DELETE FROM program_schedule_rules`,
		`DELETE FROM program_sets`,
		`DELETE FROM program_exercises`,
		`DELETE FROM program_days`,
		`DELETE FROM programs`,
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

// registerAndLogin creates a verified user and returns the access token.
func registerAndLogin(t *testing.T, router *gin.Engine, a *app.App) string {
	t.Helper()
	return loginAs(t, router, a, fmt.Sprintf("sc%d@gym.test", time.Now().UnixNano()), false)
}

// promoteAndLogin registers an admin user and returns the access token.
func promoteAndLogin(t *testing.T, router *gin.Engine, a *app.App) string {
	t.Helper()
	return loginAs(t, router, a, fmt.Sprintf("sa%d@gym.test", time.Now().UnixNano()), true)
}

func loginAs(t *testing.T, router *gin.Engine, a *app.App, email string, admin bool) string {
	t.Helper()

	rec, _ := do(t, router, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
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
	if admin {
		if _, err := a.DB.Exec(`UPDATE users SET role = 'admin' WHERE id = ?`, userID); err != nil {
			t.Fatalf("promote: %v", err)
		}
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
	return tokens.AccessToken
}

// createProgram returns the id of the created program.
func createProgram(t *testing.T, router *gin.Engine, bearer string, body map[string]any) uint32 {
	t.Helper()
	rec, env := do(t, router, http.MethodPost, "/api/v1/programs", bearer, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("create program: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var out struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("unmarshal program: %v", err)
	}
	return out.ID
}

func createProgramWithDays(t *testing.T, router *gin.Engine, bearer, name string, days []map[string]any) uint32 {
	t.Helper()
	return createProgram(t, router, bearer, map[string]any{"name": name, "days": days})
}

func day(name string, schedule map[string]any) map[string]any {
	return map[string]any{"name": name, "schedule": schedule, "exercises": []map[string]any{}}
}

func weekly(interval int, weekdays ...string) map[string]any {
	return map[string]any{"frequency": "weekly", "interval_n": interval, "weekdays": weekdays}
}

func daily(interval int) map[string]any {
	return map[string]any{"frequency": "daily", "interval_n": interval}
}

func once(date string) map[string]any {
	return map[string]any{"frequency": "once", "once_date": date}
}

func selectProgram(t *testing.T, router *gin.Engine, bearer string, programID uint32, startDate string) (int, apiEnvelope, userProgramJSON) {
	t.Helper()
	body := map[string]any{}
	if startDate != "" {
		body["start_date"] = startDate
	}
	rec, env := do(t, router, http.MethodPost, fmt.Sprintf("/api/v1/programs/%d/select", programID), bearer, body)
	var out userProgramJSON
	if env.Data != nil {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			t.Fatalf("unmarshal selected program: %v", err)
		}
	}
	return rec.Code, env, out
}

func getSchedule(t *testing.T, router *gin.Engine, bearer string) []userProgramJSON {
	t.Helper()
	rec, env := do(t, router, http.MethodGet, "/api/v1/schedule", bearer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get schedule: expected 200, got %d", rec.Code)
	}
	var out []userProgramJSON
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}
	return out
}

func getWeek(t *testing.T, router *gin.Engine, bearer, date string) weekPlanJSON {
	t.Helper()
	path := "/api/v1/schedule/week"
	if date != "" {
		path += "?date=" + date
	}
	rec, env := do(t, router, http.MethodGet, path, bearer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get week: expected 200, got %d: %s", rec.Code, env.Message)
	}
	var out weekPlanJSON
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("unmarshal week plan: %v", err)
	}
	return out
}

func updateRule(t *testing.T, router *gin.Engine, bearer string, ruleID uint32, body map[string]any) (int, apiEnvelope, ruleJSON) {
	t.Helper()
	rec, env := do(t, router, http.MethodPut, fmt.Sprintf("/api/v1/schedule/rules/%d", ruleID), bearer, body)
	var out ruleJSON
	if env.Data != nil {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			t.Fatalf("unmarshal rule: %v", err)
		}
	}
	return rec.Code, env, out
}

func deactivate(t *testing.T, router *gin.Engine, bearer string, userProgramID uint32) int {
	t.Helper()
	rec, _ := do(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/schedule/programs/%d", userProgramID), bearer, nil)
	return rec.Code
}

func entriesOn(plan weekPlanJSON, date string) []planEntryJSON {
	for _, d := range plan.Days {
		if d.Date == date {
			return d.Entries
		}
	}
	return nil
}

// dayEntries returns the entries of the date or fails the test.
func dayEntries(t *testing.T, plan weekPlanJSON, date string) []planEntryJSON {
	t.Helper()
	entries := entriesOn(plan, date)
	if entries == nil {
		t.Fatalf("date %s is not in the week plan (%s..%s)", date, plan.StartDate, plan.EndDate)
	}
	return entries
}

func totalEntries(plan weekPlanJSON) int {
	n := 0
	for _, d := range plan.Days {
		n += len(d.Entries)
	}
	return n
}

func TestScheduleRequireAuth(t *testing.T) {
	router, _ := setup(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/schedule"},
		{http.MethodGet, "/api/v1/schedule/week"},
		{http.MethodPut, "/api/v1/schedule/rules/1"},
		{http.MethodDelete, "/api/v1/schedule/programs/1"},
		{http.MethodPost, "/api/v1/programs/1/select"},
	} {
		rec, _ := do(t, router, tc.method, tc.path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestSelectCopiesRulesAndWeekPlan(t *testing.T) {
	router, a := setup(t)
	bearer := registerAndLogin(t, router, a)

	// A program: base day on mon/wed/fri, one-off day on a concrete date.
	programID := createProgramWithDays(t, router, bearer, "Пн/Ср/Пт", []map[string]any{
		day("Верх", weekly(1, "mon", "wed", "fri")),
		day("Разовая", once("2026-11-25")),
	})

	code, env, up := selectProgram(t, router, bearer, programID, "2026-11-02")
	if code != http.StatusOK {
		t.Fatalf("select: expected 200, got %d: %s", code, env.Message)
	}
	if up.ID == 0 || !up.IsActive || up.ProgramID != programID || up.ProgramName != "Пн/Ср/Пт" {
		t.Fatalf("unexpected selected program: %+v", up)
	}
	if up.StartDate != "2026-11-02" {
		t.Fatalf("unexpected start_date: %s", up.StartDate)
	}
	if len(up.Rules) != 2 {
		t.Fatalf("expected 2 copied rules, got %d: %+v", len(up.Rules), up.Rules)
	}
	if up.Rules[0].DayName != "Верх" || up.Rules[0].Frequency != "weekly" ||
		len(up.Rules[0].Weekdays) != 3 || !up.Rules[0].IsEnabled {
		t.Fatalf("unexpected rule 1: %+v", up.Rules[0])
	}
	if up.Rules[1].DayName != "Разовая" || up.Rules[1].Frequency != "once" ||
		up.Rules[1].OnceDate == nil || *up.Rules[1].OnceDate != "2026-11-25" {
		t.Fatalf("unexpected rule 2: %+v", up.Rules[1])
	}

	// GET /schedule mirrors the selection.
	sched := getSchedule(t, router, bearer)
	if len(sched) != 1 || sched[0].ID != up.ID || len(sched[0].Rules) != 2 {
		t.Fatalf("unexpected schedule: %+v", sched)
	}

	// Week 2026-11-02..08: base day on mon/wed/fri, the once rule is silent.
	plan := getWeek(t, router, bearer, "2026-11-02")
	if plan.StartDate != "2026-11-02" || plan.EndDate != "2026-11-08" || len(plan.Days) != 7 {
		t.Fatalf("unexpected week bounds: %+v", plan)
	}
	if plan.Days[0].Weekday != "mon" || plan.Days[6].Weekday != "sun" {
		t.Fatalf("unexpected weekday labels: %+v", plan.Days)
	}
	for _, date := range []string{"2026-11-02", "2026-11-04", "2026-11-06"} {
		entries := dayEntries(t, plan, date)
		if len(entries) != 1 || entries[0].DayName != "Верх" || entries[0].ProgramID != programID {
			t.Fatalf("unexpected entries on %s: %+v", date, entries)
		}
	}
	for _, date := range []string{"2026-11-03", "2026-11-05", "2026-11-07", "2026-11-08"} {
		if entries := entriesOn(plan, date); len(entries) != 0 {
			t.Fatalf("unexpected entries on %s: %+v", date, entries)
		}
	}

	// Week with the once rule. 2026-11-25 is a Wednesday: the base day
	// fires there too, the once rule adds a second entry.
	plan = getWeek(t, router, bearer, "2026-11-23")
	entries := dayEntries(t, plan, "2026-11-25")
	if len(entries) != 2 || entries[0].DayName != "Верх" || entries[1].DayName != "Разовая" {
		t.Fatalf("unexpected once entry: %+v", entries)
	}
	for _, d := range plan.Days {
		want := 1 // base day on mon/wed/fri
		if d.Date == "2026-11-25" {
			want = 2
		}
		if d.Date == "2026-11-25" && len(d.Entries) != want || d.Date != "2026-11-25" && len(d.Entries) > want {
			t.Fatalf("unexpected entries on %s: %+v", d.Date, d.Entries)
		}
	}
	if len(entriesOn(plan, "2026-11-24")) != 0 || len(entriesOn(plan, "2026-11-26")) != 0 {
		t.Fatalf("once rule must fire only on 2026-11-25")
	}

	// Second activation of the same program: entries merge (multiple
	// active programs are allowed).
	if code, _, _ := selectProgram(t, router, bearer, programID, "2026-11-02"); code != http.StatusOK {
		t.Fatalf("second select failed")
	}
	sched = getSchedule(t, router, bearer)
	if len(sched) != 2 {
		t.Fatalf("expected 2 active programs, got %d", len(sched))
	}
	plan = getWeek(t, router, bearer, "2026-11-02")
	if got := dayEntries(t, plan, "2026-11-02"); len(got) != 2 {
		t.Fatalf("entries from both activations must be merged: %+v", got)
	}
}

// Only Saturdays: a single-day program scheduled on "sat".
func TestSaturdayOnlyProgram(t *testing.T) {
	router, a := setup(t)
	bearer := registerAndLogin(t, router, a)

	programID := createProgramWithDays(t, router, bearer, "По субботам", []map[string]any{
		day("Суббота", weekly(1, "sat")),
	})
	if _, _, up := selectProgram(t, router, bearer, programID, "2026-11-02"); up.ID == 0 {
		t.Fatalf("select failed")
	}

	plan := getWeek(t, router, bearer, "2026-11-02")
	entries := dayEntries(t, plan, "2026-11-07")
	if len(entries) != 1 || entries[0].DayName != "Суббота" || plan.Days[5].Weekday != "sat" {
		t.Fatalf("unexpected saturday entry: %+v", entries)
	}
	if totalEntries(plan) != 1 {
		t.Fatalf("only saturday must be scheduled: %+v", plan)
	}
}

func TestDailyAndWeeklyIntervals(t *testing.T) {
	router, a := setup(t)
	bearer := registerAndLogin(t, router, a)

	programID := createProgramWithDays(t, router, bearer, "Интервалы", []map[string]any{
		day("Каждый 2-й день", daily(2)),
		day("Через неделю в понедельник", weekly(2, "mon")),
	})
	if _, _, up := selectProgram(t, router, bearer, programID, "2026-11-02"); up.ID == 0 {
		t.Fatalf("select failed")
	}

	// Week 0 (anchor week): daily on 02/04/06, weekly mon on 02.
	plan := getWeek(t, router, bearer, "2026-11-02")
	if len(dayEntries(t, plan, "2026-11-02")) != 2 {
		t.Fatalf("anchor monday must have both entries: %+v", dayEntries(t, plan, "2026-11-02"))
	}
	for _, date := range []string{"2026-11-04", "2026-11-06"} {
		if entries := dayEntries(t, plan, date); len(entries) != 1 || entries[0].DayName != "Каждый 2-й день" {
			t.Fatalf("unexpected daily entries on %s: %+v", date, entries)
		}
	}

	// Week 1: weekly mon is skipped (interval 2); daily fires on 10/12/14.
	plan = getWeek(t, router, bearer, "2026-11-09")
	if entries := dayEntries(t, plan, "2026-11-09"); len(entries) != 0 {
		t.Fatalf("weekly mon must be skipped in week 1: %+v", entries)
	}
	for _, date := range []string{"2026-11-10", "2026-11-12", "2026-11-14"} {
		if entries := dayEntries(t, plan, date); len(entries) != 1 {
			t.Fatalf("unexpected daily entries on %s: %+v", date, entries)
		}
	}

	// Week 2: weekly mon fires again on 16.
	plan = getWeek(t, router, bearer, "2026-11-16")
	entries := dayEntries(t, plan, "2026-11-16")
	if len(entries) != 2 {
		t.Fatalf("week 2 monday must have both entries: %+v", entries)
	}
}

func TestUpdateRuleAndDisable(t *testing.T) {
	router, a := setup(t)
	bearer := registerAndLogin(t, router, a)

	programID := createProgramWithDays(t, router, bearer, "Правила", []map[string]any{
		day("База", weekly(1, "mon", "wed")),
	})
	_, _, up := selectProgram(t, router, bearer, programID, "2026-11-02")
	rule := up.Rules[0]

	// Change the weekdays.
	code, _, updated := updateRule(t, router, bearer, rule.ID, map[string]any{
		"frequency":  "weekly",
		"interval_n": 1,
		"weekdays":   []string{"tue", "thu"},
		"is_enabled": true,
	})
	if code != http.StatusOK {
		t.Fatalf("update rule: expected 200, got %d", code)
	}
	if len(updated.Weekdays) != 2 || updated.Weekdays[0] != "tue" || updated.Weekdays[1] != "thu" {
		t.Fatalf("unexpected updated weekdays: %+v", updated)
	}
	plan := getWeek(t, router, bearer, "2026-11-02")
	if len(dayEntries(t, plan, "2026-11-03")) != 1 || len(dayEntries(t, plan, "2026-11-05")) != 1 {
		t.Fatalf("entries must follow the updated rule: %+v", plan)
	}
	if len(dayEntries(t, plan, "2026-11-02")) != 0 {
		t.Fatalf("old weekdays must be gone: %+v", plan)
	}

	// Disable the rule: the plan becomes empty.
	if code, _, updated := updateRule(t, router, bearer, rule.ID, map[string]any{
		"frequency":  "weekly",
		"interval_n": 1,
		"weekdays":   []string{"tue", "thu"},
		"is_enabled": false,
	}); code != http.StatusOK || updated.IsEnabled {
		t.Fatalf("disable rule: code=%d rule=%+v", code, updated)
	}
	if plan := getWeek(t, router, bearer, "2026-11-02"); totalEntries(plan) != 0 {
		t.Fatalf("disabled rule must not fire: %+v", plan)
	}

	// Malformed rule -> 400.
	if code, _, _ := updateRule(t, router, bearer, rule.ID, map[string]any{
		"frequency": "bogus",
	}); code != http.StatusBadRequest {
		t.Fatalf("bogus rule: expected 400, got %d", code)
	}
	// Unknown rule -> 404.
	if code, _, _ := updateRule(t, router, bearer, 4294967295, map[string]any{
		"frequency": "weekly", "interval_n": 1, "weekdays": []string{"mon"},
	}); code != http.StatusNotFound {
		t.Fatalf("unknown rule: expected 404, got %d", code)
	}
}

func TestSelectPermissionsAndDeactivate(t *testing.T) {
	router, a := setup(t)
	user1 := registerAndLogin(t, router, a)
	user2 := registerAndLogin(t, router, a)
	admin := promoteAndLogin(t, router, a)

	// A ready program is created published; unpublishing makes it a draft
	// invisible to users: select -> 404.
	readyID := createProgram(t, router, admin, map[string]any{
		"name":         "Черновик",
		"is_published": true,
		"days":         []map[string]any{day("День", weekly(1, "mon"))},
	})
	if c := doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", readyID), admin, map[string]any{
		"name":         "Черновик",
		"is_published": false,
		"days":         []map[string]any{day("День", weekly(1, "mon"))},
	}); c != http.StatusOK {
		t.Fatalf("unpublish: %d", c)
	}
	if code, _, _ := selectProgram(t, router, user1, readyID, "2026-11-02"); code != http.StatusNotFound {
		t.Fatalf("select hidden draft: expected 404, got %d", code)
	}

	// Unknown program -> 404.
	if code, _, _ := selectProgram(t, router, user1, 4294967295, "2026-11-02"); code != http.StatusNotFound {
		t.Fatalf("select unknown: expected 404, got %d", code)
	}

	// Publish: now the user can select it.
	if c := doCode(t, router, http.MethodPut, fmt.Sprintf("/api/v1/programs/%d", readyID), admin, map[string]any{
		"name":         "Черновик",
		"is_published": true,
		"days":         []map[string]any{day("День", weekly(1, "mon"))},
	}); c != http.StatusOK {
		t.Fatalf("publish: %d", c)
	}
	code, _, up := selectProgram(t, router, user1, readyID, "2026-11-02")
	if code != http.StatusOK || up.ID == 0 {
		t.Fatalf("select published: %d", code)
	}

	// Foreign users cannot touch the selection.
	if c, _, _ := updateRule(t, router, user2, up.Rules[0].ID, map[string]any{
		"frequency": "weekly", "interval_n": 1, "weekdays": []string{"fri"},
	}); c != http.StatusForbidden {
		t.Fatalf("foreign update rule: expected 403, got %d", c)
	}
	if c := deactivate(t, router, user2, up.ID); c != http.StatusForbidden {
		t.Fatalf("foreign deactivate: expected 403, got %d", c)
	}

	// Owner deactivates: schedule and plan become empty.
	if c := deactivate(t, router, user1, up.ID); c != http.StatusOK {
		t.Fatalf("deactivate: %d", c)
	}
	if sched := getSchedule(t, router, user1); len(sched) != 0 {
		t.Fatalf("deactivated program must leave the schedule: %+v", sched)
	}
	if plan := getWeek(t, router, user1, "2026-11-02"); totalEntries(plan) != 0 {
		t.Fatalf("deactivated program must leave the plan: %+v", plan)
	}
}

func doCode(t *testing.T, router *gin.Engine, method, path, bearer string, body any) int {
	t.Helper()
	rec, _ := do(t, router, method, path, bearer, body)
	return rec.Code
}
