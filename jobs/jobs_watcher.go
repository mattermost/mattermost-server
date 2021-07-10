// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package jobs

import (
	"math/rand"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// DefaultWatcherPollingInterval is the default polling interval for jobs termination.
// (Defining as `var` rather than `const` allows tests to lower the interval.)
var DefaultWatcherPollingInterval = 15000

type Watcher struct {
	srv     *JobServer
	workers *Workers

	stop            chan struct{}
	stopped         chan struct{}
	pollingInterval int
}

func (srv *JobServer) MakeWatcher(workers *Workers, pollingInterval int) *Watcher {
	return &Watcher{
		stop:            make(chan struct{}),
		stopped:         make(chan struct{}),
		pollingInterval: pollingInterval,
		workers:         workers,
		srv:             srv,
	}
}

func (watcher *Watcher) Start() {
	mlog.Debug("Watcher Started")

	// Delay for some random number of milliseconds before starting to ensure that multiple
	// instances of the jobserver  don't poll at a time too close to each other.
	rand.Seed(time.Now().UTC().UnixNano())
	<-time.After(time.Duration(rand.Intn(watcher.pollingInterval)) * time.Millisecond)

	defer func() {
		mlog.Debug("Watcher Finished")
		close(watcher.stopped)
	}()

	for {
		select {
		case <-watcher.stop:
			mlog.Debug("Watcher: Received stop signal")
			return
		case <-time.After(time.Duration(watcher.pollingInterval) * time.Millisecond):
			watcher.PollAndNotify()
		}
	}
}

func (watcher *Watcher) Stop() {
	mlog.Debug("Watcher Stopping")
	close(watcher.stop)
	<-watcher.stopped
}

func (watcher *Watcher) PollAndNotify() {
	jobs, err := watcher.srv.Store.Job().GetAllByStatus(model.JobStatusPending)
	if err != nil {
		mlog.Error("Error occurred getting all pending statuses.", mlog.Err(err))
		return
	}

	for _, job := range jobs {
		if job.Type == model.JobTypeDataRetention {
			if watcher.workers.DataRetention != nil {
				select {
				case watcher.workers.DataRetention.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeMessageExport {
			if watcher.workers.MessageExport != nil {
				select {
				case watcher.workers.MessageExport.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeElasticsearchPostIndexing {
			if watcher.workers.ElasticsearchIndexing != nil {
				select {
				case watcher.workers.ElasticsearchIndexing.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeElasticsearchPostAggregation {
			if watcher.workers.ElasticsearchAggregation != nil {
				select {
				case watcher.workers.ElasticsearchAggregation.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeBlevePostIndexing {
			if watcher.workers.BleveIndexing != nil {
				select {
				case watcher.workers.BleveIndexing.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeLdapSync {
			if watcher.workers.LdapSync != nil {
				select {
				case watcher.workers.LdapSync.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeMigrations {
			if watcher.workers.Migrations != nil {
				select {
				case watcher.workers.Migrations.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypePlugins {
			if watcher.workers.Plugins != nil {
				select {
				case watcher.workers.Plugins.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeExpiryNotify {
			if watcher.workers.ExpiryNotify != nil {
				select {
				case watcher.workers.ExpiryNotify.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeProductNotices {
			if watcher.workers.ProductNotices != nil {
				select {
				case watcher.workers.ProductNotices.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeActiveUsers {
			if watcher.workers.ActiveUsers != nil {
				select {
				case watcher.workers.ActiveUsers.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeImportProcess {
			if watcher.workers.ImportProcess != nil {
				select {
				case watcher.workers.ImportProcess.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeImportDelete {
			if watcher.workers.ImportDelete != nil {
				select {
				case watcher.workers.ImportDelete.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeExportProcess {
			if watcher.workers.ExportProcess != nil {
				select {
				case watcher.workers.ExportProcess.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeExportDelete {
			if watcher.workers.ExportDelete != nil {
				select {
				case watcher.workers.ExportDelete.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeCloud {
			if watcher.workers.Cloud != nil {
				select {
				case watcher.workers.Cloud.JobChannel() <- *job:
				default:
				}
			}
		} else if job.Type == model.JobTypeResendInvitationEmail {
			if watcher.workers.ResendInvitationEmail != nil {
				select {
				case watcher.workers.ResendInvitationEmail.JobChannel() <- *job:
				default:
				}
			}
		}
	}
}
