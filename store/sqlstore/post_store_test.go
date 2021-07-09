// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/store/searchtest"
	"github.com/mattermost/mattermost-server/v5/store/storetest"
)

func TestPostStore(t *testing.T) {
	StoreTestWithSQLStore(t, storetest.TestPostStore)
}

func TestSearchPostStore(t *testing.T) {
	StoreTestWithSearchTestEngine(t, searchtest.TestSearchPostStore)
}

func TestMysqlStopWords(t *testing.T) {
	mysqlStopWordsTests := []struct {
		Name     string
		Args     []string
		Expected []string
		Empty    bool
	}{
		{
			Name:     "Should remove only the stop words",
			Args:     []string{"where is my car", "so this is real", "test this-and-that is awesome"},
			Expected: []string{"my car", "so real", "test this-and-that awesome"},
		},
		{
			Name:     "Should not remove part of a word containing stop words",
			Args:     []string{"whereabouts", "wherein", "tothis", "thisorthat", "waswhen", "whowas", "inthe", "whowill", "thewww"},
			Expected: []string{"whereabouts", "wherein", "tothis", "thisorthat", "waswhen", "whowas", "inthe", "whowill", "thewww"},
		},
		{
			Name:  "Should remove all words from terms",
			Args:  []string{"where about", "where in", "to this", "this or that", "was when", "who was", "in the", "who will", "the www"},
			Empty: true,
		},
		{
			Name:     "Should not remove part of a word containing stop words separated by hyphens",
			Args:     []string{"where-about", "where-in", "to-this", "this-or-that", "was-when", "who-was", "in-the", "who-will", "the-www"},
			Expected: []string{"where-about", "where-in", "to-this", "this-or-that", "was-when", "who-was", "in-the", "who-will", "the-www"},
		},
	}

	for _, tc := range mysqlStopWordsTests {
		t.Run(tc.Name, func(t *testing.T) {
			for i, term := range tc.Args {
				got, err := removeMysqlStopWordsFromTerms(term)
				require.NoError(t, err)
				if tc.Empty {
					require.Empty(t, got)
				} else {
					require.Equal(t, tc.Expected[i], got)
				}
			}
		})
	}
}
