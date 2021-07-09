// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine/mocks"
	filesStoreMocks "github.com/mattermost/mattermost-server/v5/shared/filestore/mocks"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func TestGeneratePublicLinkHash(t *testing.T) {
	filename1 := model.NewID() + "/" + model.NewRandomString(16) + ".txt"
	filename2 := model.NewID() + "/" + model.NewRandomString(16) + ".txt"
	salt1 := model.NewRandomString(32)
	salt2 := model.NewRandomString(32)

	hash1 := GeneratePublicLinkHash(filename1, salt1)
	hash2 := GeneratePublicLinkHash(filename2, salt1)
	hash3 := GeneratePublicLinkHash(filename1, salt2)

	hash := GeneratePublicLinkHash(filename1, salt1)
	assert.Equal(t, hash, hash1, "hash should be equal for the same file name and salt")

	assert.NotEqual(t, hash1, hash2, "hashes for different files should not be equal")

	assert.NotEqual(t, hash1, hash3, "hashes for the same file with different salts should not be equal")
}

func TestDoUploadFile(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	teamID := model.NewID()
	channelID := model.NewID()
	userID := model.NewID()
	filename := "test"
	data := []byte("abcd")

	info1, err := th.App.DoUploadFile(th.Context, time.Date(2007, 2, 4, 1, 2, 3, 4, time.Local), teamID, channelID, userID, filename, data)
	require.Nil(t, err, "DoUploadFile should succeed with valid data")
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info1.ID)
		th.App.RemoveFile(info1.Path)
	}()

	value := fmt.Sprintf("20070204/teams/%v/channels/%v/users/%v/%v/%v", teamID, channelID, userID, info1.ID, filename)
	assert.Equal(t, value, info1.Path, "stored file at incorrect path")

	info2, err := th.App.DoUploadFile(th.Context, time.Date(2007, 2, 4, 1, 2, 3, 4, time.Local), teamID, channelID, userID, filename, data)
	require.Nil(t, err, "DoUploadFile should succeed with valid data")
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info2.ID)
		th.App.RemoveFile(info2.Path)
	}()

	value = fmt.Sprintf("20070204/teams/%v/channels/%v/users/%v/%v/%v", teamID, channelID, userID, info2.ID, filename)
	assert.Equal(t, value, info2.Path, "stored file at incorrect path")

	info3, err := th.App.DoUploadFile(th.Context, time.Date(2008, 3, 5, 1, 2, 3, 4, time.Local), teamID, channelID, userID, filename, data)
	require.Nil(t, err, "DoUploadFile should succeed with valid data")
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info3.ID)
		th.App.RemoveFile(info3.Path)
	}()

	value = fmt.Sprintf("20080305/teams/%v/channels/%v/users/%v/%v/%v", teamID, channelID, userID, info3.ID, filename)
	assert.Equal(t, value, info3.Path, "stored file at incorrect path")

	info4, err := th.App.DoUploadFile(th.Context, time.Date(2009, 3, 5, 1, 2, 3, 4, time.Local), "../../"+teamID, "../../"+channelID, "../../"+userID, "../../"+filename, data)
	require.Nil(t, err, "DoUploadFile should succeed with valid data")
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info4.ID)
		th.App.RemoveFile(info4.Path)
	}()

	value = fmt.Sprintf("20090305/teams/%v/channels/%v/users/%v/%v/%v", teamID, channelID, userID, info4.ID, filename)
	assert.Equal(t, value, info4.Path, "stored file at incorrect path")
}

func TestUploadFile(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channelID := th.BasicChannel.ID
	filename := "test"
	data := []byte("abcd")

	info1, err := th.App.UploadFile(th.Context, data, "wrong", filename)
	require.NotNil(t, err, "Wrong Channel ID.")
	require.Nil(t, info1, "Channel ID does not exist.")

	info1, err = th.App.UploadFile(th.Context, data, "", filename)
	require.Nil(t, err, "empty channel IDs should be valid")
	require.NotNil(t, info1)

	info1, err = th.App.UploadFile(th.Context, data, channelID, filename)
	require.Nil(t, err, "UploadFile should succeed with valid data")
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info1.ID)
		th.App.RemoveFile(info1.Path)
	}()

	value := fmt.Sprintf("%v/teams/noteam/channels/%v/users/nouser/%v/%v",
		time.Now().Format("20060102"), channelID, info1.ID, filename)
	assert.Equal(t, value, info1.Path, "Stored file at incorrect path")
}

