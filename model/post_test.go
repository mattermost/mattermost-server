// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"io/ioutil"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostToJson(t *testing.T) {
	o := Post{Id: NewId(), Message: NewId()}
	j := o.ToJson()
	ro := PostFromJson(strings.NewReader(j))

	assert.NotNil(t, ro)
	assert.Equal(t, &o, ro.Clone())
}

func TestPostFromJsonError(t *testing.T) {
	ro := PostFromJson(strings.NewReader(""))
	assert.Nil(t, ro)
}

func TestPostIsValid(t *testing.T) {
	o := Post{}
	maxPostSize := 10000

	err := o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.Id = NewId()
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.CreateAt = GetMillis()
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.UpdateAt = GetMillis()
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.UserId = NewId()
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.ChannelId = NewId()
	o.RootId = "123"
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.RootId = ""
	o.ParentId = "123"
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.ParentId = NewId()
	o.RootId = ""
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.ParentId = ""
	o.Message = strings.Repeat("0", maxPostSize+1)
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.Message = strings.Repeat("0", maxPostSize)
	err = o.IsValid(maxPostSize)
	require.Nil(t, err)

	o.Message = "test"
	err = o.IsValid(maxPostSize)
	require.Nil(t, err)
	o.Type = "junk"
	err = o.IsValid(maxPostSize)
	require.NotNil(t, err)

	o.Type = PostCustomTypePrefix + "type"
	err = o.IsValid(maxPostSize)
	require.Nil(t, err)
}

func TestPostPreSave(t *testing.T) {
	o := Post{Message: "test"}
	o.PreSave()

	require.NotEqual(t, 0, o.CreateAt)

	past := GetMillis() - 1
	o = Post{Message: "test", CreateAt: past}
	o.PreSave()

	require.LessOrEqual(t, o.CreateAt, past)

	o.Etag()
}

func TestPostIsSystemMessage(t *testing.T) {
	post1 := Post{Message: "test_1"}
	post1.PreSave()

	require.False(t, post1.IsSystemMessage())

	post2 := Post{Message: "test_2", Type: PostTypeJoinChannel}
	post2.PreSave()

	require.True(t, post2.IsSystemMessage())
}

func TestPostChannelMentions(t *testing.T) {
	post := Post{Message: "~a ~b ~b ~c/~d."}
	assert.Equal(t, []string{"a", "b", "c", "d"}, post.ChannelMentions())
}

func TestPostSanitizeProps(t *testing.T) {
	post1 := &Post{
		Message: "test",
	}

	post1.SanitizeProps()

	require.Nil(t, post1.GetProp(PropsAddChannelMember))

	post2 := &Post{
		Message: "test",
		Props: StringInterface{
			PropsAddChannelMember: "test",
		},
	}

	post2.SanitizeProps()

	require.Nil(t, post2.GetProp(PropsAddChannelMember))

	post3 := &Post{
		Message: "test",
		Props: StringInterface{
			PropsAddChannelMember: "no good",
			"attachments":         "good",
		},
	}

	post3.SanitizeProps()

	require.Nil(t, post3.GetProp(PropsAddChannelMember))

	require.NotNil(t, post3.GetProp("attachments"))
}

