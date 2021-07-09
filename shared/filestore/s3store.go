// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package filestore

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	s3 "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

// S3FileBackend contains all necessary information to communicate with
// an AWS S3 compatible API backend.
type S3FileBackend struct {
	endpoint   string
	accessKey  string
	secretKey  string
	secure     bool
	signV2     bool
	region     string
	bucket     string
	pathPrefix string
	encrypt    bool
	trace      bool
	client     *s3.Client
}

type S3FileBackendAuthError struct {
	DetailedError string
}

// S3FileBackendNoBucketError is returned when testing a connection and no S3 bucket is found
type S3FileBackendNoBucketError struct{}

const (
	// This is not exported by minio. See: https://github.com/minio/minio-go/issues/1339
	bucketNotFound = "NoSuchBucket"
)

var (
	imageExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".png": true, ".tiff": true, "tif": true}
	imageMimeTypes  = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".bmp": "image/bmp", ".png": "image/png", ".tiff": "image/tiff", ".tif": "image/tif"}
)

func isFileExtImage(ext string) bool {
	ext = strings.ToLower(ext)
	return imageExtensions[ext]
}

func getImageMimeType(ext string) string {
	ext = strings.ToLower(ext)
	if imageMimeTypes[ext] == "" {
		return "image"
	}
	return imageMimeTypes[ext]
}

func (s *S3FileBackendAuthError) Error() string {
	return s.DetailedError
}

func (s *S3FileBackendNoBucketError) Error() string {
	return "no such bucket"
}

// NewS3FileBackend returns an instance of an S3FileBackend.
func NewS3FileBackend(settings FileBackendSettings) (*S3FileBackend, error) {
	backend := &S3FileBackend{
		endpoint:   settings.AmazonS3Endpoint,
		accessKey:  settings.AmazonS3AccessKeyID,
		secretKey:  settings.AmazonS3SecretAccessKey,
		secure:     settings.AmazonS3SSL,
		signV2:     settings.AmazonS3SignV2,
		region:     settings.AmazonS3Region,
		bucket:     settings.AmazonS3Bucket,
		pathPrefix: settings.AmazonS3PathPrefix,
		encrypt:    settings.AmazonS3SSE,
		trace:      settings.AmazonS3Trace,
	}
	cli, err := backend.s3New()
	if err != nil {
		return nil, err
	}
	backend.client = cli
	return backend, nil
}

// Similar to s3.New() but allows initialization of signature v2 or signature v4 client.
// If signV2 input is false, function always returns signature v4.
//
// Additionally this function also takes a user defined region, if set
// disables automatic region lookup.
func (b *S3FileBackend) s3New() (*s3.Client, error) {
	var creds *credentials.Credentials

	isCloud := os.Getenv("MM_CLOUD_FILESTORE_BIFROST") != ""
	if isCloud {
		creds = credentials.New(customProvider{isSignV2: b.signV2})
	} else if b.accessKey == "" && b.secretKey == "" {
		creds = credentials.NewIAM("")
	} else if b.signV2 {
		creds = credentials.NewStatic(b.accessKey, b.secretKey, "", credentials.SignatureV2)
	} else {
		creds = credentials.NewStatic(b.accessKey, b.secretKey, "", credentials.SignatureV4)
	}

	opts := s3.Options{
		Creds:  creds,
		Secure: b.secure,
		Region: b.region,
	}

	// If this is a cloud installation, we override the default transport.
	if isCloud {
		tr, err := s3.DefaultTransport(b.secure)
		if err != nil {
			return nil, err
		}
		scheme := "http"
		if b.secure {
			scheme = "https"
		}
		opts.Transport = &customTransport{
			base:   tr,
			host:   b.endpoint,
			scheme: scheme,
		}
	}

	s3Clnt, err := s3.New(b.endpoint, &opts)
	if err != nil {
		return nil, err
	}

	if b.trace {
		s3Clnt.TraceOn(os.Stdout)
	}

	return s3Clnt, nil
}

func (b *S3FileBackend) TestConnection() error {
	exists := true
	var err error
	// If a path prefix is present, we attempt to test the bucket by listing objects under the path
	// and just checking the first response. This is because the BucketExists call is only at a bucket level
	// and sometimes the user might only be allowed access to the specified path prefix.
	if b.pathPrefix != "" {
		obj := <-b.client.ListObjects(context.Background(), b.bucket, s3.ListObjectsOptions{Prefix: b.pathPrefix})
		if obj.Err != nil {
			typedErr := s3.ToErrorResponse(obj.Err)
			if typedErr.Code != bucketNotFound {
				return &S3FileBackendAuthError{DetailedError: "unable to list objects in the S3 bucket"}
			}
			exists = false
		}
	} else {
		exists, err = b.client.BucketExists(context.Background(), b.bucket)
		if err != nil {
			return &S3FileBackendAuthError{DetailedError: "unable to check if the S3 bucket exists"}
		}
	}

	if !exists {
		return &S3FileBackendNoBucketError{}
	}
	mlog.Debug("Connection to S3 or minio is good. Bucket exists.")
	return nil
}

