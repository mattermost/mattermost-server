// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dyatlov/go-opengraph/opengraph"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/services/cache"
	"github.com/mattermost/mattermost-server/v5/shared/markdown"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils/imgutils"
)

type linkMetadataCache struct {
	OpenGraph *opengraph.OpenGraph
	PostImage *model.PostImage
	Permalink *model.Permalink
}

const LinkCacheSize = 10000
const LinkCacheDuration = 1 * time.Hour
const MaxMetadataImageSize = MaxOpenGraphResponseSize

var linkCache = cache.NewLRU(cache.LRUOptions{
	Size: LinkCacheSize,
})

func (s *Server) initPostMetadata() {
	// Dump any cached links if the proxy settings have changed so image URLs can be updated
	s.AddConfigListener(func(before, after *model.Config) {
		if (before.ImageProxySettings.Enable != after.ImageProxySettings.Enable) ||
			(before.ImageProxySettings.ImageProxyType != after.ImageProxySettings.ImageProxyType) ||
			(before.ImageProxySettings.RemoteImageProxyURL != after.ImageProxySettings.RemoteImageProxyURL) ||
			(before.ImageProxySettings.RemoteImageProxyOptions != after.ImageProxySettings.RemoteImageProxyOptions) {
			linkCache.Purge()
		}
	})
}

func (a *App) PreparePostListForClient(originalList *model.PostList, askingUserID string) *model.PostList {
	list := &model.PostList{
		Posts:      make(map[string]*model.Post, len(originalList.Posts)),
		Order:      originalList.Order,
		NextPostId: originalList.NextPostId,
		PrevPostId: originalList.PrevPostId,
	}

	for id, originalPost := range originalList.Posts {
		post := a.PreparePostForClient(originalPost, false, false, askingUserID)

		list.Posts[id] = post
	}

	return list
}

// OverrideIconURLIfEmoji changes the post icon override URL prop, if it has an emoji icon,
// so that it points to the URL (relative) of the emoji - static if emoji is default, /api if custom.
func (a *App) OverrideIconURLIfEmoji(post *model.Post) {
	prop, ok := post.GetProps()[model.POST_PROPS_OVERRIDE_ICON_EMOJI]
	if !ok || prop == nil {
		return
	}

	emojiName, ok := prop.(string)
	if !ok {
		return
	}

	if !*a.Config().ServiceSettings.EnablePostIconOverride || emojiName == "" {
		return
	}

	emojiName = strings.ReplaceAll(emojiName, ":", "")

	if emojiUrl, err := a.GetEmojiStaticUrl(emojiName); err == nil {
		post.AddProp(model.POST_PROPS_OVERRIDE_ICON_URL, emojiUrl)
	} else {
		mlog.Warn("Failed to retrieve URL for overridden profile icon (emoji)", mlog.String("emojiName", emojiName), mlog.Err(err))
	}
}

func (a *App) PreparePostForClient(originalPost *model.Post, isNewPost bool, isEditPost bool, askingUserID string) *model.Post {
	post := originalPost.Clone()

	// Proxy image links before constructing metadata so that requests go through the proxy
	post = a.PostWithProxyAddedToImageURLs(post)

	a.OverrideIconURLIfEmoji(post)

	post.Metadata = &model.PostMetadata{}

	if post.DeleteAt > 0 {
		// For deleted posts we don't fill out metadata nor do we return the post content
		post.Message = ""
		return post
	}

	// Emojis and reaction counts
	if emojis, reactions, err := a.getEmojisAndReactionsForPost(post); err != nil {
		mlog.Warn("Failed to get emojis and reactions for a post", mlog.String("post_id", post.Id), mlog.Err(err))
	} else {
		post.Metadata.Emojis = emojis
		post.Metadata.Reactions = reactions
	}

	// Files
	if fileInfos, err := a.getFileMetadataForPost(post, isNewPost || isEditPost); err != nil {
		mlog.Warn("Failed to get files for a post", mlog.String("post_id", post.Id), mlog.Err(err))
	} else {
		post.Metadata.Files = fileInfos
	}

	// Embeds and image dimensions
	firstLink, images := a.getFirstLinkAndImages(post.Message)

	if embed, err := a.getEmbedForPost(post, firstLink, isNewPost, askingUserID); err != nil {
		mlog.Debug("Failed to get embedded content for a post", mlog.String("post_id", post.Id), mlog.Err(err))
	} else if embed == nil {
		post.Metadata.Embeds = []*model.PostEmbed{}
	} else {
		post.Metadata.Embeds = []*model.PostEmbed{embed}
	}

	post.Metadata.Images = a.getImagesForPost(post, images, isNewPost)

	return post
}

