// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLLinkMetadataStore struct {
	*SQLStore
}

func newSQLLinkMetadataStore(sqlStore *SQLStore) store.LinkMetadataStore {
	s := &SQLLinkMetadataStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.LinkMetadata{}, "LinkMetadata").SetKeys(false, "Hash")
		table.ColMap("URL").SetMaxSize(2048)
		table.ColMap("Type").SetMaxSize(16)
		table.ColMap("Data").SetMaxSize(4096)
	}

	return s
}

func (s SQLLinkMetadataStore) createIndexesIfNotExists() {
	if s.DriverName() == model.DatabaseDriverMysql {
		s.CreateCompositeIndexIfNotExists("idx_link_metadata_url_timestamp", "LinkMetadata", []string{"URL(512)", "Timestamp"})
	} else {
		s.CreateCompositeIndexIfNotExists("idx_link_metadata_url_timestamp", "LinkMetadata", []string{"URL", "Timestamp"})
	}
}

func (s SQLLinkMetadataStore) Save(metadata *model.LinkMetadata) (*model.LinkMetadata, error) {
	if err := metadata.IsValid(); err != nil {
		return nil, err
	}

	metadata.PreSave()

	err := s.GetMaster().Insert(metadata)
	if err != nil && !IsUniqueConstraintError(err, []string{"PRIMARY", "linkmetadata_pkey"}) {
		return nil, errors.Wrap(err, "could not save link metadata")
	}

	return metadata, nil
}

func (s SQLLinkMetadataStore) Get(url string, timestamp int64) (*model.LinkMetadata, error) {
	var metadata *model.LinkMetadata
	query, args, err := s.getQueryBuilder().
		Select("*").
		From("LinkMetadata").
		Where(sq.Eq{"URL": url, "Timestamp": timestamp}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "could not create query with querybuilder")
	}
	err = s.GetReplica().SelectOne(&metadata, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("LinkMetadata", "url="+url)
		}
		return nil, errors.Wrapf(err, "could not get metadata with selectone: url=%s", url)
	}

	err = metadata.DeserializeDataToConcreteType()
	if err != nil {
		return nil, errors.Wrapf(err, "could not deserialize metadata to concrete type for url=%s", url)
	}

	return metadata, nil
}