func (b *S3FileBackend) MakeBucket() error {
	err := b.client.MakeBucket(context.Background(), b.bucket, s3.MakeBucketOptions{Region: b.region})
	if err != nil {
		return errors.Wrap(err, "unable to create the s3 bucket")
	}
	return nil
}

// Caller must close the first return value
func (b *S3FileBackend) Reader(path string) (ReadCloseSeeker, error) {
	path = filepath.Join(b.pathPrefix, path)
	minioObject, err := b.client.GetObject(context.Background(), b.bucket, path, s3.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open file %s", path)
	}

	return minioObject, nil
}

func (b *S3FileBackend) ReadFile(path string) ([]byte, error) {
	path = filepath.Join(b.pathPrefix, path)
	minioObject, err := b.client.GetObject(context.Background(), b.bucket, path, s3.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open file %s", path)
	}

	defer minioObject.Close()
	f, err := ioutil.ReadAll(minioObject)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read file %s", path)
	}
	return f, nil
}

func (b *S3FileBackend) FileExists(path string) (bool, error) {
	path = filepath.Join(b.pathPrefix, path)

	_, err := b.client.StatObject(context.Background(), b.bucket, path, s3.StatObjectOptions{})
	if err == nil {
		return true, nil
	}

	var s3Err s3.ErrorResponse
	if errors.As(err, &s3Err); s3Err.Code == "NoSuchKey" {
		return false, nil
	}

	return false, errors.Wrapf(err, "unable to know if file %s exists", path)
}

func (b *S3FileBackend) FileSize(path string) (int64, error) {
	path = filepath.Join(b.pathPrefix, path)

	info, err := b.client.StatObject(context.Background(), b.bucket, path, s3.StatObjectOptions{})
	if err != nil {
		return 0, errors.Wrapf(err, "unable to get file size for %s", path)
	}

	return info.Size, nil
}

func (b *S3FileBackend) FileModTime(path string) (time.Time, error) {
	path = filepath.Join(b.pathPrefix, path)

	info, err := b.client.StatObject(context.Background(), b.bucket, path, s3.StatObjectOptions{})
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "unable to get modification time for file %s", path)
	}

	return info.LastModified, nil
}

func (b *S3FileBackend) CopyFile(oldPath, newPath string) error {
	oldPath = filepath.Join(b.pathPrefix, oldPath)
	newPath = filepath.Join(b.pathPrefix, newPath)
	srcOpts := s3.CopySrcOptions{
		Bucket:     b.bucket,
		Object:     oldPath,
		Encryption: encrypt.NewSSE(),
	}
	dstOpts := s3.CopyDestOptions{
		Bucket:     b.bucket,
		Object:     newPath,
		Encryption: encrypt.NewSSE(),
	}
	if _, err := b.client.CopyObject(context.Background(), dstOpts, srcOpts); err != nil {
		return errors.Wrapf(err, "unable to copy file from %s to %s", oldPath, newPath)
	}
	return nil
}

func (b *S3FileBackend) MoveFile(oldPath, newPath string) error {
	oldPath = filepath.Join(b.pathPrefix, oldPath)
	newPath = filepath.Join(b.pathPrefix, newPath)
	srcOpts := s3.CopySrcOptions{
		Bucket:     b.bucket,
		Object:     oldPath,
		Encryption: encrypt.NewSSE(),
	}
	dstOpts := s3.CopyDestOptions{
		Bucket:     b.bucket,
		Object:     newPath,
		Encryption: encrypt.NewSSE(),
	}

	if _, err := b.client.CopyObject(context.Background(), dstOpts, srcOpts); err != nil {
		return errors.Wrapf(err, "unable to copy the file to %s to the new destionation", newPath)
	}

	if err := b.client.RemoveObject(context.Background(), b.bucket, oldPath, s3.RemoveObjectOptions{}); err != nil {
		return errors.Wrapf(err, "unable to remove the file old file %s", oldPath)
	}

	return nil
}

