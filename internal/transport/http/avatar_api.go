package httptransport

import (
	"net/http"

	"github.com/asccclass/sherryserver/internal/avatarconfig"
)

type AvatarAPI struct {
	Store *avatarconfig.Store
}

func NewAvatarAPI(store *avatarconfig.Store) *AvatarAPI {
	return &AvatarAPI{Store: store}
}

func (api *AvatarAPI) ListAvatars(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.Store == nil {
		writeJSONError(w, http.StatusInternalServerError, "avatar store not configured")
		return
	}

	defaultAvatar := api.Store.Default()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"defaultAvatarId": defaultAvatar.ID,
		"avatars":         api.Store.Enabled(),
	})
}