func TestPost_AttachmentsEqual(t *testing.T) {
	post1 := &Post{}
	post2 := &Post{}
	for name, tc := range map[string]struct {
		Attachments1 []*SlackAttachment
		Attachments2 []*SlackAttachment
		Expected     bool
	}{
		"Empty": {
			nil,
			nil,
			true,
		},
		"DifferentLength": {
			[]*SlackAttachment{
				{
					Text: "Hello World",
				},
			},
			nil,
			false,
		},
		"EqualText": {
			[]*SlackAttachment{
				{
					Text: "Hello World",
				},
			},
			[]*SlackAttachment{
				{
					Text: "Hello World",
				},
			},
			true,
		},
		"DifferentText": {
			[]*SlackAttachment{
				{
					Text: "Hello World",
				},
			},
			[]*SlackAttachment{
				{
					Text: "Hello World 2",
				},
			},
			false,
		},
		"DifferentColor": {
			[]*SlackAttachment{
				{
					Text:  "Hello World",
					Color: "#152313",
				},
			},
			[]*SlackAttachment{
				{
					Text: "Hello World 2",
				},
			},
			false,
		},
		"EqualFields": {
			[]*SlackAttachment{
				{
					Fields: []*SlackAttachmentField{
						{
							Title: "Hello World",
							Value: "FooBar",
						},
						{
							Title: "Hello World2",
							Value: "FooBar2",
						},
					},
				},
			},
			[]*SlackAttachment{
				{
					Fields: []*SlackAttachmentField{
						{
							Title: "Hello World",
							Value: "FooBar",
						},
						{
							Title: "Hello World2",
							Value: "FooBar2",
						},
					},
				},
			},
			true,
		},
		"DifferentFields": {
			[]*SlackAttachment{
				{
					Fields: []*SlackAttachmentField{
						{
							Title: "Hello World",
							Value: "FooBar",
						},
					},
				},
			},
			[]*SlackAttachment{
				{
					Fields: []*SlackAttachmentField{
						{
							Title: "Hello World",
							Value: "FooBar",
							Short: false,
						},
						{
							Title: "Hello World2",
							Value: "FooBar2",
							Short: true,
						},
					},
				},
			},
			false,
		},
		"EqualActions": {
			[]*SlackAttachment{
				{
					Actions: []*PostAction{
						{
							Name: "FooBar",
							Options: []*PostActionOptions{
								{
									Text:  "abcdef",
									Value: "abcdef",
								},
							},
							Integration: &PostActionIntegration{
								URL: "http://localhost",
								Context: map[string]interface{}{
									"context": "foobar",
									"test":    123,
								},
							},
						},
					},
				},
			},
			[]*SlackAttachment{
				{
					Actions: []*PostAction{
						{
							Name: "FooBar",
							Options: []*PostActionOptions{
								{
									Text:  "abcdef",
									Value: "abcdef",
								},
							},
							Integration: &PostActionIntegration{
								URL: "http://localhost",
								Context: map[string]interface{}{
									"context": "foobar",
									"test":    123,
								},
							},
						},
					},
				},
			},
			true,
		},
		"DifferentActions": {
			[]*SlackAttachment{
				{
					Actions: []*PostAction{
						{
							Name: "FooBar",
							Options: []*PostActionOptions{
								{
									Text:  "abcdef",
									Value: "abcdef",
								},
							},
							Integration: &PostActionIntegration{
								URL: "http://localhost",
								Context: map[string]interface{}{
									"context": "foobar",
									"test":    "mattermost",
								},
							},
						},
					},
				},
			},
			[]*SlackAttachment{
				{
					Actions: []*PostAction{
						{
							Name: "FooBar",
							Options: []*PostActionOptions{
								{
									Text:  "abcdef",
									Value: "abcdef",
								},
							},
							Integration: &PostActionIntegration{
								URL: "http://localhost",
								Context: map[string]interface{}{
									"context": "foobar",
									"test":    123,
								},
							},
						},
					},
				},
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			post1.AddProp("attachments", tc.Attachments1)
			post2.AddProp("attachments", tc.Attachments2)
			assert.Equal(t, tc.Expected, post1.AttachmentsEqual(post2))
		})
	}
}

var markdownSample, markdownSampleWithRewrittenImageURLs string

func init() {
	bytes, err := ioutil.ReadFile("testdata/markdown-sample.md")
	if err != nil {
		panic(err)
	}
	markdownSample = string(bytes)

	bytes, err = ioutil.ReadFile("testdata/markdown-sample-with-rewritten-image-urls.md")
	if err != nil {
		panic(err)
	}
	markdownSampleWithRewrittenImageURLs = string(bytes)
}

