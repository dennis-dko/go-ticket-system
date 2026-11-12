package service_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/dennis-dko/go-time-recording/internal/application/v1/service"
	"github.com/dennis-dko/go-time-recording/internal/domain/model"
	"github.com/dennis-dko/go-time-recording/internal/support/apperror"
)

// The synchronisation's schedule is held to the same bound the form keeps, long
// enough for any expression somebody means: one listing every minute of an hour
// is taken, where the form's 120 characters refused it and the server took any
// length at all. A longer expression is refused even where the scheduler could
// read it.
func TestTheSchedulesLengthIsTheFormsAndFitsAnyRealExpression(t *testing.T) {
	f := newSyncFixture(t, 1.0)
	settings := service.NewSettingsService(newStubSettings(), f.roleRepo, f.userRepo, "Test")
	ctx := context.Background()

	minutes := make([]string, 60)
	for minute := range minutes {
		minutes[minute] = strconv.Itoa(minute)
	}

	config := model.DefaultLDAPConfig()
	config.SyncSchedule = strings.Join(minutes, ",") + " * * * *"

	if err := settings.SaveLDAP(ctx, config); err != nil {
		t.Fatalf("a schedule listing every minute (%d characters) is refused: %v", len(config.SyncSchedule), err)
	}

	config.SyncSchedule = strings.Repeat("0,", model.MaxScheduleLength/2) + "0 * * * *"

	if !model.ValidSchedule(config.SyncSchedule) {
		t.Fatal("the long schedule is not one the scheduler reads, so its refusal would prove nothing about its length")
	}

	requireKind(t, settings.SaveLDAP(ctx, config), apperror.KindInvalid)
}
