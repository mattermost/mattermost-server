// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/v5/app/request"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
)

const (
	importMultiplePostsThreshold = 1000
	maxScanTokenSize             = 16 * 1024 * 1024 // Need to set a higher limit than default because some customers cross the limit. See MM-22314
)

func stopOnError(err LineImportWorkerError) bool {
	switch err.Error.ID {
	case "api.file.upload_file.large_image.app_error":
		mlog.Warn("Large image import error", mlog.Err(err.Error))
		return false
	case "app.import.validate_direct_channel_import_data.members_too_few.error", "app.import.validate_direct_channel_import_data.members_too_many.error":
		mlog.Warn("Invalid direct channel import data", mlog.Err(err.Error))
		return false
	default:
		return true
	}
}

func rewriteAttachmentPaths(files *[]AttachmentImportData, basePath string) {
	if files == nil {
		return
	}
	for _, f := range *files {
		if f.Path != nil {
			*f.Path = filepath.Join(basePath, *f.Path)
		}
	}
}

func rewriteFilePaths(line *LineImportData, basePath string) {
	switch line.Type {
	case "post", "direct_post":
		var replies []ReplyImportData
		if line.Type == "direct_post" {
			rewriteAttachmentPaths(line.DirectPost.Attachments, basePath)
			if line.DirectPost.Replies != nil {
				replies = *line.DirectPost.Replies
			}
		} else {
			rewriteAttachmentPaths(line.Post.Attachments, basePath)
			if line.Post.Replies != nil {
				replies = *line.Post.Replies
			}
		}
		for _, reply := range replies {
			rewriteAttachmentPaths(reply.Attachments, basePath)
		}
	case "user":
		if line.User.ProfileImage != nil {
			*line.User.ProfileImage = filepath.Join(basePath, *line.User.ProfileImage)
		}
	case "emoji":
		if line.Emoji.Image != nil {
			*line.Emoji.Image = filepath.Join(basePath, *line.Emoji.Image)
		}
	}
}

func (a *App) bulkImportWorker(c *request.Context, dryRun bool, wg *sync.WaitGroup, lines <-chan LineImportWorkerData, errors chan<- LineImportWorkerError) {
	postLines := []LineImportWorkerData{}
	directPostLines := []LineImportWorkerData{}
	for line := range lines {
		switch {
		case line.LineImportData.Type == "post":
			postLines = append(postLines, line)
			if line.Post == nil {
				errors <- LineImportWorkerError{model.NewAppError("BulkImport", "app.import.import_line.null_post.error", nil, "", http.StatusBadRequest), line.LineNumber}
			}
			if len(postLines) >= importMultiplePostsThreshold {
				if errLine, err := a.importMultiplePostLines(c, postLines, dryRun); err != nil {
					errors <- LineImportWorkerError{err, errLine}
				}
				postLines = []LineImportWorkerData{}
			}
		case line.LineImportData.Type == "direct_post":
			directPostLines = append(directPostLines, line)
			if line.DirectPost == nil {
				errors <- LineImportWorkerError{model.NewAppError("BulkImport", "app.import.import_line.null_direct_post.error", nil, "", http.StatusBadRequest), line.LineNumber}
			}
			if len(directPostLines) >= importMultiplePostsThreshold {
				if errLine, err := a.importMultipleDirectPostLines(c, directPostLines, dryRun); err != nil {
					errors <- LineImportWorkerError{err, errLine}
				}
				directPostLines = []LineImportWorkerData{}
			}
		default:
			if err := a.importLine(c, line.LineImportData, dryRun); err != nil {
				errors <- LineImportWorkerError{err, line.LineNumber}
			}
		}
	}

	if len(postLines) > 0 {
		if errLine, err := a.importMultiplePostLines(c, postLines, dryRun); err != nil {
			errors <- LineImportWorkerError{err, errLine}
		}
	}
	if len(directPostLines) > 0 {
		if errLine, err := a.importMultipleDirectPostLines(c, directPostLines, dryRun); err != nil {
			errors <- LineImportWorkerError{err, errLine}
		}
	}
	wg.Done()
}

func (a *App) BulkImport(c *request.Context, jsonlReader io.Reader, attachmentsReader *zip.Reader, dryRun bool, workers int) (*model.AppError, int) {
	return a.bulkImport(c, jsonlReader, attachmentsReader, dryRun, workers, "")
}

func (a *App) BulkImportWithPath(c *request.Context, jsonlReader io.Reader, attachmentsReader *zip.Reader, dryRun bool, workers int, importPath string) (*model.AppError, int) {
	return a.bulkImport(c, jsonlReader, attachmentsReader, dryRun, workers, importPath)
}

