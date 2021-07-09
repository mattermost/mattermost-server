// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bleveengine

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/blevesearch/bleve"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/searchengine"
	"github.com/mattermost/mattermost-server/v5/store/searchlayer"
	"github.com/mattermost/mattermost-server/v5/store/searchtest"
	"github.com/mattermost/mattermost-server/v5/store/sqlstore"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
	"github.com/mattermost/mattermost-server/v5/testlib"
)

type BleveEngineTestSuite struct {
	suite.Suite

	SQLSettings  *model.SQLSettings
	SQLStore     *sqlstore.SQLStore
	SearchEngine *searchengine.Broker
	Store        *searchlayer.SearchStore
	BleveEngine  *BleveEngine
	IndexDir     string
}

func TestBleveEngineTestSuite(t *testing.T) {
	suite.Run(t, new(BleveEngineTestSuite))
}

func (s *BleveEngineTestSuite) setupIndexes() {
	indexDir, err := ioutil.TempDir("", "mmbleve")
	if err != nil {
		s.Require().FailNow("Cannot setup bleveengine tests: %s", err.Error())
	}
	s.IndexDir = indexDir
}

func (s *BleveEngineTestSuite) setupStore() {
	driverName := os.Getenv("MM_SQLSETTINGS_DRIVERNAME")
	if driverName == "" {
		driverName = model.DatabaseDriverPostgres
	}
	s.SQLSettings = storetest.MakeSQLSettings(driverName, false)
	s.SQLStore = sqlstore.New(*s.SQLSettings, nil)

	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.BleveSettings.EnableIndexing = model.NewBool(true)
	cfg.BleveSettings.EnableSearching = model.NewBool(true)
	cfg.BleveSettings.EnableAutocomplete = model.NewBool(true)
	cfg.BleveSettings.IndexDir = model.NewString(s.IndexDir)
	cfg.SQLSettings.DisableDatabaseSearch = model.NewBool(true)

	s.SearchEngine = searchengine.NewBroker(cfg, nil)
	s.Store = searchlayer.NewSearchLayer(&testlib.TestStore{Store: s.SQLStore}, s.SearchEngine, cfg)

	s.BleveEngine = NewBleveEngine(cfg, nil)
	s.BleveEngine.indexSync = true
	s.SearchEngine.RegisterBleveEngine(s.BleveEngine)
	if err := s.BleveEngine.Start(); err != nil {
		s.Require().FailNow("Cannot start bleveengine: %s", err.Error())
	}
}

func (s *BleveEngineTestSuite) SetupSuite() {
	s.setupIndexes()
	s.setupStore()
}

func (s *BleveEngineTestSuite) TearDownSuite() {
	os.RemoveAll(s.IndexDir)
	s.SQLStore.Close()
	storetest.CleanupSQLSettings(s.SQLSettings)
}

func (s *BleveEngineTestSuite) TestBleveSearchStoreTests() {
	searchTestEngine := &searchtest.SearchTestEngine{
		Driver: searchtest.EngineBleve,
	}

	s.Run("TestSearchChannelStore", func() {
		searchtest.TestSearchChannelStore(s.T(), s.Store, searchTestEngine)
	})

	s.Run("TestSearchUserStore", func() {
		searchtest.TestSearchUserStore(s.T(), s.Store, searchTestEngine)
	})

	s.Run("TestSearchPostStore", func() {
		searchtest.TestSearchPostStore(s.T(), s.Store, searchTestEngine)
	})

	s.Run("TestSearchFileInfoStore", func() {
		searchtest.TestSearchFileInfoStore(s.T(), s.Store, searchTestEngine)
	})
}

