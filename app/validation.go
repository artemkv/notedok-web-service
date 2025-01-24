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
	return len(continuationToken) <= 100
}

func isFileNameValid(fileName string) bool {
	return len(fileName) <= 200 &&
		len(fileName) > 4 && // ensure it's not just ".txt"
		!strings.Contains(fileName, "/") &&
		strings.HasSuffix(fileName, ".txt")
}

func isEtagValid(etag string) bool {
	return len(etag) <= 100
}

func isContentValid(content string) bool {
	return len(content) <= 102400
}
