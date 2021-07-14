// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"fmt"
	"strconv"
	"strings"
)

// This is a list of all the current versions including any patches.
// It should be maintained in chronological order with most current
// release at the front of the list.
var versions = []string{
	"5.38.0",
	"5.37.0",
	"5.36.0",
	"5.35.0",
	"5.34.0",
	"5.33.0",
	"5.32.0",
	"5.31.0",
	"5.30.0",
	"5.29.0",
	"5.28.0",
	"5.27.0",
	"5.26.0",
	"5.25.0",
	"5.24.0",
	"5.23.0",
	"5.22.0",
	"5.21.0",
	"5.20.0",
	"5.19.0",
	"5.18.0",
	"5.17.0",
	"5.16.0",
	"5.15.0",
	"5.14.0",
	"5.13.0",
	"5.12.0",
	"5.11.0",
	"5.10.0",
	"5.9.0",
	"5.8.0",
	"5.7.0",
	"5.6.0",
	"5.5.0",
	"5.4.0",
	"5.3.0",
	"5.2.0",
	"5.1.0",
	"5.0.0",
	"4.10.0",
	"4.9.0",
	"4.8.1",
	"4.8.0",
	"4.7.2",
	"4.7.1",
	"4.7.0",
	"4.6.0",
	"4.5.0",
	"4.4.0",
	"4.3.0",
	"4.2.0",
	"4.1.0",
	"4.0.0",
	"3.10.0",
	"3.9.0",
	"3.8.0",
	"3.7.0",
	"3.6.0",
	"3.5.0",
	"3.4.0",
	"3.3.0",
	"3.2.0",
	"3.1.0",
	"3.0.0",
	"2.2.0",
	"2.1.0",
	"2.0.0",
	"1.4.0",
	"1.3.0",
	"1.2.1",
	"1.2.0",
	"1.1.0",
	"1.0.0",
	"0.7.1",
	"0.7.0",
	"0.6.0",
	"0.5.0",
}

var CurrentVersion string = versions[0]
var BuildNumber string
var BuildDate string
var BuildHash string
var BuildHashEnterprise string
var BuildEnterpriseReady string
var versionsWithoutHotFixes []string

func init() {
	versionsWithoutHotFixes = make([]string, 0, len(versions))
	seen := make(map[string]string)

	for _, version := range versions {
		maj, min, _ := SplitVersion(version)
		verStr := fmt.Sprintf("%v.%v.0", maj, min)

		if seen[verStr] == "" {
			versionsWithoutHotFixes = append(versionsWithoutHotFixes, verStr)
			seen[verStr] = verStr
		}
	}
}

func SplitVersion(version string) (int64, int64, int64) {
	parts := strings.Split(version, ".")

	major := int64(0)
	minor := int64(0)
	patch := int64(0)

	if len(parts) > 0 {
		major, _ = strconv.ParseInt(parts[0], 10, 64)
	}

	if len(parts) > 1 {
		minor, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	if len(parts) > 2 {
		patch, _ = strconv.ParseInt(parts[2], 10, 64)
	}

	return major, minor, patch
}

func GetPreviousVersion(version string) string {
	verMajor, verMinor, _ := SplitVersion(version)
	verStr := fmt.Sprintf("%v.%v.0", verMajor, verMinor)

	for index, v := range versionsWithoutHotFixes {
		if v == verStr && len(versionsWithoutHotFixes) > index+1 {
			return versionsWithoutHotFixes[index+1]
		}
	}

	return ""
}

func IsCurrentVersion(versionToCheck string) bool {
	currentMajor, currentMinor, _ := SplitVersion(CurrentVersion)
	toCheckMajor, toCheckMinor, _ := SplitVersion(versionToCheck)

	if toCheckMajor == currentMajor && toCheckMinor == currentMinor {
		return true
	}
	return false
}

func IsPreviousVersionsSupported(versionToCheck string) bool {
	toCheckMajor, toCheckMinor, _ := SplitVersion(versionToCheck)
	versionToCheckStr := fmt.Sprintf("%v.%v.0", toCheckMajor, toCheckMinor)

	// Current Supported
	if versionsWithoutHotFixes[0] == versionToCheckStr {
		return true
	}

	// Current - 1 Supported
	if versionsWithoutHotFixes[1] == versionToCheckStr {
		return true
	}

	// Current - 2 Supported
	if versionsWithoutHotFixes[2] == versionToCheckStr {
		return true
	}

	// Current - 3 Supported
	if versionsWithoutHotFixes[3] == versionToCheckStr {
		return true
	}

	return false
}
