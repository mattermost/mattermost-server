// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

// SqlLicenseStore encapsulates the database writes and reads for
// model.LicenseRecord objects.
type SQLLicenseStore struct {
	*SQLStore
}

func newSQLLicenseStore(sqlStore *SQLStore) store.LicenseStore {
	ls := &SQLLicenseStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.LicenseRecord{}, "Licenses").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Bytes").SetMaxSize(10000)
	}

	return ls
}

func (ls SQLLicenseStore) createIndexesIfNotExists() {
}

// Save validates and stores the license instance in the database. The Id
// and Bytes fields are mandatory. The Bytes field is limited to a maximum
// of 10000 bytes. If the license ID matches an existing license in the
// database it returns the license stored in the database. If not, it saves the
// new database and returns the created license with the CreateAt field
// updated.
func (ls SQLLicenseStore) Save(license *model.LicenseRecord) (*model.LicenseRecord, error) {
	license.PreSave()
	if err := license.IsValid(); err != nil {
		return nil, err
	}
	query := ls.getQueryBuilder().
		Select("*").
		From("Licenses").
		Where(sq.Eq{"Id": license.ID})
	queryString, args, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "license_tosql")
	}
	var storedLicense model.LicenseRecord
	if err := ls.GetReplica().SelectOne(&storedLicense, queryString, args...); err != nil {
		// Only insert if not exists
		if err := ls.GetMaster().Insert(license); err != nil {
			return nil, errors.Wrapf(err, "failed to get License with licenseId=%s", license.ID)
		}
		return license, nil
	}
	return &storedLicense, nil
}

// Get obtains the license with the provided id parameter from the database.
// If the license doesn't exist it returns a model.AppError with
// http.StatusNotFound in the StatusCode field.
func (ls SQLLicenseStore) Get(id string) (*model.LicenseRecord, error) {
	obj, err := ls.GetReplica().Get(model.LicenseRecord{}, id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get License with licenseId=%s", id)
	}
	if obj == nil {
		return nil, store.NewErrNotFound("License", id)
	}
	return obj.(*model.LicenseRecord), nil
}

func (ls SQLLicenseStore) GetAll() ([]*model.LicenseRecord, error) {
	query := ls.getQueryBuilder().
		Select("*").
		From("Licenses")

	queryString, _, err := query.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "license_tosql")
	}

	var licenses []*model.LicenseRecord
	if _, err := ls.GetReplica().Select(&licenses, queryString); err != nil {
		return nil, errors.Wrap(err, "failed to fetch licenses")
	}

	return licenses, nil
}