func TestRewriteImageURLs(t *testing.T) {
	for name, tc := range map[string]struct {
		Markdown string
		Expected string
	}{
		"Empty": {
			Markdown: ``,
			Expected: ``,
		},
		"NoImages": {
			Markdown: `foo`,
			Expected: `foo`,
		},
		"Link": {
			Markdown: `[foo](/url)`,
			Expected: `[foo](/url)`,
		},
		"Image": {
			Markdown: `![foo](/url)`,
			Expected: `![foo](rewritten:/url)`,
		},
		"SpacedURL": {
			Markdown: `![foo]( /url )`,
			Expected: `![foo]( rewritten:/url )`,
		},
		"Title": {
			Markdown: `![foo](/url "title")`,
			Expected: `![foo](rewritten:/url "title")`,
		},
		"Parentheses": {
			Markdown: `![foo](/url(1) "title")`,
			Expected: `![foo](rewritten:/url\(1\) "title")`,
		},
		"AngleBrackets": {
			Markdown: `![foo](</url\<1\>\\> "title")`,
			Expected: `![foo](<rewritten:/url\<1\>\\> "title")`,
		},
		"MultipleLines": {
			Markdown: `![foo](
				</url\<1\>\\>
				"title"
			)`,
			Expected: `![foo](
				<rewritten:/url\<1\>\\>
				"title"
			)`,
		},
		"ReferenceLink": {
			Markdown: `[foo]: </url\<1\>\\> "title"
		 		[foo]`,
			Expected: `[foo]: </url\<1\>\\> "title"
		 		[foo]`,
		},
		"ReferenceImage": {
			Markdown: `[foo]: </url\<1\>\\> "title"
		 		![foo]`,
			Expected: `[foo]: <rewritten:/url\<1\>\\> "title"
		 		![foo]`,
		},
		"MultipleReferenceImages": {
			Markdown: `[foo]: </url1> "title"
				[bar]: </url2>
				[baz]: /url3 "title"
				[qux]: /url4
				![foo]![qux]`,
			Expected: `[foo]: <rewritten:/url1> "title"
				[bar]: </url2>
				[baz]: /url3 "title"
				[qux]: rewritten:/url4
				![foo]![qux]`,
		},
		"DuplicateReferences": {
			Markdown: `[foo]: </url1> "title"
				[foo]: </url2>
				[foo]: /url3 "title"
				[foo]: /url4
				![foo]![foo]![foo]`,
			Expected: `[foo]: <rewritten:/url1> "title"
				[foo]: </url2>
				[foo]: /url3 "title"
				[foo]: /url4
				![foo]![foo]![foo]`,
		},
		"TrailingURL": {
			Markdown: "![foo]\n\n[foo]: /url",
			Expected: "![foo]\n\n[foo]: rewritten:/url",
		},
		"Sample": {
			Markdown: markdownSample,
			Expected: markdownSampleWithRewrittenImageURLs,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.Expected, RewriteImageURLs(tc.Markdown, func(url string) string {
				return "rewritten:" + url
			}))
		})
	}
}

var rewriteImageURLsSink string

func BenchmarkRewriteImageURLs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rewriteImageURLsSink = RewriteImageURLs(markdownSample, func(url string) string {
			return "rewritten:" + url
		})
	}
}

func TestPostShallowCopy(t *testing.T) {
	var dst *Post
	p := &Post{
		Id: NewId(),
	}

	err := p.ShallowCopy(dst)
	require.Error(t, err)

	dst = &Post{}
	err = p.ShallowCopy(dst)
	require.NoError(t, err)
	require.Equal(t, p, dst)
	require.Condition(t, func() bool {
		return p != dst
	})
}

func TestPostClone(t *testing.T) {
	p := &Post{
		Id: NewId(),
	}

	pp := p.Clone()
	require.Equal(t, p, pp)
	require.Condition(t, func() bool {
		return p != pp
	})
	require.Condition(t, func() bool {
		return &p.propsMu != &pp.propsMu
	})
}

func BenchmarkClonePost(b *testing.B) {
	p := Post{}
	for i := 0; i < b.N; i++ {
		_ = p.Clone()
	}
}

func BenchmarkPostPropsGet_indirect(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	for i := 0; i < b.N; i++ {
		_ = p.GetProps()
	}
}

func BenchmarkPostPropsGet_direct(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	for i := 0; i < b.N; i++ {
		_ = p.Props
	}
}

