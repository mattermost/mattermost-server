// Copyright (c) 2015 Spinpunch, Inc. All Rights Reserved.
// See License.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

const (
	PREFERENCE_CATEGORY_DIRECT_CHANNELS = "direct_channels"
	PREFERENCE_NAME_SHOWHIDE            = "show_hide"
)

type Preference struct {
	UserId   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	AltId    string `json:"alt_id"`
	Value    string `json:"value"`
}

func (o *Preference) ToJson() string {
	b, err := json.Marshal(o)
	if err != nil {
		return ""
	} else {
		return string(b)
	}
}

func PreferenceFromJson(data io.Reader) *Preference {
	decoder := json.NewDecoder(data)
	var o Preference
	err := decoder.Decode(&o)
	if err == nil {
		return &o
	} else {
		return nil
	}
}

func (o *Preference) IsValid() *AppError {
	if len(o.UserId) != 26 {
		return NewAppError("Preference.IsValid", "Invalid user id", "user_id="+o.UserId)
	}

	if len(o.Category) == 0 || len(o.Category) > 32 || !IsPreferenceCategoryValid(o.Category) {
		return NewAppError("Preference.IsValid", "Invalid category", "category="+o.Category)
	}

	if len(o.Name) == 0 || len(o.Name) > 32 || !IsPreferenceNameValid(o.Name) {
		return NewAppError("Preference.IsValid", "Invalid name", "name="+o.Name)
	}

	if o.AltId != "" && len(o.AltId) != 26 {
		return NewAppError("Preference.IsValid", "Invalid alternate id", "altId="+o.AltId)
	}

	if len(o.Value) > 128 {
		return NewAppError("Preference.IsValid", "Value is too long", "value="+o.Value)
	}

	return nil
}

func IsPreferenceCategoryValid(category string) bool {
	return category == PREFERENCE_CATEGORY_DIRECT_CHANNELS
}

func IsPreferenceNameValid(name string) bool {
	return name == PREFERENCE_NAME_SHOWHIDE
}
