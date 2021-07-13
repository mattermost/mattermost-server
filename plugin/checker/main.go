// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const pluginPackagePath = "github.com/mattermost/mattermost-server/v5/plugin"

type result struct {
	Warnings []string
	Errors   []string
}

type checkFn func(pkgPath string) (result, error)

var checks = []checkFn{
	checkAPIVersionComments,
}

func main() {
	var res result
	for _, check := range checks {
		res = runCheck(res, check)
	}

	var msgs []string
	msgs = append(msgs, res.Errors...)
	msgs = append(msgs, res.Warnings...)
	sort.Strings(msgs)

	if len(msgs) > 0 {
		fmt.Fprintln(os.Stderr, "#", pluginPackagePath)
		fmt.Fprintln(os.Stderr, strings.Join(msgs, "\n"))
	}

	if len(res.Errors) > 0 {
		os.Exit(1)
	}
}

func runCheck(prev result, fn checkFn) result {
	res, err := fn(pluginPackagePath)
	if err != nil {
		prev.Errors = append(prev.Errors, err.Error())
		return prev
	}

	if len(res.Warnings) > 0 {
		prev.Warnings = append(prev.Warnings, mapWarnings(res.Warnings)...)
	}

	if len(res.Errors) > 0 {
		prev.Errors = append(prev.Errors, res.Errors...)
	}

	return prev
}

func mapWarnings(ss []string) []string {
	var out []string
	for _, s := range ss {
		out = append(out, "[warn] "+s)
	}
	return out
}
