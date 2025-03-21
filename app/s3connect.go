package app

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	log "github.com/sirupsen/logrus"
)

var (
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotFound           = errors.New("not found")
	ErrNotModified        = errors.New("not modified")
	ErrAlreadyExists      = errors.New("already exists")
)

type ListFilesResult struct {
	Files                 []*FileData
	HasMore               bool
	NextContinuationToken string
}

type FileData struct {
	FileName     string
	LastModified time.Time
	ETag         string
}

type GetFileContentResult struct {
	Content string // UTF-8 encoded content of the file
	ETag    string
}

type SaveFileContentResult struct {
	ETag string
}

type RenameFileResult struct {
	ETag string
}

func logAndReturnError(errIn error, errOut error) error {
	log.Printf("%v", errIn)
	return errOut
}

func isSupportedFileType(fileName *string) bool {
	return strings.HasSuffix(*fileName, ".txt") || strings.HasSuffix(*fileName, ".md")
}

func isMarkdown(fileName string) bool {
	return strings.HasSuffix(fileName, ".md")
}

// Retrieves the list of files by the prefix.
// Supports 2 types of files: text (.txt) and markdown (.md)
// Every record in the file list is the file name in the format "my file.md" or "my file.txt" (stripping the prefix).
//
// File names are returned as they are, no additional processing is done by this method.
// Business code is supposed to be able to properly convert the file name to the note title by its own means.
//
// Only markdown and text files are retrieved (files that have extension either ".md" or ".txt").
// The filtering is done after fetching the page from s3, so the page returned back to the client may be empty.
// To avoid this, the API should prevent users from submitting files that are neither ".md" nor ".txt".
//
// This method has no check for filtering out subfolders. The API should ensure the file name never comes with "/".
//
// The results are not in any particular order.
func listFiles(bucket string, prefix string, pageSize int, continuationToken string) (*ListFilesResult, error) {
	// Setup client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}
	s3client := s3.NewFromConfig(cfg)

	// Initialize input
	maxKeys := int32(pageSize)
	input := &s3.ListObjectsV2Input{
		Bucket:  &bucket,
		Prefix:  &prefix,
		MaxKeys: &maxKeys,
	}
	if continuationToken != "" {
		input.ContinuationToken = &continuationToken
	}

	// Fetch the files
	output, err := s3client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		// Since we control for the rest of the parameters,
		// the only one that can fail, in theory, is a continuation token
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "InvalidArgument" {
				return nil, logAndReturnError(err, ErrInvalidArgument)
			}
		}

		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	// Process the output
	files := make([]*FileData, 0, len(output.Contents))
	for _, obj := range output.Contents {
		if isSupportedFileType(obj.Key) {
			prefixStripped, _ := strings.CutPrefix(*obj.Key, prefix)

			file := &FileData{
				FileName:     prefixStripped,
				LastModified: *obj.LastModified,
				ETag:         *obj.ETag,
			}
			files = append(files, file)
		}
	}

	// Prepare the result
	result := &ListFilesResult{
		Files:                 files,
		HasMore:               *output.IsTruncated,
		NextContinuationToken: "",
	}
	if output.NextContinuationToken != nil {
		result.NextContinuationToken = *output.NextContinuationToken
	}

	return result, nil
}

// Retrieves the file content as a string.
// The file name in format "my file.md" or "my file.txt" (exactly as retrieved by listFiles).
//
// The string that is returned contains the byte array exactly as returned by S3.
//
// If etag matches, returns "not modified".
func getFileContent(bucket string, prefix string, fileName string, etag string) (*GetFileContentResult, error) {
	// Setup client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}
	s3client := s3.NewFromConfig(cfg)

	// Initialize input
	key := prefix + fileName
	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	if etag != "" {
		input.IfNoneMatch = &etag
	}

	// Fetch the content
	output, err := s3client.GetObject(context.TODO(), input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" {
				return nil, logAndReturnError(err, ErrNotFound)
			}

			if apiErr.ErrorCode() == "NotModified" {
				return nil, logAndReturnError(err, ErrNotModified)
			}
		}

		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	// Process the output
	defer output.Body.Close()
	bytes, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	// Prepare the result
	result := &GetFileContentResult{
		Content: string(bytes[:]),
		ETag:    *output.ETag,
	}

	return result, nil
}

