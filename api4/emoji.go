// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/web"
)

const (
	EmojiMaxAutocompleteItems = 100
)

func (api *API) InitEmoji() {
	api.BaseRoutes.Emojis.Handle("", api.APISessionRequired(createEmoji)).Methods("POST")
	api.BaseRoutes.Emojis.Handle("", api.APISessionRequired(getEmojiList)).Methods("GET")
	api.BaseRoutes.Emojis.Handle("/search", api.APISessionRequired(searchEmojis)).Methods("POST")
	api.BaseRoutes.Emojis.Handle("/autocomplete", api.APISessionRequired(autocompleteEmojis)).Methods("GET")
	api.BaseRoutes.Emoji.Handle("", api.APISessionRequired(deleteEmoji)).Methods("DELETE")
	api.BaseRoutes.Emoji.Handle("", api.APISessionRequired(getEmoji)).Methods("GET")
	api.BaseRoutes.EmojiByName.Handle("", api.APISessionRequired(getEmojiByName)).Methods("GET")
	api.BaseRoutes.Emoji.Handle("/image", api.APISessionRequiredTrustRequester(getEmojiImage)).Methods("GET")
}

func createEmoji(c *Context, w http.ResponseWriter, r *http.Request) {
	defer io.Copy(ioutil.Discard, r.Body)

	if !*c.App.Config().ServiceSettings.EnableCustomEmoji {
		c.Err = model.NewAppError("createEmoji", "api.emoji.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if r.ContentLength > app.MaxEmojiFileSize {
		c.Err = model.NewAppError("createEmoji", "api.emoji.create.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(app.MaxEmojiFileSize); err != nil {
		c.Err = model.NewAppError("createEmoji", "api.emoji.create.parse.app_error", nil, err.Error(), http.StatusBadRequest)
		return
	}

	auditRec := c.MakeAuditRecord("createEmoji", audit.Fail)
	defer c.LogAuditRec(auditRec)

	// Allow any user with CREATE_EMOJIS permission at Team level to create emojis at system level
	memberships, err := c.App.GetTeamMembersForUser(c.AppContext.Session().UserID)

	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionCreateEmojis) {
		hasPermission := false
		for _, membership := range memberships {
			if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), membership.TeamID, model.PermissionCreateEmojis) {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			c.SetPermissionError(model.PermissionCreateEmojis)
			return
		}
	}

	m := r.MultipartForm
	props := m.Value

	if len(props["emoji"]) == 0 {
		c.SetInvalidParam("emoji")
		return
	}

	emoji := model.EmojiFromJSON(strings.NewReader(props["emoji"][0]))
	if emoji == nil {
		c.SetInvalidParam("emoji")
		return
	}

	auditRec.AddMeta("emoji", emoji)

	newEmoji, err := c.App.CreateEmoji(c.AppContext.Session().UserID, emoji, m)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	w.Write([]byte(newEmoji.ToJSON()))
}

func getEmojiList(c *Context, w http.ResponseWriter, r *http.Request) {
	if !*c.App.Config().ServiceSettings.EnableCustomEmoji {
		c.Err = model.NewAppError("getEmoji", "api.emoji.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	sort := r.URL.Query().Get("sort")
	if sort != "" && sort != model.EmojiSortByName {
		c.SetInvalidURLParam("sort")
		return
	}

	listEmoji, err := c.App.GetEmojiList(c.Params.Page, c.Params.PerPage, sort)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.EmojiListToJSON(listEmoji)))
}

func deleteEmoji(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireEmojiID()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("deleteEmoji", audit.Fail)
	defer c.LogAuditRec(auditRec)

	emoji, err := c.App.GetEmoji(c.Params.EmojiID)
	if err != nil {
		auditRec.AddMeta("emoji_id", c.Params.EmojiID)
		c.Err = err
		return
	}
	auditRec.AddMeta("emoji", emoji)

	// Allow any user with DELETE_EMOJIS permission at Team level to delete emojis at system level
	memberships, err := c.App.GetTeamMembersForUser(c.AppContext.Session().UserID)

	if err != nil {
		c.Err = err
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionDeleteEmojis) {
		hasPermission := false
		for _, membership := range memberships {
			if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), membership.TeamID, model.PermissionDeleteEmojis) {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			c.SetPermissionError(model.PermissionDeleteEmojis)
			return
		}
	}

	if c.AppContext.Session().UserID != emoji.CreatorID {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionDeleteOthersEmojis) {
			hasPermission := false
			for _, membership := range memberships {
				if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), membership.TeamID, model.PermissionDeleteOthersEmojis) {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				c.SetPermissionError(model.PermissionDeleteOthersEmojis)
				return
			}
		}
	}

	err = c.App.DeleteEmoji(emoji)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()

	ReturnStatusOK(w)
}

func getEmoji(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireEmojiID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().ServiceSettings.EnableCustomEmoji {
		c.Err = model.NewAppError("getEmoji", "api.emoji.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	emoji, err := c.App.GetEmoji(c.Params.EmojiID)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(emoji.ToJSON()))
}

func getEmojiByName(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireEmojiName()
	if c.Err != nil {
		return
	}

	emoji, err := c.App.GetEmojiByName(c.Params.EmojiName)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(emoji.ToJSON()))
}

func getEmojiImage(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireEmojiID()
	if c.Err != nil {
		return
	}

	if !*c.App.Config().ServiceSettings.EnableCustomEmoji {
		c.Err = model.NewAppError("getEmojiImage", "api.emoji.disabled.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	image, imageType, err := c.App.GetEmojiImage(c.Params.EmojiID)
	if err != nil {
		c.Err = err
		return
	}

	w.Header().Set("Content-Type", "image/"+imageType)
	w.Header().Set("Cache-Control", "max-age=2592000, private")
	w.Write(image)
}

func searchEmojis(c *Context, w http.ResponseWriter, r *http.Request) {
	emojiSearch := model.EmojiSearchFromJSON(r.Body)
	if emojiSearch == nil {
		c.SetInvalidParam("term")
		return
	}

	if emojiSearch.Term == "" {
		c.SetInvalidParam("term")
		return
	}

	emojis, err := c.App.SearchEmoji(emojiSearch.Term, emojiSearch.PrefixOnly, web.PerPageMaximum)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.EmojiListToJSON(emojis)))
}

func autocompleteEmojis(c *Context, w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	if name == "" {
		c.SetInvalidURLParam("name")
		return
	}

	emojis, err := c.App.SearchEmoji(name, true, EmojiMaxAutocompleteItems)
	if err != nil {
		c.Err = err
		return
	}

	w.Write([]byte(model.EmojiListToJSON(emojis)))
}