func BenchmarkPostPropsAdd_indirect(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	for i := 0; i < b.N; i++ {
		p.AddProp("test", "somevalue")
	}
}

func BenchmarkPostPropsAdd_direct(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	for i := 0; i < b.N; i++ {
		p.Props["test"] = "somevalue"
	}
}

func BenchmarkPostPropsDel_indirect(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	p.AddProp("test", "somevalue")
	for i := 0; i < b.N; i++ {
		p.DelProp("test")
	}
}

func BenchmarkPostPropsDel_direct(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	for i := 0; i < b.N; i++ {
		delete(p.Props, "test")
	}
}

func BenchmarkPostPropGet_direct(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	p.Props["somekey"] = "somevalue"
	for i := 0; i < b.N; i++ {
		_ = p.Props["somekey"]
	}
}

func BenchmarkPostPropGet_indirect(b *testing.B) {
	p := Post{
		Props: make(StringInterface),
	}
	p.Props["somekey"] = "somevalue"
	for i := 0; i < b.N; i++ {
		_ = p.GetProp("somekey")
	}
}

// TestPostPropsDataRace tries to trigger data race conditions related to Post.Props.
// It's meant to be run with the -race flag.
func TestPostPropsDataRace(t *testing.T) {
	p := Post{Message: "test"}

	wg := sync.WaitGroup{}
	wg.Add(7)

	go func() {
		for i := 0; i < 100; i++ {
			p.AddProp("test", "test")
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = p.GetProp("test")
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			p.AddProp("test", "test2")
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = p.GetProps()["test"]
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			p.DelProp("test")
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			p.SetProps(make(StringInterface))
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = p.Clone()
		}
		wg.Done()
	}()

	wg.Wait()
}

func Test_findAtChannelMention(t *testing.T) {
	testCases := []struct {
		Name    string
		Message string
		Mention string
		Found   bool
	}{
		{
			"Returns mention for @here wrapped by spaces",
			"hi guys @here wrapped by spaces",
			"@here",
			true,
		},
		{
			"Returns mention for @all wrapped by spaces",
			"hi guys @all wrapped by spaces",
			"@all",
			true,
		},
		{
			"Returns mention for @channel wrapped by spaces",
			"hi guys @channel wrapped by spaces",
			"@channel",
			true,
		},
		{
			"Returns mention for @here wrapped by dash",
			"-@here-",
			"@here",
			true,
		},
		{
			"Returns mention for @all wrapped by back tick",
			"`@all`",
			"@all",
			true,
		},
		{
			"Returns mention for @channel wrapped by tags",
			"<@channel>",
			"@channel",
			true,
		},
		{
			"Returns mention for @channel wrapped by asterisks",
			"*@channel*",
			"@channel",
			true,
		},
		{
			"Does not return mention when prefixed by letters",
			"hi@channel",
			"",
			false,
		},
		{
			"Does not return mention when suffixed by letters",
			"hi @channelanotherword",
			"",
			false,
		},
		{
			"Returns mention when prefixed by word ending in special character",
			"hi-@channel",
			"@channel",
			true,
		},
		{
			"Returns mention when suffixed by word starting in special character",
			"hi @channel-guys",
			"@channel",
			true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			mention, found := findAtChannelMention(tc.Message)
			assert.Equal(t, tc.Mention, mention)
			assert.Equal(t, tc.Found, found)
		})
	}
}

func TestPostDisableMentionHighlights(t *testing.T) {
	post := &Post{}

	testCases := []struct {
		Name            string
		Message         string
		ExpectedProps   StringInterface
		ExpectedMention string
	}{
		{
			"Does nothing for post with no mentions",
			"Sample message with no mentions",
			StringInterface(nil),
			"",
		},
		{
			"Sets PostPropsMentionHighlightDisabled and returns mention",
			"Sample message with @here",
			StringInterface{PostPropsMentionHighlightDisabled: true},
			"@here",
		},
		{
			"Sets PostPropsMentionHighlightDisabled and returns mention",
			"Sample message with @channel",
			StringInterface{PostPropsMentionHighlightDisabled: true},
			"@channel",
		},
		{
			"Sets PostPropsMentionHighlightDisabled and returns mention",
			"Sample message with @all",
			StringInterface{PostPropsMentionHighlightDisabled: true},
			"@all",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			post.Message = tc.Message
			mention := post.DisableMentionHighlights()
			assert.Equal(t, tc.ExpectedMention, mention)
			assert.Equal(t, tc.ExpectedProps, post.Props)
			post.Props = StringInterface{}
		})
	}
}

