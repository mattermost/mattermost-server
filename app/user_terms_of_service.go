// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) GetUserTermsOfService(userID string) (*model.UserTermsOfService, *model.AppError) {
	u, err := a.Srv().Store.UserTermsOfService().GetByUser(userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetUserTermsOfService", "app.user_terms_of_service.get_by_user.no_rows.app_error", nil, nfErr.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetUserTermsOfService", "app.user_terms_of_service.get_by_user.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return u, nil
}

func (a *App) SaveUserTermsOfService(userID, termsOfServiceID string, accepted bool) *model.AppError {
	if accepted {
		userTermsOfService := &model.UserTermsOfService{
			UserID:           userID,
			TermsOfServiceID: termsOfServiceID,
		}

		if _, err := a.Srv().Store.UserTermsOfService().Save(userTermsOfService); err != nil {
			var appErr *model.AppError
			switch {
			case errors.As(err, &appErr):
				return appErr
			default:
				return model.NewAppError("SaveUserTermsOfService", "app.user_terms_of_service.save.app_error", nil, err.Error(), http.StatusInternalServerError)
			}
		}
	} else {
		if err := a.Srv().Store.UserTermsOfService().Delete(userID, termsOfServiceID); err != nil {
			return model.NewAppError("SaveUserTermsOfService", "app.user_terms_of_service.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}

	return nil
}