func (a *App) getFileMetadataForPost(post *model.Post, fromMaster bool) ([]*model.FileInfo, *model.AppError) {
	if len(post.FileIds) == 0 {
		return nil, nil
	}

	return a.GetFileInfosForPost(post.Id, fromMaster)
}

func (a *App) getEmojisAndReactionsForPost(post *model.Post) ([]*model.Emoji, []*model.Reaction, *model.AppError) {
	var reactions []*model.Reaction
	if post.HasReactions {
		var err *model.AppError
		reactions, err = a.GetReactionsForPost(post.Id)
		if err != nil {
			return nil, nil, err
		}
	}

	emojis, err := a.getCustomEmojisForPost(post, reactions)
	if err != nil {
		return nil, nil, err
	}

	return emojis, reactions, nil
}

func (a *App) getEmbedForPost(post *model.Post, firstLink string, isNewPost bool, askingUserID string) (*model.PostEmbed, error) {
	if _, ok := post.GetProps()["attachments"]; ok {
		return &model.PostEmbed{
			Type: model.POST_EMBED_MESSAGE_ATTACHMENT,
		}, nil
	}

	if firstLink == "" {
		return nil, nil
	}

	// Permalink previews are not toggled via the ServiceSettings.EnableLinkPreviews config setting.
	if !*a.Config().ServiceSettings.EnableLinkPreviews && !looksLikeAPermalink(firstLink, *a.Config().ServiceSettings.SiteURL) {
		return nil, nil
	}

	og, image, permalink, err := a.getLinkMetadata(firstLink, post.CreateAt, isNewPost)
	if err != nil {
		return nil, err
	}

	if !*a.Config().ServiceSettings.EnablePermalinkPreviews {
		permalink = nil
	}

	if og != nil {
		return &model.PostEmbed{
			Type: model.POST_EMBED_OPENGRAPH,
			URL:  firstLink,
			Data: og,
		}, nil
	}

	if image != nil {
		// Note that we're not passing the image info here since it'll be part of the PostMetadata.Images field
		return &model.PostEmbed{
			Type: model.POST_EMBED_IMAGE,
			URL:  firstLink,
		}, nil
	}

	if permalink != nil {
		embed := &model.PostEmbed{Type: model.POST_EMBED_PERMALINK}

		if !model.IsValidId(askingUserID) {
			return nil, fmt.Errorf("cannot embed permalink preview because user id %q is not valid", askingUserID)
		}

		if permalink.PreviewPost != nil {
			referencedChannel, err := a.GetChannel(permalink.PreviewPost.Post.ChannelId)
			if err != nil && err.StatusCode != http.StatusNotFound {
				return nil, err
			}

			// Only embed the referenced post into the Data field if the containing channel wasn't deleted and the requesting user has access to the post.
			if referencedChannel != nil && a.HasPermissionToChannel(askingUserID, referencedChannel.Id, model.PERMISSION_READ_CHANNEL) || (referencedChannel.Type == model.CHANNEL_OPEN && a.HasPermissionToTeam(askingUserID, referencedChannel.TeamId, model.PERMISSION_READ_PUBLIC_CHANNEL)) {
				embed.Data = permalink.PreviewPost
			}
		}

		return embed, nil
	}

	return &model.PostEmbed{
		Type: model.POST_EMBED_LINK,
		URL:  firstLink,
	}, nil
}