func TestParseOldFilenames(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	fileID := model.NewID()

	tests := []struct {
		description string
		filenames   []string
		channelID   string
		userID      string
		expected    [][]string
	}{
		{
			description: "Empty input should result in empty output",
			filenames:   []string{},
			channelID:   th.BasicChannel.ID,
			userID:      th.BasicUser.ID,
			expected:    [][]string{},
		},
		{
			description: "Filename with invalid format should not parse",
			filenames:   []string{"/path/to/some/file.png"},
			channelID:   th.BasicChannel.ID,
			userID:      th.BasicUser.ID,
			expected:    [][]string{},
		},
		{
			description: "ChannelId in Filename should not match",
			filenames: []string{
				fmt.Sprintf("/%v/%v/%v/file.png", model.NewID(), th.BasicUser.ID, fileID),
			},
			channelID: th.BasicChannel.ID,
			userID:    th.BasicUser.ID,
			expected:  [][]string{},
		},
		{
			description: "UserId in Filename should not match",
			filenames: []string{
				fmt.Sprintf("/%v/%v/%v/file.png", th.BasicChannel.ID, model.NewID(), fileID),
			},
			channelID: th.BasicChannel.ID,
			userID:    th.BasicUser.ID,
			expected:  [][]string{},
		},
		{
			description: "../ in filename should not parse",
			filenames: []string{
				fmt.Sprintf("/%v/%v/%v/../../../file.png", th.BasicChannel.ID, th.BasicUser.ID, fileID),
			},
			channelID: th.BasicChannel.ID,
			userID:    th.BasicUser.ID,
			expected:  [][]string{},
		},
		{
			description: "Should only parse valid filenames",
			filenames: []string{
				fmt.Sprintf("/%v/%v/%v/../otherfile.png", th.BasicChannel.ID, th.BasicUser.ID, fileID),
				fmt.Sprintf("/%v/%v/%v/file.png", th.BasicChannel.ID, th.BasicUser.ID, fileID),
			},
			channelID: th.BasicChannel.ID,
			userID:    th.BasicUser.ID,
			expected: [][]string{
				{
					th.BasicChannel.ID,
					th.BasicUser.ID,
					fileID,
					"file.png",
				},
			},
		},
		{
			description: "Valid Filename should parse",
			filenames: []string{
				fmt.Sprintf("/%v/%v/%v/file.png", th.BasicChannel.ID, th.BasicUser.ID, fileID),
			},
			channelID: th.BasicChannel.ID,
			userID:    th.BasicUser.ID,
			expected: [][]string{
				{
					th.BasicChannel.ID,
					th.BasicUser.ID,
					fileID,
					"file.png",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(tt *testing.T) {
			result := parseOldFilenames(test.filenames, test.channelID, test.userID)
			require.Equal(tt, result, test.expected)
		})
	}
}

func TestGetInfoForFilename(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := th.BasicPost
	teamID := th.BasicTeam.ID

	info := th.App.getInfoForFilename(post, teamID, post.ChannelID, post.UserID, "someid", "somefile.png")
	assert.Nil(t, info, "Test non-existent file")
}

func TestFindTeamIDForFilename(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	teamID := th.App.findTeamIDForFilename(th.BasicPost, "someid", "somefile.png")
	assert.Equal(t, th.BasicTeam.ID, teamID)

	_, err := th.App.CreateTeamWithUser(th.Context, &model.Team{Email: th.BasicUser.Email, Name: "zz" + model.NewID(), DisplayName: "Joram's Test Team", Type: model.TeamOpen}, th.BasicUser.ID)
	require.Nil(t, err)

	teamID = th.App.findTeamIDForFilename(th.BasicPost, "someid", "somefile.png")
	assert.Equal(t, "", teamID)
}

func TestMigrateFilenamesToFileInfos(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := th.BasicPost
	infos := th.App.MigrateFilenamesToFileInfos(post)
	assert.Equal(t, 0, len(infos))

	post.Filenames = []string{fmt.Sprintf("/%v/%v/%v/blargh.png", th.BasicChannel.ID, th.BasicUser.ID, "someid")}
	infos = th.App.MigrateFilenamesToFileInfos(post)
	assert.Equal(t, 0, len(infos))

	path, _ := fileutils.FindDir("tests")
	file, fileErr := os.Open(filepath.Join(path, "test.png"))
	require.NoError(t, fileErr)
	defer file.Close()

	fileID := model.NewID()
	fpath := fmt.Sprintf("/teams/%v/channels/%v/users/%v/%v/test.png", th.BasicTeam.ID, th.BasicChannel.ID, th.BasicUser.ID, fileID)
	_, err := th.App.WriteFile(file, fpath)
	require.Nil(t, err)
	rpost, err := th.App.CreatePost(th.Context, &model.Post{UserID: th.BasicUser.ID, ChannelID: th.BasicChannel.ID, Filenames: []string{fmt.Sprintf("/%v/%v/%v/test.png", th.BasicChannel.ID, th.BasicUser.ID, fileID)}}, th.BasicChannel, false, true)
	require.Nil(t, err)

	infos = th.App.MigrateFilenamesToFileInfos(rpost)
	assert.Equal(t, 1, len(infos))

	rpost, err = th.App.CreatePost(th.Context, &model.Post{UserID: th.BasicUser.ID, ChannelID: th.BasicChannel.ID, Filenames: []string{fmt.Sprintf("/%v/%v/%v/../../test.png", th.BasicChannel.ID, th.BasicUser.ID, fileID)}}, th.BasicChannel, false, true)
	require.Nil(t, err)

	infos = th.App.MigrateFilenamesToFileInfos(rpost)
	assert.Equal(t, 0, len(infos))
}

func TestCreateZipFileAndAddFiles(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	mockBackend := filesStoreMocks.FileBackend{}
	mockBackend.On("WriteFile", mock.Anything, "directory-to-heaven/zip-file-name-to-heaven.zip").Return(int64(666), errors.New("only those who dare to fail greatly can ever achieve greatly"))

	err := th.App.CreateZipFileAndAddFiles(&mockBackend, []model.FileData{}, "zip-file-name-to-heaven.zip", "directory-to-heaven")

	require.Error(t, err)
	require.Equal(t, err.Error(), "only those who dare to fail greatly can ever achieve greatly")

	mockBackend = filesStoreMocks.FileBackend{}
	mockBackend.On("WriteFile", mock.Anything, "directory-to-heaven/zip-file-name-to-heaven.zip").Return(int64(666), nil)
	err = th.App.CreateZipFileAndAddFiles(&mockBackend, []model.FileData{}, "zip-file-name-to-heaven.zip", "directory-to-heaven")
	require.NoError(t, err)
}

func TestCopyFileInfos(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	teamID := model.NewID()
	channelID := model.NewID()
	userID := model.NewID()
	filename := "test"
	data := []byte("abcd")

	info1, err := th.App.DoUploadFile(th.Context, time.Date(2007, 2, 4, 1, 2, 3, 4, time.Local), teamID, channelID, userID, filename, data)
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info1.ID)
		th.App.RemoveFile(info1.Path)
	}()

	infoIDs, err := th.App.CopyFileInfos(userID, []string{info1.ID})
	require.Nil(t, err)

	info2, err := th.App.GetFileInfo(infoIDs[0])
	require.Nil(t, err)
	defer func() {
		th.App.Srv().Store.FileInfo().PermanentDelete(info2.ID)
		th.App.RemoveFile(info2.Path)
	}()

	assert.NotEqual(t, info1.ID, info2.ID, "should not be equal")
	assert.Equal(t, info2.PostID, "", "should be empty string")
}

