package app

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

var _bucket string

func InitBucket(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("empty value for the bucket")
	}

	_bucket = bucket
	return nil
}

var (
	PAGE_SIZE_DEFAULT int = 100 // promote small pages to avoid loading too much into memory
)

type getFilesDataIn struct {
	PageSize          int    `form:"pageSize"` // TODO: maybe rename to MaxPageSize, since can return less
	ContinuationToken string `form:"continuationToken"`
}

type getFilesDataOut struct {
	Files                 []*FileDataOut `json:"files"`
	HasMore               bool           `json:"hasMore"`
	NextContinuationToken string         `json:"nextContinuationToken"`
}

type FileDataOut struct {
	FileName     string    `json:"fileName"`
	LastModified time.Time `json:"lastModified"`
	ETag         string    `json:"etag"`
}

type getFileDataIn struct {
	FileName string `uri:"filename" binding:"required"`
}

type putFileDataIn struct {
	FileName string `uri:"filename" binding:"required"`
}

type postFileDataIn struct {
	FileName string `uri:"filename" binding:"required"`
}

type deleteFileDataIn struct {
	FileName string `uri:"filename" binding:"required"`
}

type renameFileDataIn struct {
	FileName    string `json:"fileName" binding:"required"`
	NewFileName string `json:"newFileName" binding:"required"`
}

func handleGetFiles(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get params from query string
	var getFilesIn getFilesDataIn
	if err := c.ShouldBindQuery(&getFilesIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// sanitize
	pageSize := getFilesIn.PageSize
	if !isPageSizeValid(getFilesIn.PageSize) {
		err := fmt.Errorf("invalid pageSize '%d', should be less or equal than 1000", pageSize)
		toBadRequest(c, err)
		return
	}
	if pageSize == 0 {
		pageSize = PAGE_SIZE_DEFAULT
	}
	if !isContinuationTokenValid(getFilesIn.ContinuationToken) {
		err := fmt.Errorf("invalid continuationToken '%s', should be less than 1000 chars long", getFilesIn.ContinuationToken)
		toBadRequest(c, err)
		return
	}
	// In theory, we should use QueryUnescape, but it unescapes '+' to ' ' (space).
	// PathUnescape is identical to QueryUnescape except that it does not unescape '+' to ' ' (space).
	continuationToken, err := url.PathUnescape(getFilesIn.ContinuationToken)
	if err != nil {
		err := fmt.Errorf("invalid continuationToken '%s'", getFilesIn.ContinuationToken)
		toBadRequest(c, err)
		return
	}

	// get files
	result, err := listFiles(_bucket, prefix, pageSize, continuationToken)
	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			toBadRequest(c, err)
			return
		}

		toInternalServerError(c, err.Error())
		return
	}

	// pack result
	files := make([]*FileDataOut, 0, len(result.Files))
	for _, file := range result.Files {
		if isFileNameValid(file.FileName) {
			files = append(files, &FileDataOut{
				FileName:     file.FileName,
				LastModified: file.LastModified,
				ETag:         file.ETag,
			})
		}
	}
	getFilesDataOut := &getFilesDataOut{
		Files:   files,
		HasMore: result.HasMore,
		// Since the continuation token comes in the query param, we use QueryEscape
		NextContinuationToken: url.QueryEscape(result.NextContinuationToken),
	}

	// create response
	toSuccess(c, getFilesDataOut)
}