func (a *App) getImagesForPost(post *model.Post, imageURLs []string, isNewPost bool) map[string]*model.PostImage {
	images := map[string]*model.PostImage{}

	for _, embed := range post.Metadata.Embeds {
		switch embed.Type {
		case model.POST_EMBED_IMAGE:
			// These dimensions will generally be cached by a previous call to getEmbedForPost
			imageURLs = append(imageURLs, embed.URL)

		case model.POST_EMBED_MESSAGE_ATTACHMENT:
			imageURLs = append(imageURLs, a.getImagesInMessageAttachments(post)...)

		case model.POST_EMBED_OPENGRAPH:
			for _, image := range embed.Data.(*opengraph.OpenGraph).Images {
				var imageURL string
				if image.SecureURL != "" {
					imageURL = image.SecureURL
				} else if image.URL != "" {
					imageURL = image.URL
				}

				if imageURL == "" {
					continue
				}

				imageURLs = append(imageURLs, imageURL)
			}
		}
	}

	// Removing duplicates isn't strictly since images is a map, but it feels safer to do it beforehand
	if len(imageURLs) > 1 {
		imageURLs = model.RemoveDuplicateStrings(imageURLs)
	}

	for _, imageURL := range imageURLs {
		if _, image, _, err := a.getLinkMetadata(imageURL, post.CreateAt, isNewPost); err != nil {
			mlog.Debug("Failed to get dimensions of an image in a post",
				mlog.String("post_id", post.Id), mlog.String("image_url", imageURL), mlog.Err(err))
		} else if image != nil {
			images[imageURL] = image
		}
	}

	return images
}

func getEmojiNamesForString(s string) []string {
	names := model.EMOJI_PATTERN.FindAllString(s, -1)

	for i, name := range names {
		names[i] = strings.Trim(name, ":")
	}

	return names
}

func getEmojiNamesForPost(post *model.Post, reactions []*model.Reaction) []string {
	// Post message
	names := getEmojiNamesForString(post.Message)

	// Reactions
	for _, reaction := range reactions {
		names = append(names, reaction.EmojiName)
	}

	// Post attachments
	for _, attachment := range post.Attachments() {
		if attachment.Text != "" {
			names = append(names, getEmojiNamesForString(attachment.Text)...)
		}

		if attachment.Pretext != "" {
			names = append(names, getEmojiNamesForString(attachment.Pretext)...)
		}

		for _, field := range attachment.Fields {
			if value, ok := field.Value.(string); ok {
				names = append(names, getEmojiNamesForString(value)...)
			}
		}
	}

	// Remove duplicates
	names = model.RemoveDuplicateStrings(names)

	return names
}

func (a *App) getCustomEmojisForPost(post *model.Post, reactions []*model.Reaction) ([]*model.Emoji, *model.AppError) {
	if !*a.Config().ServiceSettings.EnableCustomEmoji {
		// Only custom emoji are returned
		return []*model.Emoji{}, nil
	}

	names := getEmojiNamesForPost(post, reactions)

	if len(names) == 0 {
		return []*model.Emoji{}, nil
	}

	return a.GetMultipleEmojiByName(names)
}

func (a *App) isLinkAllowedForPreview(link string) bool {
	domains := a.normalizeDomains(*a.Config().ServiceSettings.RestrictLinkPreviews)
	for _, d := range domains {
		if strings.Contains(link, d) {
			return false
		}
	}

	return true
}

