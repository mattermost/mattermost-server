// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

func (a *App) GetScheme(id string) (*model.Scheme, *model.AppError) {
	if appErr := a.IsPhase2MigrationCompleted(); appErr != nil {
		return nil, appErr
	}

	scheme, err := a.Srv().Store.Scheme().Get(id)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetScheme", "app.scheme.get.app_error", nil, err.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetScheme", "app.scheme.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return scheme, nil
}

func (a *App) GetSchemeByName(name string) (*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	scheme, err := a.Srv().Store.Scheme().GetByName(name)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetSchemeByName", "app.scheme.get.app_error", nil, err.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("GetSchemeByName", "app.scheme.get.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return scheme, nil
}

func (a *App) GetSchemesPage(scope string, page int, perPage int) ([]*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	return a.GetSchemes(scope, page*perPage, perPage)
}

func (s *Server) GetSchemes(scope string, offset int, limit int) ([]*model.Scheme, *model.AppError) {
	if err := s.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	scheme, err := s.Store.Scheme().GetAllPage(scope, offset, limit)
	if err != nil {
		return nil, model.NewAppError("GetSchemes", "app.scheme.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return scheme, nil
}

func (a *App) GetSchemes(scope string, offset int, limit int) ([]*model.Scheme, *model.AppError) {
	return a.Srv().GetSchemes(scope, offset, limit)
}

func (a *App) CreateScheme(scheme *model.Scheme) (*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	// Clear any user-provided values for trusted properties.
	scheme.DefaultTeamAdminRole = ""
	scheme.DefaultTeamUserRole = ""
	scheme.DefaultTeamGuestRole = ""
	scheme.DefaultChannelAdminRole = ""
	scheme.DefaultChannelUserRole = ""
	scheme.DefaultChannelGuestRole = ""
	scheme.CreateAt = 0
	scheme.UpdateAt = 0
	scheme.DeleteAt = 0

	scheme, err := a.Srv().Store.Scheme().Save(scheme)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("CreateScheme", "app.scheme.save.invalid_scheme.app_error", nil, err.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("CreateScheme", "app.scheme.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return scheme, nil
}

func (a *App) PatchScheme(scheme *model.Scheme, patch *model.SchemePatch) (*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	scheme.Patch(patch)
	scheme, err := a.UpdateScheme(scheme)
	if err != nil {
		return nil, err
	}

	return scheme, err
}

func (a *App) UpdateScheme(scheme *model.Scheme) (*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	scheme, err := a.Srv().Store.Scheme().Save(scheme)
	if err != nil {
		var invErr *store.ErrInvalidInput
		var appErr *model.AppError
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &invErr):
			return nil, model.NewAppError("UpdateScheme", "app.scheme.save.invalid_scheme.app_error", nil, err.Error(), http.StatusBadRequest)
		default:
			return nil, model.NewAppError("UpdateScheme", "app.scheme.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return scheme, nil
}

func (a *App) DeleteScheme(schemeID string) (*model.Scheme, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	scheme, err := a.Srv().Store.Scheme().Delete(schemeID)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("DeleteScheme", "app.scheme.get.app_error", nil, err.Error(), http.StatusNotFound)
		default:
			return nil, model.NewAppError("DeleteScheme", "app.scheme.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return scheme, nil
}

func (a *App) GetTeamsForSchemePage(scheme *model.Scheme, page int, perPage int) ([]*model.Team, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	return a.GetTeamsForScheme(scheme, page*perPage, perPage)
}

func (a *App) GetTeamsForScheme(scheme *model.Scheme, offset int, limit int) ([]*model.Team, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	teams, err := a.Srv().Store.Team().GetTeamsByScheme(scheme.ID, offset, limit)
	if err != nil {
		return nil, model.NewAppError("GetTeamsForScheme", "app.team.get_by_scheme.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return teams, nil
}

func (a *App) GetChannelsForSchemePage(scheme *model.Scheme, page int, perPage int) (model.ChannelList, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	return a.GetChannelsForScheme(scheme, page*perPage, perPage)
}

func (a *App) GetChannelsForScheme(scheme *model.Scheme, offset int, limit int) (model.ChannelList, *model.AppError) {
	if err := a.IsPhase2MigrationCompleted(); err != nil {
		return nil, err
	}

	channelList, nErr := a.Srv().Store.Channel().GetChannelsByScheme(scheme.ID, offset, limit)
	if nErr != nil {
		return nil, model.NewAppError("GetChannelsForScheme", "app.channel.get_by_scheme.app_error", nil, nErr.Error(), http.StatusInternalServerError)
	}

	return channelList, nil
}

func (s *Server) IsPhase2MigrationCompleted() *model.AppError {
	if s.phase2PermissionsMigrationComplete {
		return nil
	}

	if _, err := s.Store.System().GetByName(model.MigrationKeyAdvancedPermissionsPhase2); err != nil {
		return model.NewAppError("App.IsPhase2MigrationCompleted", "app.schemes.is_phase_2_migration_completed.not_completed.app_error", nil, err.Error(), http.StatusNotImplemented)
	}

	s.phase2PermissionsMigrationComplete = true

	return nil
}

func (a *App) IsPhase2MigrationCompleted() *model.AppError {
	return a.Srv().IsPhase2MigrationCompleted()
}

func (a *App) SchemesIterator(scope string, batchSize int) func() []*model.Scheme {
	offset := 0
	return func() []*model.Scheme {
		schemes, err := a.Srv().Store.Scheme().GetAllPage(scope, offset, batchSize)
		if err != nil {
			return []*model.Scheme{}
		}
		offset += batchSize
		return schemes
	}
}