func handleGetFile(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get params from url
	var getFileIn getFileDataIn
	if err := c.ShouldBindUri(&getFileIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// get params from headers
	etag := ""
	ifNoneMatch := c.Request.Header["If-None-Match"]
	if len(ifNoneMatch) > 0 {
		etag = ifNoneMatch[0]
	}

	// sanitize
	if !isFileNameValid(getFileIn.FileName) {
		err := fmt.Errorf("invalid fileName '%s', check the requirements", getFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	fileName, err := url.PathUnescape(getFileIn.FileName)
	if err != nil {
		err := fmt.Errorf("invalid fileName '%s', could not decode", getFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	if !isEtagValid(etag) {
		err := fmt.Errorf("invalid etag '%s', should be less than 100 chars long", etag)
		toBadRequest(c, err)
		return
	}

	// get file content
	result, err := getFileContent(_bucket, prefix, fileName, etag)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			toNotFound(c)
			return
		}
		if errors.Is(err, ErrNotModified) {
			toNotModified(c)
			return
		}

		toInternalServerError(c, err.Error())
		return
	}

	// technically speaking, this should be "text/markdown; charset=UTF-8" for markdown files
	toPlainTextWithEtag(c, result.Content, result.ETag)
}

func handlePutFile(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get params from url
	var putFileIn putFileDataIn
	if err := c.ShouldBindUri(&putFileIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// read body
	content := readBody(c)

	// sanitize
	if !isFileNameValid(putFileIn.FileName) {
		err := fmt.Errorf("invalid fileName '%s', check the requirements", putFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	fileName, err := url.PathUnescape(putFileIn.FileName)
	if err != nil {
		err := fmt.Errorf("invalid fileName '%s', could not decode", putFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	if !isContentValid(content) {
		err := fmt.Errorf("invalid content, should be less or equal than 100KB")
		toBadRequest(c, err)
		return
	}

	// save file content
	result, err := saveFileContent(_bucket, prefix, fileName, content, true)
	if err != nil {
		toInternalServerError(c, err.Error())
		return
	}

	toNoContentWithEtag(c, result.ETag)
}

func handlePostFile(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get params from url
	var postFileIn postFileDataIn
	if err := c.ShouldBindUri(&postFileIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// read body
	content := readBody(c)

	// sanitize
	if !isFileNameValid(postFileIn.FileName) {
		err := fmt.Errorf("invalid fileName '%s', check the requirements", postFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	fileName, err := url.PathUnescape(postFileIn.FileName)
	if err != nil {
		err := fmt.Errorf("invalid fileName '%s', could not decode", postFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	if !isContentValid(content) {
		err := fmt.Errorf("invalid content, should be less or equal than 100KB")
		toBadRequest(c, err)
		return
	}

	// save file content
	result, err := saveFileContent(_bucket, prefix, fileName, content, false)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			toConflict(c, err)
			return
		}

		toInternalServerError(c, err.Error())
		return
	}

	toNoContentWithEtag(c, result.ETag)
}

func handleDeleteFile(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get params from url
	var deleteFileIn deleteFileDataIn
	if err := c.ShouldBindUri(&deleteFileIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// sanitize
	if !isFileNameValid(deleteFileIn.FileName) {
		err := fmt.Errorf("invalid fileName '%s', check the requirements", deleteFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	fileName, err := url.PathUnescape(deleteFileIn.FileName)
	if err != nil {
		err := fmt.Errorf("invalid fileName '%s', could not decode", deleteFileIn.FileName)
		toBadRequest(c, err)
		return
	}

	// get file content
	err = deleteFile(_bucket, prefix, fileName)
	if err != nil {
		toInternalServerError(c, err.Error())
		return
	}

	toNoContent(c)
}

func handleRenameFile(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	// get app data from the POST body
	var renameFileIn renameFileDataIn
	if err := c.ShouldBindJSON(&renameFileIn); err != nil {
		toBadRequest(c, err)
		return
	}

	// sanitize
	if !isFileNameValid(renameFileIn.FileName) {
		err := fmt.Errorf("invalid fileName '%s', check the requirements", renameFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	fileName, err := url.PathUnescape(renameFileIn.FileName)
	if err != nil {
		err := fmt.Errorf("invalid fileName '%s', could not decode", renameFileIn.FileName)
		toBadRequest(c, err)
		return
	}
	if !isFileNameValid(renameFileIn.NewFileName) {
		err := fmt.Errorf("invalid new fileName '%s', check the requirements", renameFileIn.NewFileName)
		toBadRequest(c, err)
		return
	}
	newFileName, err := url.PathUnescape(renameFileIn.NewFileName)
	if err != nil {
		err := fmt.Errorf("invalid new fileName '%s', could not decode", renameFileIn.NewFileName)
		toBadRequest(c, err)
		return
	}

	// rename the file
	result, err := renameFile(_bucket, prefix, fileName, newFileName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			toNotFound(c)
			return
		}
		if errors.Is(err, ErrAlreadyExists) {
			toConflict(c, err)
			return
		}

		toInternalServerError(c, err.Error())
		return
	}

	toNoContentWithEtag(c, result.ETag)
}

func handleDeleteAllFiles(c *gin.Context, userId string, email string) {
	prefix := userId + "/"

	err := deleteAllFiles(_bucket, prefix)
	if err != nil {

		toInternalServerError(c, err.Error())
		return
	}

	toNoContent(c)
}

func readBody(c *gin.Context) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(c.Request.Body)
	return buf.String()
}
