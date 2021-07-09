// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/mattermost/gorp"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SQLJobStore struct {
	*SQLStore
}

func newSQLJobStore(sqlStore *SQLStore) store.JobStore {
	s := &SQLJobStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Job{}, "Jobs").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("Type").SetMaxSize(32)
		table.ColMap("Status").SetMaxSize(32)
		table.ColMap("Data").SetMaxSize(1024)
	}

	return s
}

func (jss SQLJobStore) createIndexesIfNotExists() {
	jss.CreateIndexIfNotExists("idx_jobs_type", "Jobs", "Type")
}

func (jss SQLJobStore) Save(job *model.Job) (*model.Job, error) {
	if err := jss.GetMaster().Insert(job); err != nil {
		return nil, errors.Wrap(err, "failed to save Job")
	}
	return job, nil
}

func (jss SQLJobStore) UpdateOptimistically(job *model.Job, currentStatus string) (bool, error) {
	query, args, err := jss.getQueryBuilder().
		Update("Jobs").
		Set("LastActivityAt", model.GetMillis()).
		Set("Status", job.Status).
		Set("Data", job.DataToJSON()).
		Set("Progress", job.Progress).
		Where(sq.Eq{"Id": job.ID, "Status": currentStatus}).ToSql()
	if err != nil {
		return false, errors.Wrap(err, "job_tosql")
	}
	sqlResult, err := jss.GetMaster().Exec(query, args...)
	if err != nil {
		return false, errors.Wrap(err, "failed to update Job")
	}

	rows, err := sqlResult.RowsAffected()

	if err != nil {
		return false, errors.Wrap(err, "unable to get rows affected")
	}

	if rows != 1 {
		return false, nil
	}

	return true, nil
}

func (jss SQLJobStore) UpdateStatus(id string, status string) (*model.Job, error) {
	job := &model.Job{
		ID:             id,
		Status:         status,
		LastActivityAt: model.GetMillis(),
	}

	if _, err := jss.GetMaster().UpdateColumns(func(col *gorp.ColumnMap) bool {
		return col.ColumnName == "Status" || col.ColumnName == "LastActivityAt"
	}, job); err != nil {
		return nil, errors.Wrapf(err, "failed to update Job with id=%s", id)
	}

	return job, nil
}

func (jss SQLJobStore) UpdateStatusOptimistically(id string, currentStatus string, newStatus string) (bool, error) {
	builder := jss.getQueryBuilder().
		Update("Jobs").
		Set("LastActivityAt", model.GetMillis()).
		Set("Status", newStatus).
		Where(sq.Eq{"Id": id, "Status": currentStatus})

	if newStatus == model.JobStatusInProgress {
		builder = builder.Set("StartAt", model.GetMillis())
	}
	query, args, err := builder.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "job_tosql")
	}

	sqlResult, err := jss.GetMaster().Exec(query, args...)
	if err != nil {
		return false, errors.Wrapf(err, "failed to update Job with id=%s", id)
	}
	rows, err := sqlResult.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "unable to get rows affected")
	}
	if rows != 1 {
		return false, nil
	}

	return true, nil
}

func (jss SQLJobStore) Get(id string) (*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Id": id}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}
	var status *model.Job
	if err = jss.GetReplica().SelectOne(&status, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Job", id)
		}
		return nil, errors.Wrapf(err, "failed to get Job with id=%s", id)
	}
	return status, nil
}

func (jss SQLJobStore) GetAllPage(offset int, limit int) ([]*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		OrderBy("CreateAt DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}

	var statuses []*model.Job
	if _, err = jss.GetReplica().Select(&statuses, query, args...); err != nil {
		return nil, errors.Wrap(err, "failed to find Jobs")
	}
	return statuses, nil
}

func (jss SQLJobStore) GetAllByTypesPage(jobTypes []string, offset int, limit int) ([]*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Type": jobTypes}).
		OrderBy("CreateAt DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}

	var jobs []*model.Job
	if _, err = jss.GetReplica().Select(&jobs, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Jobs with types")
	}
	return jobs, nil
}

func (jss SQLJobStore) GetAllByType(jobType string) ([]*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Type": jobType}).
		OrderBy("CreateAt DESC").ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}
	var statuses []*model.Job
	if _, err = jss.GetReplica().Select(&statuses, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Jobs with type=%s", jobType)
	}
	return statuses, nil
}

func (jss SQLJobStore) GetAllByTypePage(jobType string, offset int, limit int) ([]*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Type": jobType}).
		OrderBy("CreateAt DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}

	var statuses []*model.Job
	if _, err = jss.GetReplica().Select(&statuses, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Jobs with type=%s", jobType)
	}
	return statuses, nil
}

func (jss SQLJobStore) GetAllByStatus(status string) ([]*model.Job, error) {
	var statuses []*model.Job
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Status": status}).
		OrderBy("CreateAt ASC").ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}

	if _, err = jss.GetReplica().Select(&statuses, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find Jobs with status=%s", status)
	}
	return statuses, nil
}

func (jss SQLJobStore) GetNewestJobByStatusAndType(status string, jobType string) (*model.Job, error) {
	return jss.GetNewestJobByStatusesAndType([]string{status}, jobType)
}

func (jss SQLJobStore) GetNewestJobByStatusesAndType(status []string, jobType string) (*model.Job, error) {
	query, args, err := jss.getQueryBuilder().
		Select("*").
		From("Jobs").
		Where(sq.Eq{"Status": status, "Type": jobType}).
		OrderBy("CreateAt DESC").
		Limit(1).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "job_tosql")
	}

	var job *model.Job
	if err = jss.GetReplica().SelectOne(&job, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, store.NewErrNotFound("Job", fmt.Sprintf("<status, type>=<%s, %s>", strings.Join(status, ","), jobType))
		}
		return nil, errors.Wrapf(err, "failed to find Job with statuses=%s and type=%s", strings.Join(status, ","), jobType)
	}
	return job, nil
}

func (jss SQLJobStore) GetCountByStatusAndType(status string, jobType string) (int64, error) {
	query, args, err := jss.getQueryBuilder().
		Select("COUNT(*)").
		From("Jobs").
		Where(sq.Eq{"Status": status, "Type": jobType}).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "job_tosql")
	}
	count, err := jss.GetReplica().SelectInt(query, args...)
	if err != nil {
		return int64(0), errors.Wrapf(err, "failed to count Jobs with status=%s and type=%s", status, jobType)
	}
	return count, nil
}

func (jss SQLJobStore) Delete(id string) (string, error) {
	query, args, err := jss.getQueryBuilder().
		Delete("Jobs").
		Where(sq.Eq{"Id": id}).ToSql()
	if err != nil {
		return "", errors.Wrap(err, "job_tosql")
	}

	if _, err = jss.GetMaster().Exec(query, args...); err != nil {
		return "", errors.Wrapf(err, "failed to delete Job with id=%s", id)
	}
	return id, nil
}
