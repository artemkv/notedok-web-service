package app

import "strings"

func isUserIdValid(userId string) bool {
	return userId != ""
}

func isEmailValid(email string) bool {
	// TODO: check email format
	return email != ""
}

func isPageSizeValid(pageSize int) bool {
	return pageSize <= 1000
}

func isContinuationTokenValid(continuationToken string) bool {
	return len(continuationToken) <= 1000
}

func isFileNameValid(fileName string) bool {
	return len(fileName) <= 200 &&
		((strings.HasSuffix(fileName, ".txt") && len(fileName) > 4) ||
			(strings.HasSuffix(fileName, ".md") && len(fileName) > 3)) &&
		!strings.Contains(fileName, "/")
}

func isEtagValid(etag string) bool {
	return len(etag) <= 100
}

func isContentValid(content string) bool {
	return len(content) <= 102400
}
