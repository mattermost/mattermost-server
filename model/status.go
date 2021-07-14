// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"io"
)

const (
	STATUS_OUT_OF_OFFICE   = "ooo"
	STATUS_OFFLINE         = "offline"
	STATUS_AWAY            = "away"
	STATUS_DND             = "dnd"
	STATUS_ONLINE          = "online"
	STATUS_CACHE_SIZE      = SESSION_CACHE_SIZE
	STATUS_CHANNEL_TIMEOUT = 20000  // 20 seconds
	STATUS_MIN_UPDATE_TIME = 120000 // 2 minutes
)

type Status struct {
	UserId         string `json:"user_id"`
	Status         string `json:"status"`
	Manual         bool   `json:"manual"`
	LastActivityAt int64  `json:"last_activity_at"`
	ActiveChannel  string `json:"active_channel,omitempty" db:"-"`
	DNDEndTime     int64  `json:"dnd_end_time"`
	PrevStatus     string `json:"-"`
	MondayStart    string `json:"monday_start"`
	MondayEnd      string `json:"monday_end"`
	TuesdayStart   string `json:"tuesday_start"`
	TuesdayEnd     string `json:"tuesday_end"`
	WednesdayStart string `json:"wednesday_start"`
	WednesdayEnd   string `json:"wednesday_end"`
	ThursdayStart  string `json:"thursday_start"`
	ThursdayEnd    string `json:"thursday_end"`
	FridayStart    string `json:"friday_start"`
	FridayEnd      string `json:"friday_end"`
	SaturdayStart  string `json:"saturday_start"`
	SaturdayEnd    string `json:"saturday_end"`
	SundayStart    string `json:"sunday_start"`
	SundayEnd      string `json:"sunday_end"`
	Mode           int64  `json:"mode"`
	CurrentTime    string `json:"current_time"`
	DayOfTheWeek   string `json:"day_of_the_week"`
}

func (o *Status) ToJson() string {
	oCopy := *o
	oCopy.ActiveChannel = ""
	b, _ := json.Marshal(oCopy)
	return string(b)
}

func (o *Status) ToClusterJson() string {
	oCopy := *o
	b, _ := json.Marshal(oCopy)
	return string(b)
}

func StatusFromJson(data io.Reader) *Status {
	var o *Status
	json.NewDecoder(data).Decode(&o)
	return o
}

func StatusListToJson(u []*Status) string {
	uCopy := make([]Status, len(u))
	for i, s := range u {
		sCopy := *s
		sCopy.ActiveChannel = ""
		uCopy[i] = sCopy
	}

	b, _ := json.Marshal(uCopy)
	return string(b)
}

func StatusListFromJson(data io.Reader) []*Status {
	var statuses []*Status
	json.NewDecoder(data).Decode(&statuses)
	return statuses
}

func StatusMapToInterfaceMap(statusMap map[string]*Status) map[string]interface{} {
	interfaceMap := map[string]interface{}{}
	for _, s := range statusMap {
		// Omitted statues mean offline
		if s.Status != STATUS_OFFLINE {
			interfaceMap[s.UserId] = s.Status
		}
	}
	return interfaceMap
}