// Given a string, returns the first autolinked URL in the string as well as an array of all Markdown
// images of the form ![alt text](image url). Note that this does not return Markdown links of the
// form [text](url).
func (a *App) getFirstLinkAndImages(str string) (string, []string) {
	firstLink := ""
	images := []string{}

	markdown.Inspect(str, func(blockOrInline interface{}) bool {
		switch v := blockOrInline.(type) {
		case *markdown.Autolink:
			if link := v.Destination(); firstLink == "" && a.isLinkAllowedForPreview(link) {
				firstLink = link
			}
		case *markdown.InlineImage:
			if link := v.Destination(); a.isLinkAllowedForPreview(link) {
				images = append(images, link)
			}
		case *markdown.ReferenceImage:
			if link := v.ReferenceDefinition.Destination(); a.isLinkAllowedForPreview(link) {
				images = append(images, link)
			}
		}

		return true
	})

	return firstLink, images
}

func (a *App) getImagesInMessageAttachments(post *model.Post) []string {
	var images []string

	for _, attachment := range post.Attachments() {
		_, imagesInText := a.getFirstLinkAndImages(attachment.Text)
		images = append(images, imagesInText...)

		_, imagesInPretext := a.getFirstLinkAndImages(attachment.Pretext)
		images = append(images, imagesInPretext...)

		for _, field := range attachment.Fields {
			if value, ok := field.Value.(string); ok {
				_, imagesInFieldValue := a.getFirstLinkAndImages(value)
				images = append(images, imagesInFieldValue...)
			}
		}

		if attachment.AuthorIcon != "" {
			images = append(images, attachment.AuthorIcon)
		}

		if attachment.ImageURL != "" {
			images = append(images, attachment.ImageURL)
		}

		if attachment.ThumbURL != "" {
			images = append(images, attachment.ThumbURL)
		}

		if attachment.FooterIcon != "" {
			images = append(images, attachment.FooterIcon)
		}
	}

	return images
}

func looksLikeAPermalink(url, siteURL string) bool {
	expression := fmt.Sprintf(`^(%s).*(/pl/)[a-z0-9]{26}$`, siteURL)
	matched, err := regexp.MatchString(expression, strings.TrimSpace(url))
	if err != nil {
		mlog.Warn("error matching regex", mlog.Err(err))
	}
	return matched
}

func postIDFromPermalink(permalinkURL string) string {
	return permalinkURL[len(permalinkURL)-26:]
}

