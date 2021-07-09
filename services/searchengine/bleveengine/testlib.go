// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bleveengine

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v5/model"
)

func createPost(userID string, channelID string) *model.Post {
	post := &model.Post{
		Message:       model.NewRandomString(15),
		ChannelID:     channelID,
		PendingPostID: model.NewID() + ":" + fmt.Sprint(model.GetMillis()),
		UserID:        userID,
		CreateAt:      1000000,
	}
	post.PreSave()

	return post
}
