// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package bleveengine

import (
	"net/http"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search/query"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const DeletePostsBatchSize = 500
const DeleteFilesBatchSize = 500

func (b *BleveEngine) IndexPost(post *model.Post, teamID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	blvPost := BLVPostFromPost(post, teamID)
	if err := b.PostIndex.Index(blvPost.ID, blvPost); err != nil {
		return model.NewAppError("Bleveengine.IndexPost", "bleveengine.index_post.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) SearchPosts(channels *model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, model.PostSearchMatches, *model.AppError) {
	channelQueries := []query.Query{}
	for _, channel := range *channels {
		channelIDQ := bleve.NewTermQuery(channel.ID)
		channelIDQ.SetField("ChannelId")
		channelQueries = append(channelQueries, channelIDQ)
	}
	channelDisjunctionQ := bleve.NewDisjunctionQuery(channelQueries...)

	var termQueries []query.Query
	var notTermQueries []query.Query
	var filters []query.Query
	var notFilters []query.Query

	typeQ := bleve.NewTermQuery("")
	typeQ.SetField("Type")
	filters = append(filters, typeQ)

	for i, params := range searchParams {
		var termOperator query.MatchQueryOperator = query.MatchQueryOperatorAnd
		if searchParams[0].OrTerms {
			termOperator = query.MatchQueryOperatorOr
		}

		// Date, channels and FromUsers filters come in all
		// searchParams iteration, and as they are global to the
		// query, we only need to process them once
		if i == 0 {
			if len(params.InChannels) > 0 {
				inChannels := []query.Query{}
				for _, channelID := range params.InChannels {
					channelQ := bleve.NewTermQuery(channelID)
					channelQ.SetField("ChannelId")
					inChannels = append(inChannels, channelQ)
				}
				filters = append(filters, bleve.NewDisjunctionQuery(inChannels...))
			}

			if len(params.ExcludedChannels) > 0 {
				excludedChannels := []query.Query{}
				for _, channelID := range params.ExcludedChannels {
					channelQ := bleve.NewTermQuery(channelID)
					channelQ.SetField("ChannelId")
					excludedChannels = append(excludedChannels, channelQ)
				}
				notFilters = append(notFilters, bleve.NewDisjunctionQuery(excludedChannels...))
			}

			if len(params.FromUsers) > 0 {
				fromUsers := []query.Query{}
				for _, userID := range params.FromUsers {
					userQ := bleve.NewTermQuery(userID)
					userQ.SetField("UserId")
					fromUsers = append(fromUsers, userQ)
				}
				filters = append(filters, bleve.NewDisjunctionQuery(fromUsers...))
			}

			if len(params.ExcludedUsers) > 0 {
				excludedUsers := []query.Query{}
				for _, userID := range params.ExcludedUsers {
					userQ := bleve.NewTermQuery(userID)
					userQ.SetField("UserId")
					excludedUsers = append(excludedUsers, userQ)
				}
				notFilters = append(notFilters, bleve.NewDisjunctionQuery(excludedUsers...))
			}

			if params.OnDate != "" {
				before, after := params.GetOnDateMillis()
				beforeFloat64 := float64(before)
				afterFloat64 := float64(after)
				onDateQ := bleve.NewNumericRangeQuery(&beforeFloat64, &afterFloat64)
				onDateQ.SetField("CreateAt")
				filters = append(filters, onDateQ)
			} else {
				if params.AfterDate != "" || params.BeforeDate != "" {
					var min, max *float64
					if params.AfterDate != "" {
						minf := float64(params.GetAfterDateMillis())
						min = &minf
					}

					if params.BeforeDate != "" {
						maxf := float64(params.GetBeforeDateMillis())
						max = &maxf
					}

					dateQ := bleve.NewNumericRangeQuery(min, max)
					dateQ.SetField("CreateAt")
					filters = append(filters, dateQ)
				}

				if params.ExcludedAfterDate != "" {
					minf := float64(params.GetExcludedAfterDateMillis())
					dateQ := bleve.NewNumericRangeQuery(&minf, nil)
					dateQ.SetField("CreateAt")
					notFilters = append(notFilters, dateQ)
				}

				if params.ExcludedBeforeDate != "" {
					maxf := float64(params.GetExcludedBeforeDateMillis())
					dateQ := bleve.NewNumericRangeQuery(nil, &maxf)
					dateQ.SetField("CreateAt")
					notFilters = append(notFilters, dateQ)
				}

				if params.ExcludedDate != "" {
					before, after := params.GetExcludedDateMillis()
					beforef := float64(before)
					afterf := float64(after)
					onDateQ := bleve.NewNumericRangeQuery(&beforef, &afterf)
					onDateQ.SetField("CreateAt")
					notFilters = append(notFilters, onDateQ)
				}
			}
		}

		if params.IsHashtag {
			if params.Terms != "" {
				hashtagQ := bleve.NewMatchQuery(params.Terms)
				hashtagQ.SetField("Hashtags")
				hashtagQ.SetOperator(termOperator)
				termQueries = append(termQueries, hashtagQ)
			} else if params.ExcludedTerms != "" {
				hashtagQ := bleve.NewMatchQuery(params.ExcludedTerms)
				hashtagQ.SetField("Hashtags")
				hashtagQ.SetOperator(termOperator)
				notTermQueries = append(notTermQueries, hashtagQ)
			}
		} else {
			if params.Terms != "" {
				terms := []string{}
				for _, term := range strings.Split(params.Terms, " ") {
					if strings.HasSuffix(term, "*") {
						messageQ := bleve.NewWildcardQuery(term)
						messageQ.SetField("Message")
						termQueries = append(termQueries, messageQ)
					} else {
						terms = append(terms, term)
					}
				}

				if len(terms) > 0 {
					messageQ := bleve.NewMatchQuery(strings.Join(terms, " "))
					messageQ.SetField("Message")
					messageQ.SetOperator(termOperator)
					termQueries = append(termQueries, messageQ)
				}
			}

			if params.ExcludedTerms != "" {
				messageQ := bleve.NewMatchQuery(params.ExcludedTerms)
				messageQ.SetField("Message")
				messageQ.SetOperator(termOperator)
				notTermQueries = append(notTermQueries, messageQ)
			}
		}
	}

	allTermsQ := bleve.NewBooleanQuery()
	allTermsQ.AddMustNot(notTermQueries...)
	if searchParams[0].OrTerms {
		allTermsQ.AddShould(termQueries...)
	} else {
		allTermsQ.AddMust(termQueries...)
	}

	query := bleve.NewBooleanQuery()
	query.AddMust(channelDisjunctionQ)

	if len(termQueries) > 0 || len(notTermQueries) > 0 {
		query.AddMust(allTermsQ)
	}

	if len(filters) > 0 {
		query.AddMust(bleve.NewConjunctionQuery(filters...))
	}
	if len(notFilters) > 0 {
		query.AddMustNot(notFilters...)
	}

	search := bleve.NewSearchRequestOptions(query, perPage, page*perPage, false)
	search.SortBy([]string{"-CreateAt"})
	results, err := b.PostIndex.Search(search)
	if err != nil {
		return nil, nil, model.NewAppError("Bleveengine.SearchPosts", "bleveengine.search_posts.error", nil, err.Error(), http.StatusInternalServerError)
	}

	postIDs := []string{}
	matches := model.PostSearchMatches{}

	for _, r := range results.Hits {
		postIDs = append(postIDs, r.ID)
	}

	return postIDs, matches, nil
}

func (b *BleveEngine) deletePosts(searchRequest *bleve.SearchRequest, batchSize int) (int64, error) {
	resultsCount := int64(0)

	for {
		// As we are deleting the posts after fetching them, we need to keep
		// From fixed always to 0
		searchRequest.From = 0
		searchRequest.Size = batchSize
		results, err := b.PostIndex.Search(searchRequest)
		if err != nil {
			return -1, err
		}
		batch := b.PostIndex.NewBatch()
		for _, post := range results.Hits {
			batch.Delete(post.ID)
		}
		if err := b.PostIndex.Batch(batch); err != nil {
			return -1, err
		}
		resultsCount += int64(results.Hits.Len())
		if results.Hits.Len() < batchSize {
			break
		}
	}

	return resultsCount, nil
}

func (b *BleveEngine) DeleteChannelPosts(channelID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	query := bleve.NewTermQuery(channelID)
	query.SetField("ChannelId")
	search := bleve.NewSearchRequest(query)
	deleted, err := b.deletePosts(search, DeletePostsBatchSize)
	if err != nil {
		return model.NewAppError("Bleveengine.DeleteChannelPosts",
			"bleveengine.delete_channel_posts.error", nil,
			err.Error(), http.StatusInternalServerError)
	}

	mlog.Info("Posts for channel deleted", mlog.String("channel_id", channelID), mlog.Int64("deleted", deleted))

	return nil
}

func (b *BleveEngine) DeleteUserPosts(userID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	query := bleve.NewTermQuery(userID)
	query.SetField("UserId")
	search := bleve.NewSearchRequest(query)
	deleted, err := b.deletePosts(search, DeletePostsBatchSize)
	if err != nil {
		return model.NewAppError("Bleveengine.DeleteUserPosts",
			"bleveengine.delete_user_posts.error", nil,
			err.Error(), http.StatusInternalServerError)
	}

	mlog.Info("Posts for user deleted", mlog.String("user_id", userID), mlog.Int64("deleted", deleted))

	return nil
}

func (b *BleveEngine) DeletePost(post *model.Post) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	if err := b.PostIndex.Delete(post.ID); err != nil {
		return model.NewAppError("Bleveengine.DeletePost", "bleveengine.delete_post.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) IndexChannel(channel *model.Channel) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	blvChannel := BLVChannelFromChannel(channel)
	if err := b.ChannelIndex.Index(blvChannel.ID, blvChannel); err != nil {
		return model.NewAppError("Bleveengine.IndexChannel", "bleveengine.index_channel.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) SearchChannels(teamID, term string) ([]string, *model.AppError) {
	teamIDQ := bleve.NewTermQuery(teamID)
	teamIDQ.SetField("TeamId")
	queries := []query.Query{teamIDQ}

	if term != "" {
		nameSuggestQ := bleve.NewPrefixQuery(strings.ToLower(term))
		nameSuggestQ.SetField("NameSuggest")
		queries = append(queries, nameSuggestQ)
	}

	query := bleve.NewSearchRequest(bleve.NewConjunctionQuery(queries...))
	query.Size = model.ChannelSearchDefaultLimit
	results, err := b.ChannelIndex.Search(query)
	if err != nil {
		return nil, model.NewAppError("Bleveengine.SearchChannels", "bleveengine.search_channels.error", nil, err.Error(), http.StatusInternalServerError)
	}

	channelIDs := []string{}
	for _, result := range results.Hits {
		channelIDs = append(channelIDs, result.ID)
	}

	return channelIDs, nil
}

func (b *BleveEngine) DeleteChannel(channel *model.Channel) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	if err := b.ChannelIndex.Delete(channel.ID); err != nil {
		return model.NewAppError("Bleveengine.DeleteChannel", "bleveengine.delete_channel.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) IndexUser(user *model.User, teamsIDs, channelsIDs []string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	blvUser := BLVUserFromUserAndTeams(user, teamsIDs, channelsIDs)
	if err := b.UserIndex.Index(blvUser.ID, blvUser); err != nil {
		return model.NewAppError("Bleveengine.IndexUser", "bleveengine.index_user.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) SearchUsersInChannel(teamID, channelID string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, []string, *model.AppError) {
	if restrictedToChannels != nil && len(restrictedToChannels) == 0 {
		return []string{}, []string{}, nil
	}

	// users in channel
	var queries []query.Query
	if term != "" {
		termQ := bleve.NewPrefixQuery(strings.ToLower(term))
		if options.AllowFullNames {
			termQ.SetField("SuggestionsWithFullname")
		} else {
			termQ.SetField("SuggestionsWithoutFullname")
		}
		queries = append(queries, termQ)
	}

	channelIDQ := bleve.NewTermQuery(channelID)
	channelIDQ.SetField("ChannelsIds")
	queries = append(queries, channelIDQ)

	query := bleve.NewConjunctionQuery(queries...)

	uchanSearch := bleve.NewSearchRequest(query)
	uchanSearch.Size = options.Limit
	uchan, err := b.UserIndex.Search(uchanSearch)
	if err != nil {
		return nil, nil, model.NewAppError("Bleveengine.SearchUsersInChannel", "bleveengine.search_users_in_channel.uchan.error", nil, err.Error(), http.StatusInternalServerError)
	}

	// users not in channel
	boolQ := bleve.NewBooleanQuery()

	if term != "" {
		termQ := bleve.NewPrefixQuery(strings.ToLower(term))
		if options.AllowFullNames {
			termQ.SetField("SuggestionsWithFullname")
		} else {
			termQ.SetField("SuggestionsWithoutFullname")
		}
		boolQ.AddMust(termQ)
	}

	teamIDQ := bleve.NewTermQuery(teamID)
	teamIDQ.SetField("TeamsIds")
	boolQ.AddMust(teamIDQ)

	outsideChannelIDQ := bleve.NewTermQuery(channelID)
	outsideChannelIDQ.SetField("ChannelsIds")
	boolQ.AddMustNot(outsideChannelIDQ)

	if len(restrictedToChannels) > 0 {
		restrictedChannelsQ := bleve.NewDisjunctionQuery()
		for _, channelID := range restrictedToChannels {
			restrictedChannelQ := bleve.NewTermQuery(channelID)
			restrictedChannelsQ.AddQuery(restrictedChannelQ)
		}
		boolQ.AddMust(restrictedChannelsQ)
	}

	nuchanSearch := bleve.NewSearchRequest(boolQ)
	nuchanSearch.Size = options.Limit
	nuchan, err := b.UserIndex.Search(nuchanSearch)
	if err != nil {
		return nil, nil, model.NewAppError("Bleveengine.SearchUsersInChannel", "bleveengine.search_users_in_channel.nuchan.error", nil, err.Error(), http.StatusInternalServerError)
	}

	uchanIDs := []string{}
	for _, result := range uchan.Hits {
		uchanIDs = append(uchanIDs, result.ID)
	}

	nuchanIDs := []string{}
	for _, result := range nuchan.Hits {
		nuchanIDs = append(nuchanIDs, result.ID)
	}

	return uchanIDs, nuchanIDs, nil
}

func (b *BleveEngine) SearchUsersInTeam(teamID string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, *model.AppError) {
	if restrictedToChannels != nil && len(restrictedToChannels) == 0 {
		return []string{}, nil
	}

	var rootQ query.Query
	if term == "" && teamID == "" && restrictedToChannels == nil {
		rootQ = bleve.NewMatchAllQuery()
	} else {
		boolQ := bleve.NewBooleanQuery()

		if term != "" {
			termQ := bleve.NewPrefixQuery(strings.ToLower(term))
			if options.AllowFullNames {
				termQ.SetField("SuggestionsWithFullname")
			} else {
				termQ.SetField("SuggestionsWithoutFullname")
			}
			boolQ.AddMust(termQ)
		}

		if len(restrictedToChannels) > 0 {
			// restricted channels are already filtered by team, so we
			// can search only those matches
			restrictedChannelsQ := []query.Query{}
			for _, channelID := range restrictedToChannels {
				channelIDQ := bleve.NewTermQuery(channelID)
				channelIDQ.SetField("ChannelsIds")
				restrictedChannelsQ = append(restrictedChannelsQ, channelIDQ)
			}
			boolQ.AddMust(bleve.NewDisjunctionQuery(restrictedChannelsQ...))
		} else {
			// this means that we only need to restrict by team
			if teamID != "" {
				teamIDQ := bleve.NewTermQuery(teamID)
				teamIDQ.SetField("TeamsIds")
				boolQ.AddMust(teamIDQ)
			}
		}

		rootQ = boolQ
	}

	search := bleve.NewSearchRequest(rootQ)
	search.Size = options.Limit
	results, err := b.UserIndex.Search(search)
	if err != nil {
		return nil, model.NewAppError("Bleveengine.SearchUsersInTeam", "bleveengine.search_users_in_team.error", nil, err.Error(), http.StatusInternalServerError)
	}

	usersIDs := []string{}
	for _, r := range results.Hits {
		usersIDs = append(usersIDs, r.ID)
	}

	return usersIDs, nil
}

func (b *BleveEngine) DeleteUser(user *model.User) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	if err := b.UserIndex.Delete(user.ID); err != nil {
		return model.NewAppError("Bleveengine.DeleteUser", "bleveengine.delete_user.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) IndexFile(file *model.FileInfo, channelID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	blvFile := BLVFileFromFileInfo(file, channelID)
	if err := b.FileIndex.Index(blvFile.ID, blvFile); err != nil {
		return model.NewAppError("Bleveengine.IndexFile", "bleveengine.index_file.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) SearchFiles(channels *model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, *model.AppError) {
	channelQueries := []query.Query{}
	for _, channel := range *channels {
		channelIDQ := bleve.NewTermQuery(channel.ID)
		channelIDQ.SetField("ChannelId")
		channelQueries = append(channelQueries, channelIDQ)
	}
	channelDisjunctionQ := bleve.NewDisjunctionQuery(channelQueries...)

	var termQueries []query.Query
	var notTermQueries []query.Query
	var filters []query.Query
	var notFilters []query.Query

	for i, params := range searchParams {
		var termOperator query.MatchQueryOperator = query.MatchQueryOperatorAnd
		if searchParams[0].OrTerms {
			termOperator = query.MatchQueryOperatorOr
		}

		// Date, channels and FromUsers filters come in all
		// searchParams iteration, and as they are global to the
		// query, we only need to process them once
		if i == 0 {
			if len(params.InChannels) > 0 {
				inChannels := []query.Query{}
				for _, channelID := range params.InChannels {
					channelQ := bleve.NewTermQuery(channelID)
					channelQ.SetField("ChannelId")
					inChannels = append(inChannels, channelQ)
				}
				filters = append(filters, bleve.NewDisjunctionQuery(inChannels...))
			}

			if len(params.ExcludedChannels) > 0 {
				excludedChannels := []query.Query{}
				for _, channelID := range params.ExcludedChannels {
					channelQ := bleve.NewTermQuery(channelID)
					channelQ.SetField("ChannelId")
					excludedChannels = append(excludedChannels, channelQ)
				}
				notFilters = append(notFilters, bleve.NewDisjunctionQuery(excludedChannels...))
			}

			if len(params.FromUsers) > 0 {
				fromUsers := []query.Query{}
				for _, userID := range params.FromUsers {
					userQ := bleve.NewTermQuery(userID)
					userQ.SetField("CreatorId")
					fromUsers = append(fromUsers, userQ)
				}
				filters = append(filters, bleve.NewDisjunctionQuery(fromUsers...))
			}

			if len(params.ExcludedUsers) > 0 {
				excludedUsers := []query.Query{}
				for _, userID := range params.ExcludedUsers {
					userQ := bleve.NewTermQuery(userID)
					userQ.SetField("CreatorId")
					excludedUsers = append(excludedUsers, userQ)
				}
				notFilters = append(notFilters, bleve.NewDisjunctionQuery(excludedUsers...))
			}

			if len(params.Extensions) > 0 {
				extensions := []query.Query{}
				for _, extension := range params.Extensions {
					extensionQ := bleve.NewTermQuery(extension)
					extensionQ.SetField("Extension")
					extensions = append(extensions, extensionQ)
				}
				filters = append(filters, bleve.NewDisjunctionQuery(extensions...))
			}

			if len(params.ExcludedExtensions) > 0 {
				excludedExtensions := []query.Query{}
				for _, extension := range params.ExcludedExtensions {
					extensionQ := bleve.NewTermQuery(extension)
					extensionQ.SetField("Extension")
					excludedExtensions = append(excludedExtensions, extensionQ)
				}
				notFilters = append(notFilters, bleve.NewDisjunctionQuery(excludedExtensions...))
			}

			if params.OnDate != "" {
				before, after := params.GetOnDateMillis()
				beforeFloat64 := float64(before)
				afterFloat64 := float64(after)
				onDateQ := bleve.NewNumericRangeQuery(&beforeFloat64, &afterFloat64)
				onDateQ.SetField("CreateAt")
				filters = append(filters, onDateQ)
			} else {
				if params.AfterDate != "" || params.BeforeDate != "" {
					var min, max *float64
					if params.AfterDate != "" {
						minf := float64(params.GetAfterDateMillis())
						min = &minf
					}

					if params.BeforeDate != "" {
						maxf := float64(params.GetBeforeDateMillis())
						max = &maxf
					}

					dateQ := bleve.NewNumericRangeQuery(min, max)
					dateQ.SetField("CreateAt")
					filters = append(filters, dateQ)
				}

				if params.ExcludedAfterDate != "" {
					minf := float64(params.GetExcludedAfterDateMillis())
					dateQ := bleve.NewNumericRangeQuery(&minf, nil)
					dateQ.SetField("CreateAt")
					notFilters = append(notFilters, dateQ)
				}

				if params.ExcludedBeforeDate != "" {
					maxf := float64(params.GetExcludedBeforeDateMillis())
					dateQ := bleve.NewNumericRangeQuery(nil, &maxf)
					dateQ.SetField("CreateAt")
					notFilters = append(notFilters, dateQ)
				}

				if params.ExcludedDate != "" {
					before, after := params.GetExcludedDateMillis()
					beforef := float64(before)
					afterf := float64(after)
					onDateQ := bleve.NewNumericRangeQuery(&beforef, &afterf)
					onDateQ.SetField("CreateAt")
					notFilters = append(notFilters, onDateQ)
				}
			}
		}

		if params.Terms != "" {
			terms := []string{}
			for _, term := range strings.Split(params.Terms, " ") {
				if strings.HasSuffix(term, "*") {
					nameQ := bleve.NewWildcardQuery(term)
					nameQ.SetField("Name")
					contentQ := bleve.NewWildcardQuery(term)
					contentQ.SetField("Content")
					termQueries = append(termQueries, bleve.NewDisjunctionQuery(nameQ, contentQ))
				} else {
					terms = append(terms, term)
				}
			}

			if len(terms) > 0 {
				nameQ := bleve.NewMatchQuery(strings.Join(terms, " "))
				nameQ.SetField("Name")
				nameQ.SetOperator(termOperator)
				contentQ := bleve.NewMatchQuery(strings.Join(terms, " "))
				contentQ.SetField("Content")
				contentQ.SetOperator(termOperator)
				termQueries = append(termQueries, bleve.NewDisjunctionQuery(nameQ, contentQ))
			}
		}

		if params.ExcludedTerms != "" {
			nameQ := bleve.NewMatchQuery(params.ExcludedTerms)
			nameQ.SetField("Name")
			nameQ.SetOperator(termOperator)
			contentQ := bleve.NewMatchQuery(params.ExcludedTerms)
			contentQ.SetField("Content")
			contentQ.SetOperator(termOperator)
			notTermQueries = append(notTermQueries, bleve.NewDisjunctionQuery(nameQ, contentQ))
		}
	}

	allTermsQ := bleve.NewBooleanQuery()
	allTermsQ.AddMustNot(notTermQueries...)
	if searchParams[0].OrTerms {
		allTermsQ.AddShould(termQueries...)
	} else {
		allTermsQ.AddMust(termQueries...)
	}

	query := bleve.NewBooleanQuery()
	query.AddMust(channelDisjunctionQ)

	if len(termQueries) > 0 || len(notTermQueries) > 0 {
		query.AddMust(allTermsQ)
	}

	if len(filters) > 0 {
		query.AddMust(bleve.NewConjunctionQuery(filters...))
	}
	if len(notFilters) > 0 {
		query.AddMustNot(notFilters...)
	}

	search := bleve.NewSearchRequestOptions(query, perPage, page*perPage, false)
	search.SortBy([]string{"-CreateAt"})
	results, err := b.FileIndex.Search(search)
	if err != nil {
		return nil, model.NewAppError("Bleveengine.SearchFiles", "bleveengine.search_files.error", nil, err.Error(), http.StatusInternalServerError)
	}

	fileIDs := []string{}

	for _, r := range results.Hits {
		fileIDs = append(fileIDs, r.ID)
	}

	return fileIDs, nil
}

func (b *BleveEngine) DeleteFile(fileID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	if err := b.FileIndex.Delete(fileID); err != nil {
		return model.NewAppError("Bleveengine.DeleteFile", "bleveengine.delete_file.error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (b *BleveEngine) deleteFiles(searchRequest *bleve.SearchRequest, batchSize int) (int64, error) {
	resultsCount := int64(0)

	for {
		// As we are deleting the files after fetching them, we need to keep
		// From fixed always to 0
		searchRequest.From = 0
		searchRequest.Size = batchSize
		results, err := b.FileIndex.Search(searchRequest)
		if err != nil {
			return -1, err
		}
		batch := b.FileIndex.NewBatch()
		for _, file := range results.Hits {
			batch.Delete(file.ID)
		}
		if err := b.FileIndex.Batch(batch); err != nil {
			return -1, err
		}
		resultsCount += int64(results.Hits.Len())
		if results.Hits.Len() < batchSize {
			break
		}
	}

	return resultsCount, nil
}

func (b *BleveEngine) DeleteUserFiles(userID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	query := bleve.NewTermQuery(userID)
	query.SetField("CreatorId")
	search := bleve.NewSearchRequest(query)
	deleted, err := b.deleteFiles(search, DeleteFilesBatchSize)
	if err != nil {
		return model.NewAppError("Bleveengine.DeleteUserFiles",
			"bleveengine.delete_user_files.error", nil,
			err.Error(), http.StatusInternalServerError)
	}

	mlog.Info("Files for user deleted", mlog.String("user_id", userID), mlog.Int64("deleted", deleted))

	return nil
}

func (b *BleveEngine) DeletePostFiles(postID string) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	query := bleve.NewTermQuery(postID)
	query.SetField("PostId")
	search := bleve.NewSearchRequest(query)
	deleted, err := b.deleteFiles(search, DeleteFilesBatchSize)
	if err != nil {
		return model.NewAppError("Bleveengine.DeletePostFiles",
			"bleveengine.delete_post_files.error", nil,
			err.Error(), http.StatusInternalServerError)
	}

	mlog.Info("Files for post deleted", mlog.String("post_id", postID), mlog.Int64("deleted", deleted))

	return nil
}

func (b *BleveEngine) DeleteFilesBatch(endTime, limit int64) *model.AppError {
	b.Mutex.RLock()
	defer b.Mutex.RUnlock()

	endTimeFloat := float64(endTime)
	query := bleve.NewNumericRangeQuery(nil, &endTimeFloat)
	query.SetField("CreateAt")
	search := bleve.NewSearchRequestOptions(query, int(limit), 0, false)
	search.SortBy([]string{"-CreateAt"})

	deleted, err := b.deleteFiles(search, DeleteFilesBatchSize)
	if err != nil {
		return model.NewAppError("Bleveengine.DeleteFilesBatch",
			"bleveengine.delete_files_batch.error", nil,
			err.Error(), http.StatusInternalServerError)
	}

	mlog.Info("Files in batch deleted", mlog.Int64("endTime", endTime), mlog.Int64("limit", limit), mlog.Int64("deleted", deleted))

	return nil
}