func (s *BleveEngineTestSuite) TestDeleteChannelPosts() {
	s.Run("Should remove all the posts that belongs to a channel", func() {
		s.BleveEngine.PurgeIndexes()
		teamID := model.NewID()
		userID := model.NewID()
		channelID := model.NewID()
		channelToAvoidID := model.NewID()
		posts := make([]*model.Post, 0)
		for i := 0; i < 10; i++ {
			post := createPost(userID, channelID)
			appErr := s.SearchEngine.BleveEngine.IndexPost(post, teamID)
			require.Nil(s.T(), appErr)
			posts = append(posts, post)
		}
		postToAvoid := createPost(userID, channelToAvoidID)
		appErr := s.SearchEngine.BleveEngine.IndexPost(postToAvoid, teamID)
		require.Nil(s.T(), appErr)

		s.SearchEngine.BleveEngine.DeleteChannelPosts(channelID)

		doc, err := s.BleveEngine.PostIndex.Document(postToAvoid.ID)
		require.NoError(s.T(), err)
		require.Equal(s.T(), postToAvoid.ID, doc.ID)
		numberDocs, err := s.BleveEngine.PostIndex.DocCount()
		require.NoError(s.T(), err)
		require.Equal(s.T(), 1, int(numberDocs))
	})

	s.Run("Shouldn't do anything if there is not posts for the selected channel", func() {
		s.BleveEngine.PurgeIndexes()
		teamID := model.NewID()
		userID := model.NewID()
		channelID := model.NewID()
		channelToDeleteID := model.NewID()
		post := createPost(userID, channelID)
		appErr := s.SearchEngine.BleveEngine.IndexPost(post, teamID)
		require.Nil(s.T(), appErr)

		s.SearchEngine.BleveEngine.DeleteChannelPosts(channelToDeleteID)

		_, err := s.BleveEngine.PostIndex.Document(post.ID)
		require.NoError(s.T(), err)
		numberDocs, err := s.BleveEngine.PostIndex.DocCount()
		require.NoError(s.T(), err)
		require.Equal(s.T(), 1, int(numberDocs))
	})
}

func (s *BleveEngineTestSuite) TestDeleteUserPosts() {
	s.Run("Should remove all the posts that belongs to a user", func() {
		s.BleveEngine.PurgeIndexes()
		teamID := model.NewID()
		userID := model.NewID()
		userToAvoidID := model.NewID()
		channelID := model.NewID()
		posts := make([]*model.Post, 0)
		for i := 0; i < 10; i++ {
			post := createPost(userID, channelID)
			appErr := s.SearchEngine.BleveEngine.IndexPost(post, teamID)
			require.Nil(s.T(), appErr)
			posts = append(posts, post)
		}
		postToAvoid := createPost(userToAvoidID, channelID)
		appErr := s.SearchEngine.BleveEngine.IndexPost(postToAvoid, teamID)
		require.Nil(s.T(), appErr)

		s.SearchEngine.BleveEngine.DeleteUserPosts(userID)

		doc, err := s.BleveEngine.PostIndex.Document(postToAvoid.ID)
		require.NoError(s.T(), err)
		require.Equal(s.T(), postToAvoid.ID, doc.ID)
		numberDocs, err := s.BleveEngine.PostIndex.DocCount()
		require.NoError(s.T(), err)
		require.Equal(s.T(), 1, int(numberDocs))
	})

	s.Run("Shouldn't do anything if there is not posts for the selected user", func() {
		s.BleveEngine.PurgeIndexes()
		teamID := model.NewID()
		userID := model.NewID()
		userToDeleteID := model.NewID()
		channelID := model.NewID()
		post := createPost(userID, channelID)
		appErr := s.SearchEngine.BleveEngine.IndexPost(post, teamID)
		require.Nil(s.T(), appErr)

		s.SearchEngine.BleveEngine.DeleteUserPosts(userToDeleteID)

		_, err := s.BleveEngine.PostIndex.Document(post.ID)
		require.NoError(s.T(), err)
		numberDocs, err := s.BleveEngine.PostIndex.DocCount()
		require.NoError(s.T(), err)
		require.Equal(s.T(), 1, int(numberDocs))
	})
}

func (s *BleveEngineTestSuite) TestDeletePosts() {
	s.BleveEngine.PurgeIndexes()
	teamID := model.NewID()
	userID := model.NewID()
	userToAvoidID := model.NewID()
	channelID := model.NewID()
	posts := make([]*model.Post, 0)
	for i := 0; i < 10; i++ {
		post := createPost(userID, channelID)
		appErr := s.SearchEngine.BleveEngine.IndexPost(post, teamID)
		require.Nil(s.T(), appErr)
		posts = append(posts, post)
	}
	postToAvoid := createPost(userToAvoidID, channelID)
	appErr := s.SearchEngine.BleveEngine.IndexPost(postToAvoid, teamID)
	require.Nil(s.T(), appErr)

	query := bleve.NewTermQuery(userID)
	query.SetField("UserId")
	search := bleve.NewSearchRequest(query)
	count, err := s.BleveEngine.deletePosts(search, 1)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 10, int(count))

	doc, err := s.BleveEngine.PostIndex.Document(postToAvoid.ID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), postToAvoid.ID, doc.ID)
	numberDocs, err := s.BleveEngine.PostIndex.DocCount()
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, int(numberDocs))
}
