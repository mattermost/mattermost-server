// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import "encoding/json"

type Permalink struct {
	PreviewPost *PreviewPost `json:"preview_post"`
}

type PreviewPost struct {
	PostID             string `json:"post_id"`
	Post               *Post  `json:"post"`
	TeamName           string `json:"team_name"`
	ChannelDisplayName string `json:"channel_display_name"`
}

func PreviewPostFromPost(post *Post, team *Team, channel *Channel) *PreviewPost {
	if post == nil {
		return nil
	}
	return &PreviewPost{
		PostID:             post.Id,
		Post:               post,
		TeamName:           team.Name,
		ChannelDisplayName: channel.DisplayName,
	}
}

func (o *Permalink) ToJson() string {
	b, _ := json.Marshal(o)
	return string(b)
}