func TestPostPatchDisableMentionHighlights(t *testing.T) {
	patch := &PostPatch{}

	testCases := []struct {
		Name          string
		Message       string
		ExpectedProps *StringInterface
	}{
		{
			"Does nothing for post with no mentions",
			"Sample message with no mentions",
			nil,
		},
		{
			"Sets PostPropsMentionHighlightDisabled",
			"Sample message with @here",
			&StringInterface{PostPropsMentionHighlightDisabled: true},
		},
		{
			"Sets PostPropsMentionHighlightDisabled",
			"Sample message with @channel",
			&StringInterface{PostPropsMentionHighlightDisabled: true},
		},
		{
			"Sets PostPropsMentionHighlightDisabled",
			"Sample message with @all",
			&StringInterface{PostPropsMentionHighlightDisabled: true},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			message := tc.Message
			patch.Message = &message
			patch.DisableMentionHighlights()
			if tc.ExpectedProps == nil {
				assert.Nil(t, patch.Props)
			} else {
				assert.Equal(t, *tc.ExpectedProps, *patch.Props)
			}
			patch.Props = nil
		})
	}

	t.Run("TestNilMessage", func(t *testing.T) {
		patch.Message = nil
		patch.DisableMentionHighlights()
		// Useless assertion to prevent compiler elision.
		assert.Nil(t, patch.Message)
	})
}

func TestSearchParameterFromJson(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		params, err := SearchParameterFromJson(strings.NewReader(""))
		require.Nil(t, params)
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		params, err := SearchParameterFromJson(strings.NewReader("invalid"))
		require.Nil(t, params)
		require.Error(t, err)
	})

	t.Run("valid empty input", func(t *testing.T) {
		params, err := SearchParameterFromJson(strings.NewReader("{}"))
		require.NoError(t, err)
		require.NotNil(t, params)
		require.Equal(t, &SearchParameter{}, params)
	})

	t.Run("valid non-empty input", func(t *testing.T) {
		params, err := SearchParameterFromJson(strings.NewReader("{\"terms\": \"test\"}"))
		require.NoError(t, err)
		require.NotNil(t, params)
		require.Equal(t, "test", *params.Terms)
	})
}

func TestPostAttachments(t *testing.T) {
	p := &Post{
		Props: map[string]interface{}{
			"attachments": []byte(`[{
				"actions" : {null}
			}]
			`),
		},
	}

	t.Run("empty actions", func(t *testing.T) {
		p.Props["attachments"] = []interface{}{
			map[string]interface{}{"actions": []interface{}{}},
		}
		attachments := p.Attachments()
		require.Empty(t, attachments[0].Actions)
	})

	t.Run("a couple of actions", func(t *testing.T) {
		p.Props["attachments"] = []interface{}{
			map[string]interface{}{"actions": []interface{}{
				map[string]interface{}{"id": "test1"}, map[string]interface{}{"id": "test2"}},
			},
		}

		attachments := p.Attachments()
		require.Len(t, attachments[0].Actions, 2)
		require.Equal(t, attachments[0].Actions[0].Id, "test1")
		require.Equal(t, attachments[0].Actions[1].Id, "test2")
	})

	t.Run("should ignore null actions", func(t *testing.T) {
		p.Props["attachments"] = []interface{}{
			map[string]interface{}{"actions": []interface{}{
				map[string]interface{}{"id": "test1"}, nil, map[string]interface{}{"id": "test2"}, nil, nil},
			},
		}

		attachments := p.Attachments()
		require.Len(t, attachments[0].Actions, 2)
		require.Equal(t, attachments[0].Actions[0].Id, "test1")
		require.Equal(t, attachments[0].Actions[1].Id, "test2")
	})
}