func (b *S3FileBackend) WriteFile(fr io.Reader, path string) (int64, error) {
	var contentType string
	path = filepath.Join(b.pathPrefix, path)
	if ext := filepath.Ext(path); isFileExtImage(ext) {
		contentType = getImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	options := s3PutOptions(b.encrypt, contentType)
	info, err := b.client.PutObject(context.Background(), b.bucket, path, fr, -1, options)
	if err != nil {
		return info.Size, errors.Wrapf(err, "unable write the data in the file %s", path)
	}

	return info.Size, nil
}

func (b *S3FileBackend) AppendFile(fr io.Reader, path string) (int64, error) {
	fp := filepath.Join(b.pathPrefix, path)
	if _, err := b.client.StatObject(context.Background(), b.bucket, fp, s3.StatObjectOptions{}); err != nil {
		return 0, errors.Wrapf(err, "unable to find the file %s to append the data", path)
	}

	var contentType string
	if ext := filepath.Ext(fp); isFileExtImage(ext) {
		contentType = getImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	options := s3PutOptions(b.encrypt, contentType)
	sse := options.ServerSideEncryption
	partName := fp + ".part"
	info, err := b.client.PutObject(context.Background(), b.bucket, partName, fr, -1, options)
	defer b.client.RemoveObject(context.Background(), b.bucket, partName, s3.RemoveObjectOptions{})
	if info.Size > 0 {
		src1Opts := s3.CopySrcOptions{
			Bucket: b.bucket,
			Object: fp,
		}
		src2Opts := s3.CopySrcOptions{
			Bucket: b.bucket,
			Object: partName,
		}
		dstOpts := s3.CopyDestOptions{
			Bucket:     b.bucket,
			Object:     fp,
			Encryption: sse,
		}
		_, err = b.client.ComposeObject(context.Background(), dstOpts, src1Opts, src2Opts)
		if err != nil {
			return 0, errors.Wrapf(err, "unable append the data in the file %s", path)
		}
		return info.Size, nil
	}

	return 0, errors.Wrapf(err, "unable append the data in the file %s", path)
}

func (b *S3FileBackend) RemoveFile(path string) error {
	path = filepath.Join(b.pathPrefix, path)
	if err := b.client.RemoveObject(context.Background(), b.bucket, path, s3.RemoveObjectOptions{}); err != nil {
		return errors.Wrapf(err, "unable to remove the file %s", path)
	}

	return nil
}

func getPathsFromObjectInfos(in <-chan s3.ObjectInfo) <-chan s3.ObjectInfo {
	out := make(chan s3.ObjectInfo, 1)

	go func() {
		defer close(out)

		for {
			info, done := <-in

			if !done {
				break
			}

			out <- info
		}
	}()

	return out
}

func (b *S3FileBackend) ListDirectory(path string) ([]string, error) {
	path = filepath.Join(b.pathPrefix, path)
	if !strings.HasSuffix(path, "/") && path != "" {
		// s3Clnt returns only the path itself when "/" is not present
		// appending "/" to make it consistent across all filestores
		path = path + "/"
	}

	opts := s3.ListObjectsOptions{
		Prefix: path,
	}
	var paths []string
	for object := range b.client.ListObjects(context.Background(), b.bucket, opts) {
		if object.Err != nil {
			return nil, errors.Wrapf(object.Err, "unable to list the directory %s", path)
		}
		// We strip the path prefix that gets applied,
		// so that it remains transparent to the application.
		object.Key = strings.TrimPrefix(object.Key, b.pathPrefix)
		trimmed := strings.Trim(object.Key, "/")
		if trimmed != "" {
			paths = append(paths, trimmed)
		}
	}

	return paths, nil
}

func (b *S3FileBackend) RemoveDirectory(path string) error {
	opts := s3.ListObjectsOptions{
		Prefix:    filepath.Join(b.pathPrefix, path),
		Recursive: true,
	}
	list := b.client.ListObjects(context.Background(), b.bucket, opts)
	objectsCh := b.client.RemoveObjects(context.Background(), b.bucket, getPathsFromObjectInfos(list), s3.RemoveObjectsOptions{})
	for err := range objectsCh {
		if err.Err != nil {
			return errors.Wrapf(err.Err, "unable to remove the directory %s", path)
		}
	}

	return nil
}

func s3PutOptions(encrypted bool, contentType string) s3.PutObjectOptions {
	options := s3.PutObjectOptions{}
	if encrypted {
		options.ServerSideEncryption = encrypt.NewSSE()
	}
	options.ContentType = contentType
	// We set the part size to the minimum allowed value of 5MBs
	// to avoid an excessive allocation in minio.PutObject implementation.
	options.PartSize = 1024 * 1024 * 5

	return options
}
