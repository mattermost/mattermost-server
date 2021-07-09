// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) CreateTermsOfService(text, userID string) (*model.TermsOfService, *model.AppError) {
	termsOfService := &model.TermsOfService{
		Text:   text,
		UserID: userID,
	}

	if _, appErr := a.GetUser(userID); appErr != nil {
		return nil, appErr
	}

	var err error
	if termsOfService, err = a.Srv().Store.TermsOfService().Save(termsOfService); err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateTermsOfService", "app.terms_of_service.create.existing.app_error", nil, "id="+termsOfService.ID, http.StatusBadRequest)
		case errors.As(err, &appErr):
			return nil, appErr
		default:
			return nil, model.NewAppError("CreateTermsOfService", "app.terms_of_service.create.app_error", nil, "terms_of_service_id="+termsOfService.ID+",err="+err.Error(), http.StatusInternalServerError)
		}
	}

	return termsOfService, nil
}

func (a *App) GetLatestTermsOfService() (*model.TermsOfService, *model.AppError) {
	termsOfService, err := a.Srv().Store.TermsOfService().GetLatest(true)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetLatestTermsOfService", "app.terms_of_service.get.no_rows.app_error", nil, "err="+err.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetLatestTermsOfService", "app.terms_of_service.get.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
		}
	}
	return termsOfService, nil
}

func (a *App) GetTermsOfService(id string) (*model.TermsOfService, *model.AppError) {
	termsOfService, err := a.Srv().Store.TermsOfService().Get(id, true)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetTermsOfService", "app.terms_of_service.get.no_rows.app_error", nil, "", http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetTermsOfService", "app.terms_of_service.get.app_error", nil, "err="+err.Error(), http.StatusInternalServerError)
		}
	}
	return termsOfService, nil
}
