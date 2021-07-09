// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"context"
	"sort"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/store"
)

type LocalCacheRoleStore struct {
	store.RoleStore
	rootStore *LocalCacheStore
}

func (s *LocalCacheRoleStore) handleClusterInvalidateRole(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.roleCache.Purge()
	} else {
		s.rootStore.roleCache.Remove(msg.Data)
	}
}

func (s *LocalCacheRoleStore) handleClusterInvalidateRolePermissions(msg *model.ClusterMessage) {
	if msg.Data == ClearCacheMessageData {
		s.rootStore.rolePermissionsCache.Purge()
	} else {
		s.rootStore.rolePermissionsCache.Remove(msg.Data)
	}
}

func (s LocalCacheRoleStore) Save(role *model.Role) (*model.Role, error) {
	if role.Name != "" {
		defer s.rootStore.doInvalidateCacheCluster(s.rootStore.roleCache, role.Name)
		defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)
	}
	return s.RoleStore.Save(role)
}

func (s LocalCacheRoleStore) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role *model.Role
	if err := s.rootStore.doStandardReadCache(s.rootStore.roleCache, name, &role); err == nil {
		return role, nil
	}

	role, err := s.RoleStore.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	s.rootStore.doStandardAddToCache(s.rootStore.roleCache, name, role)
	return role, nil
}

func (s LocalCacheRoleStore) GetByNames(names []string) ([]*model.Role, error) {
	var foundRoles []*model.Role
	var rolesToQuery []string

	for _, roleName := range names {
		var role *model.Role
		if err := s.rootStore.doStandardReadCache(s.rootStore.roleCache, roleName, &role); err == nil {
			foundRoles = append(foundRoles, role)
		} else {
			rolesToQuery = append(rolesToQuery, roleName)
		}
	}

	roles, err := s.RoleStore.GetByNames(rolesToQuery)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		s.rootStore.doStandardAddToCache(s.rootStore.roleCache, role.Name, role)
	}

	return append(foundRoles, roles...), nil
}

func (s LocalCacheRoleStore) Delete(roleID string) (*model.Role, error) {
	role, err := s.RoleStore.Delete(roleID)

	if err == nil {
		s.rootStore.doInvalidateCacheCluster(s.rootStore.roleCache, role.Name)
		defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)
	}
	return role, err
}

func (s LocalCacheRoleStore) PermanentDeleteAll() error {
	defer s.rootStore.roleCache.Purge()
	defer s.rootStore.doClearCacheCluster(s.rootStore.roleCache)
	defer s.rootStore.doClearCacheCluster(s.rootStore.rolePermissionsCache)

	return s.RoleStore.PermanentDeleteAll()
}

func (s LocalCacheRoleStore) ChannelHigherScopedPermissions(roleNames []string) (map[string]*model.RolePermissions, error) {
	sort.Strings(roleNames)
	cacheKey := strings.Join(roleNames, "/")
	var rolePermissionsMap map[string]*model.RolePermissions
	if err := s.rootStore.doStandardReadCache(s.rootStore.rolePermissionsCache, cacheKey, &rolePermissionsMap); err == nil {
		return rolePermissionsMap, nil
	}

	rolePermissionsMap, err := s.RoleStore.ChannelHigherScopedPermissions(roleNames)
	if err != nil {
		return nil, err
	}

	s.rootStore.doStandardAddToCache(s.rootStore.rolePermissionsCache, cacheKey, rolePermissionsMap)
	return rolePermissionsMap, nil
}