func (a *App) getLinkMetadata(requestURL string, timestamp int64, isNewPost bool) (*opengraph.OpenGraph, *model.PostImage, *model.Permalink, error) {
	requestURL = resolveMetadataURL(requestURL, a.GetSiteURL())

	timestamp = model.FloorToNearestHour(timestamp)

	// Check cache
	og, image, permalink, ok := getLinkMetadataFromCache(requestURL, timestamp)
	if !*a.Config().ServiceSettings.EnablePermalinkPreviews {
		permalink = nil
	}

	if ok {
		return og, image, permalink, nil
	}

	// Check the database if this isn't a new post. If it is a new post and the data is cached, it should be in memory.
	if !isNewPost {
		og, image, permalink, ok = a.getLinkMetadataFromDatabase(requestURL, timestamp)

		if !*a.Config().ServiceSettings.EnablePermalinkPreviews {
			permalink = nil
		}

		if permalink != nil && permalink.PreviewPost == nil {
			postID := postIDFromPermalink(requestURL)

			referencedPost, appErr := a.GetSinglePost(postID)
			// Ignore 'not found' errors; post could have been deleted via retention policy so we don't want to permanently log a warning.
			//
			// TODO: Look into saving a value in the LinkMetadat.Data field to prevent perpetually re-querying for the deleted post.
			if appErr != nil && appErr.StatusCode != http.StatusNotFound {
				return nil, nil, nil, appErr
			}

			var referencedChannel *model.Channel
			if referencedPost != nil {
				referencedChannel, appErr = a.GetChannel(referencedPost.ChannelId)
				if appErr != nil {
					return nil, nil, nil, appErr
				}
			}

			var referencedTeam *model.Team
			if referencedChannel != nil {
				referencedTeam, appErr = a.GetTeam(referencedChannel.TeamId)
				if appErr != nil {
					return nil, nil, nil, appErr
				}
			}

			permalink.PreviewPost = model.PreviewPostFromPost(referencedPost, referencedTeam, referencedChannel)
		}

		if ok {
			cacheLinkMetadata(requestURL, timestamp, og, image, permalink)

			return og, image, permalink, nil
		}
	}

	var err error

	if looksLikeAPermalink(requestURL, a.GetSiteURL()) && *a.Config().ServiceSettings.EnablePermalinkPreviews {
		referencedPostID := postIDFromPermalink(requestURL)

		referencedPost, appErr := a.GetSinglePost(referencedPostID)
		// Ignore 'not found' errors; post could have been deleted via retention policy so we don't want to permanently log a warning.
		//
		// TODO: Look into saving a value in the LinkMetadat.Data field to prevent perpetually re-querying for the deleted post.
		if appErr != nil && appErr.StatusCode != http.StatusNotFound {
			return nil, nil, nil, appErr
		}

		referencedChannel, appErr := a.GetChannel(referencedPost.ChannelId)
		if appErr != nil {
			return nil, nil, nil, appErr
		}

		referencedTeam, appErr := a.GetTeam(referencedChannel.TeamId)
		if appErr != nil {
			return nil, nil, nil, appErr
		}

		permalink = &model.Permalink{PreviewPost: model.PreviewPostFromPost(referencedPost, referencedTeam, referencedChannel)}
	} else {

		var request *http.Request
		// Make request for a web page or an image
		request, err = http.NewRequest("GET", requestURL, nil)
		if err != nil {
			return nil, nil, nil, err
		}

		var body io.ReadCloser
		var contentType string

		if (request.URL.Scheme+"://"+request.URL.Host) == a.GetSiteURL() && request.URL.Path == "/api/v4/image" {
			// /api/v4/image requires authentication, so bypass the API by hitting the proxy directly
			body, contentType, err = a.ImageProxy().GetImageDirect(a.ImageProxy().GetUnproxiedImageURL(request.URL.String()))
		} else {
			request.Header.Add("Accept", "image/*")
			request.Header.Add("Accept", "text/html;q=0.8")

			client := a.HTTPService().MakeClient(false)
			client.Timeout = time.Duration(*a.Config().ExperimentalSettings.LinkMetadataTimeoutMilliseconds) * time.Millisecond

			var res *http.Response
			res, err = client.Do(request)

			if res != nil {
				body = res.Body
				contentType = res.Header.Get("Content-Type")
			}
		}

		if body != nil {
			defer func() {
				io.Copy(ioutil.Discard, body)
				body.Close()
			}()
		}

		if err == nil {
			// Parse the data
			og, image, err = a.parseLinkMetadata(requestURL, body, contentType)
		}
		og = model.TruncateOpenGraph(og) // remove unwanted length of texts
	}

	// Write back to cache and database, even if there was an error and the results are nil
	cacheLinkMetadata(requestURL, timestamp, og, image, permalink)

	a.saveLinkMetadataToDatabase(requestURL, timestamp, og, image, permalink)

	return og, image, permalink, err
}

// resolveMetadataURL resolves a given URL relative to the server's site URL.
func resolveMetadataURL(requestURL string, siteURL string) string {
	base, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}

	resolved, err := base.Parse(requestURL)
	if err != nil {
		return ""
	}

	return resolved.String()
}

func getLinkMetadataFromCache(requestURL string, timestamp int64) (*opengraph.OpenGraph, *model.PostImage, *model.Permalink, bool) {
	var cached linkMetadataCache
	err := linkCache.Get(strconv.FormatInt(model.GenerateLinkMetadataHash(requestURL, timestamp), 16), &cached)
	if err != nil {
		return nil, nil, nil, false
	}

	return cached.OpenGraph, cached.PostImage, cached.Permalink, true
}