// bulkImport will extract attachments from attachmentsReader if it is
// not nil. If it is nil, it will look for attachments on the
// filesystem in the locations specified by the JSONL file according
// to the older behavior
func (a *App) bulkImport(c *request.Context, jsonlReader io.Reader, attachmentsReader *zip.Reader, dryRun bool, workers int, importPath string) (*model.AppError, int) {
	scanner := bufio.NewScanner(jsonlReader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxScanTokenSize)

	lineNumber := 0

	a.Srv().Store.LockToMaster()
	defer a.Srv().Store.UnlockFromMaster()

	errorsChan := make(chan LineImportWorkerError, (2*workers)+1) // size chosen to ensure it never gets filled up completely.
	var wg sync.WaitGroup
	var linesChan chan LineImportWorkerData
	lastLineType := ""

	var attachedFiles map[string]*zip.File
	if attachmentsReader != nil {
		attachedFiles = make(map[string]*zip.File, len(attachmentsReader.File))
		for _, fi := range attachmentsReader.File {
			attachedFiles[fi.Name] = fi
		}
	}

	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		lineNumber++

		var line LineImportData
		if err := decoder.Decode(&line); err != nil {
			return model.NewAppError("BulkImport", "app.import.bulk_import.json_decode.error", nil, err.Error(), http.StatusBadRequest), lineNumber
		}

		if len(attachedFiles) > 0 && line.Post != nil && line.Post.Attachments != nil {
			for i, attachment := range *line.Post.Attachments {
				var ok bool
				path := *attachment.Path
				if (*line.Post.Attachments)[i].Data, ok = attachedFiles[path]; !ok {
					return model.NewAppError("BulkImport", "app.import.bulk_import.json_decode.error", nil, fmt.Sprintf("attachment '%s' not found in map", path), http.StatusBadRequest), lineNumber
				}
			}
		}

		if importPath != "" {
			rewriteFilePaths(&line, importPath)
		}

		if lineNumber == 1 {
			importDataFileVersion, appErr := processImportDataFileVersionLine(line)
			if appErr != nil {
				return appErr, lineNumber
			}

			if importDataFileVersion != 1 {
				return model.NewAppError("BulkImport", "app.import.bulk_import.unsupported_version.error", nil, "", http.StatusBadRequest), lineNumber
			}
			lastLineType = line.Type
			continue
		}

		if line.Type != lastLineType {
			// Only clear the worker queue if is not the first data entry
			if lineNumber != 2 {
				// Changing type. Clear out the worker queue before continuing.
				close(linesChan)
				wg.Wait()

				// Check no errors occurred while waiting for the queue to empty.
				if len(errorsChan) != 0 {
					err := <-errorsChan
					if stopOnError(err) {
						return err.Error, err.LineNumber
					}
				}
			}

			// Set up the workers and channel for this type.
			lastLineType = line.Type
			linesChan = make(chan LineImportWorkerData, workers)
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go a.bulkImportWorker(c, dryRun, &wg, linesChan, errorsChan)
			}
		}

		select {
		case linesChan <- LineImportWorkerData{line, lineNumber}:
		case err := <-errorsChan:
			if stopOnError(err) {
				close(linesChan)
				wg.Wait()
				return err.Error, err.LineNumber
			}
		}
	}

	// No more lines. Clear out the worker queue before continuing.
	if linesChan != nil {
		close(linesChan)
	}
	wg.Wait()

	// Check no errors occurred while waiting for the queue to empty.
	if len(errorsChan) != 0 {
		err := <-errorsChan
		if stopOnError(err) {
			return err.Error, err.LineNumber
		}
	}

	if err := scanner.Err(); err != nil {
		return model.NewAppError("BulkImport", "app.import.bulk_import.file_scan.error", nil, err.Error(), http.StatusInternalServerError), 0
	}

	return nil, 0
}

func processImportDataFileVersionLine(line LineImportData) (int, *model.AppError) {
	if line.Type != "version" || line.Version == nil {
		return -1, model.NewAppError("BulkImport", "app.import.process_import_data_file_version_line.invalid_version.error", nil, "", http.StatusBadRequest)
	}

	return *line.Version, nil
}

func (a *App) importLine(c *request.Context, line LineImportData, dryRun bool) *model.AppError {
	switch {
	case line.Type == "scheme":
		if line.Scheme == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_scheme.error", nil, "", http.StatusBadRequest)
		}
		return a.importScheme(line.Scheme, dryRun)
	case line.Type == "team":
		if line.Team == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_team.error", nil, "", http.StatusBadRequest)
		}
		return a.importTeam(c, line.Team, dryRun)
	case line.Type == "channel":
		if line.Channel == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_channel.error", nil, "", http.StatusBadRequest)
		}
		return a.importChannel(c, line.Channel, dryRun)
	case line.Type == "user":
		if line.User == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_user.error", nil, "", http.StatusBadRequest)
		}
		return a.importUser(line.User, dryRun)
	case line.Type == "direct_channel":
		if line.DirectChannel == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_direct_channel.error", nil, "", http.StatusBadRequest)
		}
		return a.importDirectChannel(line.DirectChannel, dryRun)
	case line.Type == "emoji":
		if line.Emoji == nil {
			return model.NewAppError("BulkImport", "app.import.import_line.null_emoji.error", nil, "", http.StatusBadRequest)
		}
		return a.importEmoji(line.Emoji, dryRun)
	default:
		return model.NewAppError("BulkImport", "app.import.import_line.unknown_line_type.error", map[string]interface{}{"Type": line.Type}, "", http.StatusBadRequest)
	}
}

func (a *App) ListImports() ([]string, *model.AppError) {
	imports, appErr := a.ListDirectory(*a.Config().ImportSettings.Directory)
	if appErr != nil {
		return nil, appErr
	}

	results := make([]string, 0, len(imports))
	for i := 0; i < len(imports); i++ {
		filename := filepath.Base(imports[i])
		if !strings.HasSuffix(filename, IncompleteUploadSuffix) {
			results = append(results, filename)
		}
	}

	return results, nil
}