// Saves the content into a file with the specified file name.
// The file name in format "my file.md" or "my file.txt" (exactly as retrieved by listFiles).
//
// The file name is supposed to be file system-friendly, and don't use any special characters that are not allowed by any existing file system.
// In practice that means it should not contain any of the following characters: /?<>\:*|"^%
// S3 has it's own recommendations for special characters in the object name: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html
//
// The string is passed to S3 as is, no attempt to ensure encoding is done.
// If the file with the same path already exists, parameter "overwrite" controls the method behavior.
//
// For a new note, overwrite should be set to false to avoid replacing the existing note.
// The caller should check for "already exists" error and re-submit it with the unique name.
// Uniqueness can be ensured by applying the timestamp to the file path, i.e. "my file~~1426963430173.txt"
//
// For an existing note, overwrite should be set to true.
//
// When restoring a deleted note, overwrite should be set to false, to avoid replacing the existing note.
// If the note with the same path already exists, the caller should re-submit with the unique name.
//
// Empty file name is not allowed.
// If the note title is empty, the caller is supposed to ensure the path is non-empty, by applying the timestamp to the file path, i.e. "/~~1426963430173.txt"
func saveFileContent(bucket string, prefix string, fileName string, content string, overwrite bool) (*SaveFileContentResult, error) {
	// Setup client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}
	s3client := s3.NewFromConfig(cfg)

	// Initialize input
	key := prefix + fileName
	var contentType string
	if isMarkdown(fileName) {
		contentType = "text/markdown; charset=UTF-8"
	} else {
		contentType = "text/plain"
	}
	input := &s3.PutObjectInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: &contentType,
		Body:        strings.NewReader(content),
	}
	if !overwrite {
		asterisk := "*"
		input.IfNoneMatch = &asterisk // fails if already exists
	}

	// Store the content
	output, err := s3client.PutObject(context.TODO(), input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "PreconditionFailed" {
				return nil, logAndReturnError(err, ErrAlreadyExists)
			}
		}

		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	// Prepare the result
	result := &SaveFileContentResult{
		ETag: *output.ETag,
	}

	return result, nil
}

// Renames the file by changing the corresponding file name to the new file name.
// The file name in format "my file.md" or "my file.txt" (exactly as retrieved by listFiles).
//
// The new file name is supposed to be file system-friendly, and don't use any special characters that are not allowed by any existing file system.
// In practice that means it should not contain any of the following characters: /?<>\:*|"^%
// S3 has it's own recommendations for special characters in the object name: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-keys.html
//
// The file with the file name provided is supposed to exist, ot the error will be returned.
//
// If the file with new file name already exists, the method will return error.
// The caller should check for "already exists" error and re-submit it with the unique name.
// Uniqueness can be ensured by applying the timestamp to the file path, i.e. "my file~~1426963430173.txt"
//
// If none of the files exist, it will create an empty file with the target name, which is kind of logical.
func renameFile(bucket string, prefix string, fileName string, newFileName string) (*RenameFileResult, error) {
	// Setup client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}
	s3client := s3.NewFromConfig(cfg)

	// Pre-create an empty file, to make sure we don't overwrite
	// If someone is so mega quick that they manage to overwrite this file, we will write over them.
	// In practice this will never happen.
	// If we fail after creating a dummy, then this means the dummy will stay.
	// This is easily resolvable by a user.
	_, err = saveFileContent(bucket, prefix, newFileName, "", false)
	if err != nil {
		return nil, err // already wrapped
	}

	// Initialize input
	source := bucket + "/" + prefix + url.QueryEscape(fileName)
	newKey := prefix + newFileName
	copyObjectInput := &s3.CopyObjectInput{
		Bucket:     &bucket,
		CopySource: &source,
		Key:        &newKey,
	}

	// Copy the file
	// TODO: haven't tested with large files that might take time to copy.
	// TODO: The worry is whether it will finish synchronously, for delete to be able to do its job
	output, err := s3client.CopyObject(context.TODO(), copyObjectInput)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" {
				return nil, logAndReturnError(err, ErrNotFound)
			}
		}

		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	// Prepare the result
	result := &RenameFileResult{
		ETag: *output.CopyObjectResult.ETag,
	}

	// Initialize input for deleting the old file
	key := prefix + fileName
	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	// Deleting the old file
	_, err = s3client.DeleteObject(context.TODO(), deleteObjectInput)
	if err != nil {
		return nil, logAndReturnError(err, ErrServiceUnavailable)
	}

	return result, nil
}

// Deletes the file with the specified file name.
// The file name in format "my file.md" or "my file.txt" (exactly as retrieved by listFiles).
//
// If file does not exist, does nothing and returns success.
func deleteFile(bucket string, prefix string, fileName string) error {
	// Setup client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return logAndReturnError(err, ErrServiceUnavailable)
	}
	s3client := s3.NewFromConfig(cfg)

	// Initialize input for deleting the file
	key := prefix + fileName
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	// Delete the file
	_, err = s3client.DeleteObject(context.TODO(), input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" {
				return nil
			}
		}

		return logAndReturnError(err, ErrServiceUnavailable)
	}

	return nil
}
