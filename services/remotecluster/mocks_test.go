// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package remotecluster

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/mattermost/mattermost-server/v5/einterfaces"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/mattermost/mattermost-server/v5/store/storetest/mocks"
)

type mockServer struct {
	remotes []*model.RemoteCluster
	logger  *mockLogger
	user    *model.User
}

func newMockServer(t *testing.T, remotes []*model.RemoteCluster) *mockServer {
	return &mockServer{
		remotes: remotes,
		logger:  &mockLogger{t: t},
	}
}

func (ms *mockServer) SetUser(user *model.User) {
	ms.user = user
}

func (ms *mockServer) Config() *model.Config                                  { return nil }
func (ms *mockServer) GetMetrics() einterfaces.MetricsInterface               { return nil }
func (ms *mockServer) IsLeader() bool                                         { return true }
func (ms *mockServer) AddClusterLeaderChangedListener(listener func()) string { return model.NewID() }
func (ms *mockServer) RemoveClusterLeaderChangedListener(id string)           {}
func (ms *mockServer) GetLogger() mlog.LoggerIFace {
	return ms.logger
}
func (ms *mockServer) GetStore() store.Store {
	anyQueryFilter := mock.MatchedBy(func(filter model.RemoteClusterQueryFilter) bool {
		return true
	})
	anyUserID := mock.AnythingOfType("string")

	remoteClusterStoreMock := &mocks.RemoteClusterStore{}
	remoteClusterStoreMock.On("GetByTopic", "share").Return(ms.remotes, nil)
	remoteClusterStoreMock.On("GetAll", anyQueryFilter).Return(ms.remotes, nil)

	userStoreMock := &mocks.UserStore{}
	userStoreMock.On("Get", context.Background(), anyUserID).Return(ms.user, nil)

	storeMock := &mocks.Store{}
	storeMock.On("RemoteCluster").Return(remoteClusterStoreMock)
	storeMock.On("User").Return(userStoreMock)
	return storeMock
}
func (ms *mockServer) Shutdown() { ms.logger.Shutdown() }

type mockLogger struct {
	t   *testing.T
	mux sync.Mutex
}

func (ml *mockLogger) IsLevelEnabled(level mlog.LogLevel) bool {
	return true
}
func (ml *mockLogger) Debug(s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log("debug", s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Info(s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log("info", s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Warn(s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log("warn", s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Error(s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log("error", s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Critical(s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log("crit", s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Log(level mlog.LogLevel, s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log(level.Name, s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) LogM(levels []mlog.LogLevel, s string, flds ...mlog.Field) {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	if ml.t != nil {
		ml.t.Log(levelsToString(levels), s, fieldsToStrings(flds))
	}
}
func (ml *mockLogger) Shutdown() {
	ml.mux.Lock()
	defer ml.mux.Unlock()
	ml.t = nil
}

func levelsToString(levels []mlog.LogLevel) string {
	sb := strings.Builder{}
	for _, l := range levels {
		sb.WriteString(l.Name)
		sb.WriteString(",")
	}
	return sb.String()
}

func fieldsToStrings(fields []mlog.Field) []string {
	encoder := zapcore.NewMapObjectEncoder()
	for _, zapField := range fields {
		zapField.AddTo(encoder)
	}

	var result []string
	for k, v := range encoder.Fields {
		result = append(result, fmt.Sprintf("%s:%v", k, v))
	}
	return result
}
