// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package filestore

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Copied from model/config.go to avoid an import cycle
const (
	MinioAccessKey = "minioaccesskey"
	MinioSecretKey = "miniosecretkey"
	ImageDriverS3  = "amazons3"
)

func TestCheckMandatoryS3Fields(t *testing.T) {
	cfg := FileBackendSettings{}

	err := cfg.CheckMandatoryS3Fields()
	require.Error(t, err)
	require.Equal(t, err.Error(), "missing s3 bucket settings", "should've failed with missing s3 bucket")

	cfg.AmazonS3Bucket = "test-mm"
	err = cfg.CheckMandatoryS3Fields()
	require.NoError(t, err)

	cfg.AmazonS3Endpoint = ""
	err = cfg.CheckMandatoryS3Fields()
	require.NoError(t, err)

	require.Equal(t, "s3.amazonaws.com", cfg.AmazonS3Endpoint, "should've set the endpoint to the default")
}

func TestMakeBucket(t *testing.T) {
	s3Host := os.Getenv("CI_MINIO_HOST")
	if s3Host == "" {
		s3Host = "localhost"
	}

	s3Port := os.Getenv("CI_MINIO_PORT")
	if s3Port == "" {
		s3Port = "9000"
	}

	s3Endpoint := fmt.Sprintf("%s:%s", s3Host, s3Port)

	// Generate a random bucket name
	b := make([]byte, 30)
	rand.Read(b)
	bucketName := base64.StdEncoding.EncodeToString(b)
	bucketName = strings.ToLower(bucketName)
	bucketName = strings.Replace(bucketName, "+", "", -1)
	bucketName = strings.Replace(bucketName, "/", "", -1)

	cfg := FileBackendSettings{
		DriverName:              ImageDriverS3,
		AmazonS3AccessKeyID:     MinioAccessKey,
		AmazonS3SecretAccessKey: MinioSecretKey,
		AmazonS3Bucket:          bucketName,
		AmazonS3Endpoint:        s3Endpoint,
		AmazonS3Region:          "",
		AmazonS3PathPrefix:      "",
		AmazonS3SSL:             false,
	}

	fileBackend, err := NewS3FileBackend(cfg)
	require.NoError(t, err)

	err = fileBackend.MakeBucket()
	require.NoError(t, err)
}
