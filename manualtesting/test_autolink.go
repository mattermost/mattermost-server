// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package manualtesting

import (
	"net/http"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const linkPostText = `
Some Links:
https://spinpunch.atlassian.net/issues/?filter=10101&jql=resolution%20in%20(Fixed%2C%20%22Won't%20Fix%22%2C%20Duplicate%2C%20%22Cannot%20Reproduce%22)%20AND%20Resolution%20%3D%20Fixed%20AND%20updated%20%3E%3D%20-7d%20ORDER%20BY%20updatedDate%20DESC

https://www.google.com.pk/url?sa=t&rct=j&q=&esrc=s&source=web&cd=2&cad=rja&uact=8&ved=0CCUQFjAB&url=https%3A%2F%2Fdevelopers.google.com%2Fmaps%2Fdocumentation%2Fios%2Furlscheme&ei=HBFbVdSBN-WcygOG4oHIBw&usg=AFQjCNGI0Jg92Y7qNmyIpQyvYPut7vx5-Q&bvm=bv.93564037,d.bGg

http://www.google.com.pk/url?sa=t&rct=j&q=&esrc=s&source=web&cd=4&cad=rja&uact=8&ved=0CC8QFjAD&url=http%3A%2F%2Fwww.quora.com%2FHow-long-will-a-Google-shortened-URL-be-available&ei=XRBbVbPLGYKcsAGqiIDQAw&usg=AFQjCNHY0Xi-GG4hgbrPUY_8Kg-55_-DNQ&bvm=bv.93564037,d.bGg

https://medium.com/@slackhq/11-useful-tips-for-getting-the-most-of-slack-5dfb3d1af77
`

func testAutoLink(env TestEnvironment) *model.AppError {
	mlog.Info("Manual Auto Link Test")
	channelID, err := getChannelID(env.Context.App, model.DefaultChannelName, env.CreatedTeamID, env.CreatedUserID)
	if !err {
		return model.NewAppError("/manualtest", "manaultesting.test_autolink.unable.app_error", nil, "", http.StatusInternalServerError)
	}

	post := &model.Post{
		ChannelID: channelID,
		Message:   linkPostText}
	_, resp := env.Client.CreatePost(post)
	return resp.Error
}
