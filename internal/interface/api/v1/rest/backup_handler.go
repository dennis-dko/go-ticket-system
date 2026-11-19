package rest

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/response"

	"github.com/dennis-dko/timepulse/internal/application/v1/service"
	"github.com/dennis-dko/timepulse/internal/domain/model"
	"github.com/dennis-dko/timepulse/internal/support/backup"
)

// BackupHandler serves the backup card: the plan, the backups kept, and making,
// downloading and removing one.
//
// All of it needs settings:manage, because a backup is the whole installation -
// every account, every hour, every setting - and a download is a copy of it that
// leaves the server. Each of those, the download included, is in the
// administration log.
type BackupHandler struct {
	backups *service.BackupService
	authz   *Authorizer
}

// NewBackupHandler serves the backups the service keeps.
func NewBackupHandler(backups *service.BackupService, authz *Authorizer) *BackupHandler {
	return &BackupHandler{backups: backups, authz: authz}
}

// BackupResponse is a backup kept on the server.
type BackupResponse struct {
	Name string    `json:"name"`
	Size int64     `json:"size"`
	Made time.Time `json:"made"`
}

// BackupStateResponse is what the card shows.
type BackupStateResponse struct {
	// Enabled says a backup can be made, which takes BACKUP_PASSPHRASE of at
	// least MinPassphraseLength characters on the server.
	Enabled             bool `json:"enabled"`
	MinPassphraseLength int  `json:"minPassphraseLength"`

	// Folder is where the backups are kept, so the card can say where to look -
	// and what to put on a volume.
	Folder string `json:"folder"`

	Schedule string           `json:"schedule"`
	Keep     int              `json:"keep"`
	MaxKeep  int              `json:"maxKeep"`
	Backups  []BackupResponse `json:"backups"`
}

// BackupPlanRequest is a plan as the card saves it.
type BackupPlanRequest struct {
	Schedule string `json:"schedule"`
	Keep     int    `json:"keep"`
}

// State handles GET /api/v1/settings/backups.
func (h *BackupHandler) State(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	return h.state(c)
}

// Make handles POST /api/v1/settings/backups: a backup now, kept with the others
// and answered like one of them, so the card can download it.
func (h *BackupHandler) Make(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	made, err := h.backups.Make(c)
	if err != nil {
		return nil, toHTTPError(err)
	}

	return backupResponse(made), nil
}

// Download handles GET /api/v1/settings/backups/{name}.
func (h *BackupHandler) Download(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	file, err := h.backups.Read(c.PathParam("name"))
	if err != nil {
		return nil, toHTTPError(err)
	}

	return response.File{Content: file, ContentType: "application/octet-stream"}, nil
}

// Delete handles DELETE /api/v1/settings/backups/{name}.
func (h *BackupHandler) Delete(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	if err := h.backups.Delete(c.PathParam("name")); err != nil {
		return nil, toHTTPError(err)
	}

	return map[string]string{"status": "deleted"}, nil
}

// SavePlan handles PUT /api/v1/settings/backups/plan, and answers with the whole
// card, so it is drawn from what was kept rather than from what was sent.
func (h *BackupHandler) SavePlan(c *gofr.Context) (any, error) {
	if _, err := h.authz.RequireInstallationAdmin(c); err != nil {
		return nil, err
	}

	var req BackupPlanRequest
	if err := bind(c, &req); err != nil {
		return nil, toHTTPError(err)
	}

	if _, err := h.backups.SavePlan(c, model.BackupPlan{Schedule: req.Schedule, Keep: req.Keep}); err != nil {
		return nil, toHTTPError(err)
	}

	return h.state(c)
}

func (h *BackupHandler) state(c *gofr.Context) (BackupStateResponse, error) {
	state, err := h.backups.State(c)
	if err != nil {
		return BackupStateResponse{}, toHTTPError(err)
	}

	out := BackupStateResponse{
		Enabled: state.Enabled, MinPassphraseLength: backup.MinPassphraseLength, Folder: state.Folder,
		Schedule: state.Plan.Schedule, Keep: state.Plan.Keep, MaxKeep: model.MaxBackupsKept,
		Backups: make([]BackupResponse, 0, len(state.Stored)),
	}

	for _, stored := range state.Stored {
		out.Backups = append(out.Backups, backupResponse(stored))
	}

	return out, nil
}

func backupResponse(stored backup.Stored) BackupResponse {
	return BackupResponse{Name: stored.Name, Size: stored.Size, Made: stored.Made}
}
