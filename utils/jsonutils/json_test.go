// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package jsonutils_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/utils/jsonutils"
)

func TestHumanizeJSONError(t *testing.T) {
	t.Parallel()

	type testType struct{}

	testCases := []struct {
		Description string
		Data        []byte
		Err         error
		ExpectedErr string
	}{
		{
			"nil error",
			[]byte{},
			nil,
			"",
		},
		{
			"non-special error",
			[]byte{},
			errors.New("test"),
			"test",
		},
		{
			"syntax error, offset 17, middle of line 3",
			[]byte("line 1\nline 2\nline 3"),
			&json.SyntaxError{
				// msg can't be set
				Offset: 17,
			},
			"parsing error at line 3, character 4: ",
		},
		{
			"unmarshal type error, offset 17, middle of line 3",
			[]byte("line 1\nline 2\nline 3"),
			&json.UnmarshalTypeError{
				Value:  "bool",
				Type:   reflect.TypeOf(testType{}),
				Offset: 17,
				Struct: "struct",
				Field:  "field",
			},
			"parsing error at line 3, character 4: json: cannot unmarshal bool into Go struct field struct.field of type jsonutils_test.testType",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Description, func(t *testing.T) {
			actual := jsonutils.HumanizeJSONError(testCase.Err, testCase.Data)
			if testCase.ExpectedErr == "" {
				assert.NoError(t, actual)
			} else {
				assert.EqualError(t, actual, testCase.ExpectedErr)
			}
		})
	}
}

func TestNewHumanizedJSONError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Description string
		Data        []byte
		Offset      int64
		Err         error
		Expected    *jsonutils.HumanizedJSONError
	}{
		{
			"nil error",
			[]byte{},
			0,
			nil,
			nil,
		},
		{
			"offset -1, before start of string",
			[]byte("line 1\nline 2\nline 3"),
			-1,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err: errors.Wrap(errors.New("message"), "invalid offset -1"),
			},
		},
		{
			"offset 0, start of string",
			[]byte("line 1\nline 2\nline 3"),
			0,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 1, character 1"),
				Line:      1,
				Character: 1,
			},
		},
		{
			"offset 5, end of line 1",
			[]byte("line 1\nline 2\nline 3"),
			5,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 1, character 6"),
				Line:      1,
				Character: 6,
			},
		},
		{
			"offset 6, new line at end end of line 1",
			[]byte("line 1\nline 2\nline 3"),
			6,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 1, character 7"),
				Line:      1,
				Character: 7,
			},
		},
		{
			"offset 7, start of line 2",
			[]byte("line 1\nline 2\nline 3"),
			7,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 2, character 1"),
				Line:      2,
				Character: 1,
			},
		},
		{
			"offset 12, end of line 2",
			[]byte("line 1\nline 2\nline 3"),
			12,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 2, character 6"),
				Line:      2,
				Character: 6,
			},
		},
		{
			"offset 13, newline at end of line 2",
			[]byte("line 1\nline 2\nline 3"),
			13,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 2, character 7"),
				Line:      2,
				Character: 7,
			},
		},
		{
			"offset 17, middle of line 3",
			[]byte("line 1\nline 2\nline 3"),
			17,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 3, character 4"),
				Line:      3,
				Character: 4,
			},
		},
		{
			"offset 19, end of string",
			[]byte("line 1\nline 2\nline 3"),
			19,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 3, character 6"),
				Line:      3,
				Character: 6,
			},
		},
		{
			"offset 20, offset = length of string",
			[]byte("line 1\nline 2\nline 3"),
			20,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 3, character 7"),
				Line:      3,
				Character: 7,
			},
		},
		{
			"offset 21, offset = length of string, after newline",
			[]byte("line 1\nline 2\nline 3\n"),
			21,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err:       errors.Wrap(errors.New("message"), "parsing error at line 4, character 1"),
				Line:      4,
				Character: 1,
			},
		},
		{
			"offset 21, offset > length of string",
			[]byte("line 1\nline 2\nline 3"),
			21,
			errors.New("message"),
			&jsonutils.HumanizedJSONError{
				Err: errors.Wrap(errors.New("message"), "invalid offset 21"),
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Description, func(t *testing.T) {
			actual := jsonutils.NewHumanizedJSONError(testCase.Err, testCase.Data, testCase.Offset)
			if testCase.Expected != nil && actual.Err != nil {
				if assert.EqualValues(t, testCase.Expected.Err.Error(), actual.Err.Error()) {
					actual.Err = testCase.Expected.Err
				}
			}
			assert.Equal(t, testCase.Expected, actual)
		})
	}
}
