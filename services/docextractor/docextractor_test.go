// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docextractor

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/utils/testutils"
)

func TestExtract(t *testing.T) {
	testCases := []struct {
		Name         string
		TestFileName string
		Settings     ExtractSettings
		Contains     []string
		NotContains  []string
		ExpectError  bool
	}{
		{
			"Plain text file",
			"test-markdown-basics.md",
			ExtractSettings{},
			[]string{"followed", "separated", "Basic"},
			[]string{},
			false,
		},
		{
			"Plain small text file",
			"test-hashtags.md",
			ExtractSettings{},
			[]string{"should", "render", "strings"},
			[]string{},
			false,
		},
		{
			"Zip file without recursion",
			"Fake_Team_Import.zip",
			ExtractSettings{},
			[]string{"users", "channels", "general"},
			[]string{"purpose", "announcements"},
			false,
		},
		{
			"Zip file with recursion",
			"Fake_Team_Import.zip",
			ExtractSettings{ArchiveRecursion: true},
			[]string{"users", "channels", "general", "purpose", "announcements"},
			[]string{},
			false,
		},
		{
			"Rar file without recursion",
			"Fake_Team_Import.rar",
			ExtractSettings{},
			[]string{"users", "channels", "general"},
			[]string{"purpose", "announcements"},
			false,
		},
		{
			"Rar file with recursion",
			"Fake_Team_Import.rar",
			ExtractSettings{ArchiveRecursion: true},
			[]string{"users", "channels", "general", "purpose", "announcements"},
			[]string{},
			false,
		},
		{
			"Tar.gz file without recursion",
			"Fake_Team_Import.tar.gz",
			ExtractSettings{},
			[]string{"users", "channels", "general"},
			[]string{"purpose", "announcements"},
			false,
		},
		{
			"Tar.gz file with recursion",
			"Fake_Team_Import.tar.gz",
			ExtractSettings{ArchiveRecursion: true},
			[]string{"users", "channels", "general", "purpose", "announcements"},
			[]string{},
			false,
		},
		{
			"Pdf file",
			"sample-doc.pdf",
			ExtractSettings{},
			[]string{"simple", "document", "contains"},
			[]string{},
			false,
		},
		{
			"Doc file",
			"sample-doc.doc",
			ExtractSettings{},
			[]string{"simple", "document", "contains"},
			[]string{},
			false,
		},
		{
			"Docx file",
			"sample-doc.docx",
			ExtractSettings{},
			[]string{"simple", "document", "contains"},
			[]string{},
			false,
		},
		{
			"Odt file",
			"sample-doc.odt",
			ExtractSettings{},
			[]string{"simple", "document", "contains"},
			[]string{},
			false,
		},
		{
			"Pptx file",
			"sample-doc.pptx",
			ExtractSettings{},
			[]string{"simple", "document", "contains"},
			[]string{},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			data, err := testutils.ReadTestFile(tc.TestFileName)
			require.NoError(t, err)
			text, err := Extract(tc.TestFileName, bytes.NewReader(data), tc.Settings)
			if tc.ExpectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				for _, expectedString := range tc.Contains {
					assert.Contains(t, text, expectedString)
				}
				for _, notExpectedString := range tc.NotContains {
					assert.NotContains(t, text, notExpectedString)
				}
			}
		})
	}

	t.Run("Unsupported binary file", func(t *testing.T) {
		data, err := testutils.ReadTestFile("testjpg.jpg")
		require.NoError(t, err)
		text, err := Extract("testjpg.jpg", bytes.NewReader(data), ExtractSettings{})
		require.NoError(t, err)
		require.Equal(t, "", text)
	})

	t.Run("Wrong docx extension", func(t *testing.T) {
		data, err := testutils.ReadTestFile("sample-doc.pdf")
		require.NoError(t, err)
		text, err := Extract("sample-doc.docx", bytes.NewReader(data), ExtractSettings{})
		require.NoError(t, err)
		require.Equal(t, "", text)
	})
}

type customTestPdfExtractor struct{}

func (te *customTestPdfExtractor) Match(filename string) bool {
	return strings.HasSuffix(filename, ".pdf")
}

func (te *customTestPdfExtractor) Extract(filename string, r io.ReadSeeker) (string, error) {
	return "this is a text generated content", nil
}

type failingExtractor struct{}

func (te *failingExtractor) Match(filename string) bool {
	return true
}

func (te *failingExtractor) Extract(filename string, r io.ReadSeeker) (string, error) {
	return "", errors.New("this always fail")
}

func TestExtractWithExtraExtractors(t *testing.T) {
	t.Run("overrite existing extractor", func(t *testing.T) {
		data, err := testutils.ReadTestFile("sample-doc.pdf")
		require.NoError(t, err)

		text, err := ExtractWithExtraExtractors("sample-doc.pdf", bytes.NewReader(data), ExtractSettings{}, []Extractor{&customTestPdfExtractor{}})
		require.NoError(t, err)
		require.Equal(t, text, "this is a text generated content")
	})

	t.Run("failing extractor", func(t *testing.T) {
		data, err := testutils.ReadTestFile("sample-doc.pdf")
		require.NoError(t, err)

		text, err := ExtractWithExtraExtractors("sample-doc.pdf", bytes.NewReader(data), ExtractSettings{}, []Extractor{&failingExtractor{}})
		require.NoError(t, err)
		assert.Contains(t, text, "simple")
		assert.Contains(t, text, "document")
		assert.Contains(t, text, "contains")
	})
}
