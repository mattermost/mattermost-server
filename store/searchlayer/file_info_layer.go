// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package searchlayer

import (
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
)

type SearchFileInfoStore struct {
	store.FileInfoStore
	rootStore *SearchStore
}

func (s SearchFileInfoStore) indexFile(file *model.FileInfo) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if file.PostID == "" {
					return
				}
				post, postErr := s.rootStore.Post().GetSingle(file.PostID, false)
				if postErr != nil {
					mlog.Error("Couldn't get post for file for SearchEngine indexing.", mlog.String("post_id", file.PostID), mlog.String("search_engine", engineCopy.GetName()), mlog.String("file_info_id", file.ID), mlog.Err(postErr))
					return
				}

				if err := engineCopy.IndexFile(file, post.ChannelID); err != nil {
					mlog.Error("Encountered error indexing file", mlog.String("file_info_id", file.ID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Indexed file in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("file_info_id", file.ID))
			})
		}
	}
}

func (s SearchFileInfoStore) deleteFileIndex(fileID string) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteFile(fileID); err != nil {
					mlog.Error("Encountered error deleting file", mlog.String("file_info_id", fileID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed file from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("file_info_id", fileID))
			})
		}
	}
}

func (s SearchFileInfoStore) deleteFileIndexForUser(userID string) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteUserFiles(userID); err != nil {
					mlog.Error("Encountered error deleting files for user", mlog.String("user_id", userID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed user's files from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("user_id", userID))
			})
		}
	}
}

func (s SearchFileInfoStore) deleteFileIndexForPost(postID string) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeletePostFiles(postID); err != nil {
					mlog.Error("Encountered error deleting files for post", mlog.String("post_id", postID), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed post's files from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.String("post_id", postID))
			})
		}
	}
}

func (s SearchFileInfoStore) deleteFileIndexBatch(endTime, limit int64) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsIndexingEnabled() {
			runIndexFn(engine, func(engineCopy searchengine.SearchEngineInterface) {
				if err := engineCopy.DeleteFilesBatch(endTime, limit); err != nil {
					mlog.Error("Encountered error deleting a batch of files", mlog.Int64("limit", limit), mlog.Int64("end_time", endTime), mlog.String("search_engine", engineCopy.GetName()), mlog.Err(err))
					return
				}
				mlog.Debug("Removed batch of files from the index in search engine", mlog.String("search_engine", engineCopy.GetName()), mlog.Int64("end_time", endTime), mlog.Int64("limit", limit))
			})
		}
	}
}

func (s SearchFileInfoStore) Save(info *model.FileInfo) (*model.FileInfo, error) {
	nfile, err := s.FileInfoStore.Save(info)
	if err == nil {
		s.indexFile(nfile)
	}
	return nfile, err
}

func (s SearchFileInfoStore) SetContent(fileID, content string) error {
	err := s.FileInfoStore.SetContent(fileID, content)
	if err == nil {
		nfile, err2 := s.FileInfoStore.GetFromMaster(fileID)
		if err2 == nil {
			nfile.Content = content
			s.indexFile(nfile)
		}
	}
	return err
}

func (s SearchFileInfoStore) AttachToPost(fileID, postID, creatorID string) error {
	err := s.FileInfoStore.AttachToPost(fileID, postID, creatorID)
	if err == nil {
		nFileInfo, err2 := s.FileInfoStore.GetFromMaster(fileID)
		if err2 == nil {
			s.indexFile(nFileInfo)
		}
	}
	return err
}

func (s SearchFileInfoStore) DeleteForPost(postID string) (string, error) {
	result, err := s.FileInfoStore.DeleteForPost(postID)
	if err == nil {
		s.deleteFileIndexForPost(postID)
	}
	return result, err
}

func (s SearchFileInfoStore) PermanentDelete(fileID string) error {
	err := s.FileInfoStore.PermanentDelete(fileID)
	if err == nil {
		s.deleteFileIndex(fileID)
	}
	return err
}

func (s SearchFileInfoStore) PermanentDeleteBatch(endTime int64, limit int64) (int64, error) {
	result, err := s.FileInfoStore.PermanentDeleteBatch(endTime, limit)
	if err == nil {
		s.deleteFileIndexBatch(endTime, limit)
	}
	return result, err
}

func (s SearchFileInfoStore) PermanentDeleteByUser(userID string) (int64, error) {
	result, err := s.FileInfoStore.PermanentDeleteByUser(userID)
	if err == nil {
		s.deleteFileIndexForUser(userID)
	}
	return result, err
}

func (s SearchFileInfoStore) Search(paramsList []*model.SearchParams, userID, teamID string, page, perPage int) (*model.FileInfoList, error) {
	for _, engine := range s.rootStore.searchEngine.GetActiveEngines() {
		if engine.IsSearchEnabled() {
			userChannels, nErr := s.rootStore.Channel().GetChannels(teamID, userID, paramsList[0].IncludeDeletedChannels, 0)
			if nErr != nil {
				return nil, nErr
			}
			fileIDs, appErr := engine.SearchFiles(userChannels, paramsList, page, perPage)
			if appErr != nil {
				mlog.Error("Encountered error on Search.", mlog.String("search_engine", engine.GetName()), mlog.Err(appErr))
				continue
			}
			mlog.Debug("Using the first available search engine", mlog.String("search_engine", engine.GetName()))

			// Get the files
			filesList := model.NewFileInfoList()
			if len(fileIDs) > 0 {
				files, nErr := s.FileInfoStore.GetByIDs(fileIDs)
				if nErr != nil {
					return nil, nErr
				}
				for _, f := range files {
					filesList.AddFileInfo(f)
					filesList.AddOrder(f.ID)
				}
			}
			return filesList, nil
		}
	}

	if *s.rootStore.getConfig().SqlSettings.DisableDatabaseSearch {
		mlog.Debug("Returning empty results for file Search as the database search is disabled")
		return model.NewFileInfoList(), nil
	}

	mlog.Debug("Using database search because no other search engine is available")
	return s.FileInfoStore.Search(paramsList, userID, teamID, page, perPage)
}