func TestGenerateThumbnailImage(t *testing.T) {
	t.Run("test generating thumbnail image", func(t *testing.T) {
		// given
		th := Setup(t)
		defer th.TearDown()
		img := createDummyImage()
		dataPath, _ := fileutils.FindDir("data")
		thumbailName := "thumb.jpg"
		thumbnailPath := filepath.Join(dataPath, thumbailName)

		// when
		th.App.generateThumbnailImage(img, thumbailName)
		defer os.Remove(thumbnailPath)

		// then
		outputImage, err := os.Stat(thumbnailPath)
		assert.NoError(t, err)
		assert.Equal(t, int64(957), outputImage.Size())
	})
}

func createDummyImage() *image.RGBA {
	width := 200
	height := 100
	upperLeftCorner := image.Point{0, 0}
	lowerRightCorner := image.Point{width, height}
	return image.NewRGBA(image.Rectangle{upperLeftCorner, lowerRightCorner})
}

func TestSearchFilesInTeamForUser(t *testing.T) {
	perPage := 5
	searchTerm := "searchTerm"

	setup := func(t *testing.T, enableElasticsearch bool) (*TestHelper, []*model.FileInfo) {
		th := Setup(t).InitBasic()

		fileInfos := make([]*model.FileInfo, 7)
		for i := 0; i < cap(fileInfos); i++ {
			fileInfo, err := th.App.Srv().Store.FileInfo().Save(&model.FileInfo{
				CreatorID: th.BasicUser.ID,
				PostID:    th.BasicPost.ID,
				Name:      searchTerm,
				Path:      searchTerm,
				Extension: "jpg",
				MimeType:  "image/jpeg",
			})
			time.Sleep(1 * time.Millisecond)

			require.NoError(t, err)

			fileInfos[i] = fileInfo
		}

		if enableElasticsearch {
			th.App.Srv().SetLicense(model.NewTestLicense("elastic_search"))

			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ElasticsearchSettings.EnableIndexing = true
				*cfg.ElasticsearchSettings.EnableSearching = true
			})
		} else {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.ElasticsearchSettings.EnableSearching = false
			})
		}

		return th, fileInfos
	}

	t.Run("should return everything as first page of fileInfos from database", func(t *testing.T) {
		th, fileInfos := setup(t, false)
		defer th.TearDown()

		page := 0

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		require.NotNil(t, results)
		assert.Equal(t, []string{
			fileInfos[6].ID,
			fileInfos[5].ID,
			fileInfos[4].ID,
			fileInfos[3].ID,
			fileInfos[2].ID,
			fileInfos[1].ID,
			fileInfos[0].ID,
		}, results.Order)
	})

	t.Run("should not return later pages of fileInfos from database", func(t *testing.T) {
		th, _ := setup(t, false)
		defer th.TearDown()

		page := 1

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		require.NotNil(t, results)
		assert.Equal(t, []string{}, results.Order)
	})

	t.Run("should return first page of fileInfos from ElasticSearch", func(t *testing.T) {
		th, fileInfos := setup(t, true)
		defer th.TearDown()

		page := 0
		resultsPage := []string{
			fileInfos[6].ID,
			fileInfos[5].ID,
			fileInfos[4].ID,
			fileInfos[3].ID,
			fileInfos[2].ID,
		}

		es := &mocks.SearchEngineInterface{}
		es.On("SearchFiles", mock.Anything, mock.Anything, page, perPage).Return(resultsPage, nil)
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		require.NotNil(t, results)
		assert.Equal(t, resultsPage, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should return later pages of fileInfos from ElasticSearch", func(t *testing.T) {
		th, fileInfos := setup(t, true)
		defer th.TearDown()

		page := 1
		resultsPage := []string{
			fileInfos[1].ID,
			fileInfos[0].ID,
		}

		es := &mocks.SearchEngineInterface{}
		es.On("SearchFiles", mock.Anything, mock.Anything, page, perPage).Return(resultsPage, nil)
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		require.NotNil(t, results)
		assert.Equal(t, resultsPage, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should fall back to database if ElasticSearch fails on first page", func(t *testing.T) {
		th, fileInfos := setup(t, true)
		defer th.TearDown()

		page := 0

		es := &mocks.SearchEngineInterface{}
		es.On("SearchFiles", mock.Anything, mock.Anything, page, perPage).Return(nil, &model.AppError{})
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		require.NotNil(t, results)
		assert.Equal(t, []string{
			fileInfos[6].ID,
			fileInfos[5].ID,
			fileInfos[4].ID,
			fileInfos[3].ID,
			fileInfos[2].ID,
			fileInfos[1].ID,
			fileInfos[0].ID,
		}, results.Order)
		es.AssertExpectations(t)
	})

	t.Run("should return nothing if ElasticSearch fails on later pages", func(t *testing.T) {
		th, _ := setup(t, true)
		defer th.TearDown()

		page := 1

		es := &mocks.SearchEngineInterface{}
		es.On("SearchFiles", mock.Anything, mock.Anything, page, perPage).Return(nil, &model.AppError{})
		es.On("GetName").Return("mock")
		es.On("Start").Return(nil).Maybe()
		es.On("IsActive").Return(true)
		es.On("IsSearchEnabled").Return(true)
		th.App.Srv().SearchEngine.ElasticsearchEngine = es
		defer func() {
			th.App.Srv().SearchEngine.ElasticsearchEngine = nil
		}()

		results, err := th.App.SearchFilesInTeamForUser(th.Context, searchTerm, th.BasicUser.ID, th.BasicTeam.ID, false, false, 0, page, perPage)

		require.Nil(t, err)
		assert.Equal(t, []string{}, results.Order)
		es.AssertExpectations(t)
	})
}
