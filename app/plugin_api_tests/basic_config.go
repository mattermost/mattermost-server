// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin_api_tests

import "reflect"

type BasicConfig struct {
	BasicChannelID       string
	BasicChannelName     string
	BasicPostID          string
	BasicPostMessage     string
	BasicTeamDisplayName string
	BasicTeamID          string
	BasicTeamName        string
	BasicUser2Email      string
	BasicUser2ID         string
	BasicUserEmail       string
	BasicUserID          string
}

func IsEmpty(object interface{}) bool {

	// get nil case out of the way
	if object == nil {
		return true
	}

	objValue := reflect.ValueOf(object)

	switch objValue.Kind() {
	// collection types are empty when they have no element
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return objValue.Len() == 0
	// pointers are empty if nil or if the value they point to is empty
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}
		deref := objValue.Elem().Interface()
		return IsEmpty(deref)
	// for all other types, compare against the zero value
	default:
		zero := reflect.Zero(objValue.Type())
		return reflect.DeepEqual(object, zero.Interface())
	}
}