func (a *App) getLinkMetadataFromDatabase(requestURL string, timestamp int64) (*opengraph.OpenGraph, *model.PostImage, *model.Permalink, bool) {
	linkMetadata, err := a.Srv().Store.LinkMetadata().Get(requestURL, timestamp)
	if err != nil {
		return nil, nil, nil, false
	}

	data := linkMetadata.Data

	if linkMetadata.Type == model.LINK_METADATA_TYPE_PERMALINK && *a.Config().ServiceSettings.EnablePermalinkPreviews {
		return nil, nil, &model.Permalink{}, true
	}

	switch v := data.(type) {
	case *opengraph.OpenGraph:
		return v, nil, nil, true
	case *model.PostImage:
		return nil, v, nil, true
	default:
		return nil, nil, nil, true
	}
}

func (a *App) saveLinkMetadataToDatabase(requestURL string, timestamp int64, og *opengraph.OpenGraph, image *model.PostImage, permalink *model.Permalink) {
	metadata := &model.LinkMetadata{
		URL:       requestURL,
		Timestamp: timestamp,
	}

	if og != nil {
		metadata.Type = model.LINK_METADATA_TYPE_OPENGRAPH
		metadata.Data = og
	} else if image != nil {
		metadata.Type = model.LINK_METADATA_TYPE_IMAGE
		metadata.Data = image
	} else if permalink != nil {
		metadata.Type = model.LINK_METADATA_TYPE_PERMALINK
		metadata.Data = nil // Permalink's don't persist data; the referenced post retrieved from the data layer and cached to the LinkMetadata cache.
	} else {
		metadata.Type = model.LINK_METADATA_TYPE_NONE
	}

	_, err := a.Srv().Store.LinkMetadata().Save(metadata)
	if err != nil {
		mlog.Warn("Failed to write link metadata", mlog.String("request_url", requestURL), mlog.Err(err))
	}
}

func cacheLinkMetadata(requestURL string, timestamp int64, og *opengraph.OpenGraph, image *model.PostImage, permalink *model.Permalink) {
	metadata := linkMetadataCache{
		OpenGraph: og,
		PostImage: image,
		Permalink: permalink,
	}

	linkCache.SetWithExpiry(strconv.FormatInt(model.GenerateLinkMetadataHash(requestURL, timestamp), 16), metadata, LinkCacheDuration)
}

func (a *App) parseLinkMetadata(requestURL string, body io.Reader, contentType string) (*opengraph.OpenGraph, *model.PostImage, error) {
	if contentType == "image/svg+xml" {
		image := &model.PostImage{
			Format: "svg",
		}

		return nil, image, nil
	} else if strings.HasPrefix(contentType, "image") {
		image, err := parseImages(io.LimitReader(body, MaxMetadataImageSize))
		return nil, image, err
	} else if strings.HasPrefix(contentType, "text/html") {
		og := a.parseOpenGraphMetadata(requestURL, body, contentType)

		// The OpenGraph library and Go HTML library don't error for malformed input, so check that at least
		// one of these required fields exists before returning the OpenGraph data
		if og.Title != "" || og.Type != "" || og.URL != "" {
			return og, nil, nil
		}
		return nil, nil, nil
	} else {
		// Not an image or web page with OpenGraph information
		return nil, nil, nil
	}
}

func parseImages(body io.Reader) (*model.PostImage, error) {
	// Store any data that is read for the config for any further processing
	buf := &bytes.Buffer{}
	t := io.TeeReader(body, buf)

	// Read the image config to get the format and dimensions
	config, format, err := image.DecodeConfig(t)
	if err != nil {
		return nil, err
	}

	image := &model.PostImage{
		Width:  config.Width,
		Height: config.Height,
		Format: format,
	}

	if format == "gif" {
		// Decoding the config may have read some of the image data, so re-read the data that has already been read first
		frameCount, err := imgutils.CountFrames(io.MultiReader(buf, body))
		if err != nil {
			return nil, err
		}

		image.FrameCount = frameCount
	}

	// Make image information nil when the format is tiff
	if format == "tiff" {
		image = nil
	}

	return image, nil
}
