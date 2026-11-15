package profile_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gladart32/gym/internal/app"
	"github.com/gladart32/gym/internal/profile"
	"github.com/gladart32/gym/internal/testutil"
)

func newUserService(t *testing.T, a *app.App, email string) uint32 {
	t.Helper()
	res, err := a.DB.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`,
		email, "test-hash")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return uint32(id)
}

func TestProfileTimezone(t *testing.T) {
	a := testutil.Start(t)
	svc := profile.NewService(a.DB, a.Log)
	ctx := context.Background()

	male := "male"
	userID := newUserService(t, a, fmt.Sprintf("tz%d@profile.test", time.Now().UnixNano()))

	// Default column value is always present even without a profile row.
	got, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get empty profile: %v", err)
	}
	if got.Timezone == nil || *got.Timezone == "" {
		t.Fatalf("timezone must always be present: %+v", got)
	}
	if got.Gender != nil {
		t.Fatalf("unfilled profile must have nil fields: %+v", got)
	}

	// Update replaces the timezone and keeps it after profile saves.
	novosibirsk := "Asia/Novosibirsk"
	if _, err := svc.Update(ctx, userID, &profile.Profile{
		Gender:   &male,
		Timezone: &novosibirsk,
	}); err != nil {
		t.Fatalf("update with timezone: %v", err)
	}
	got, err = svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Timezone == nil || *got.Timezone != "Asia/Novosibirsk" {
		t.Fatalf("expected Asia/Novosibirsk, got %+v", got.Timezone)
	}
	if got.Gender == nil || *got.Gender != "male" {
		t.Fatalf("profile fields lost: %+v", got)
	}

	// Timezone nil = keep the current one.
	if _, err := svc.Update(ctx, userID, &profile.Profile{Gender: &male}); err != nil {
		t.Fatalf("update without timezone: %v", err)
	}
	got, err = svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Timezone == nil || *got.Timezone != "Asia/Novosibirsk" {
		t.Fatalf("timezone must survive a save without it: %+v", got.Timezone)
	}

	// Invalid IANA names are rejected.
	bad := "Mars/Olympus"
	if _, err := svc.Update(ctx, userID, &profile.Profile{Timezone: &bad}); err != profile.ErrInvalidTimezone {
		t.Fatalf("invalid timezone: expected ErrInvalidTimezone, got %v", err)
	}
	toolong := make([]byte, 65)
	for i := range toolong {
		toolong[i] = 'a'
	}
	badlen := string(toolong)
	if _, err := svc.Update(ctx, userID, &profile.Profile{Timezone: &badlen}); err != profile.ErrInvalidTimezone {
		t.Fatalf("too long timezone: expected ErrInvalidTimezone, got %v", err)
	}
}

func TestNormalizeTimezone(t *testing.T) {
	if loc, err := profile.NormalizeTimezone(""); err != nil || loc != nil {
		t.Fatalf("empty: expected nil/nil, got %v/%v", loc, err)
	}
	if loc, err := profile.NormalizeTimezone("  Europe/Moscow  "); err != nil || loc == nil || loc.String() != "Europe/Moscow" {
		t.Fatalf("trim: got %v/%v", loc, err)
	}
	if _, err := profile.NormalizeTimezone("no/such/zone"); err != profile.ErrInvalidTimezone {
		t.Fatalf("invalid: expected ErrInvalidTimezone, got %v", err)
	}
}
