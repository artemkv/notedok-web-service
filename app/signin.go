package app

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type tokenContainerData struct {
	IdToken string `json:"id_token" binding:"required"`
}

type sessionContainerData struct {
	Session []byte `json:"session" binding:"required"`
}

func handleSignIn(c *gin.Context) {
	// get app data from the POST body
	var tokenContainer tokenContainerData
	if err := c.ShouldBindJSON(&tokenContainer); err != nil {
		toBadRequest(c, err)
		return
	}

	// parse token
	parsedToken, err := parseAndValidateIdToken(tokenContainer.IdToken)
	if err != nil {
		log.Printf("%v", err)
		toUnauthorized(c)
		return
	}

	// sanitize
	userId := parsedToken.UserId
	if !isUserIdValid(userId) {
		log.Printf("%v", fmt.Errorf("invalid user id: '%s'", userId))
		toUnauthorized(c)
		return
	}
	userEmail := parsedToken.EMail
	if !isEmailValid(userEmail) {
		log.Printf("%v", fmt.Errorf("invalid email: '%s'", userEmail))
		toUnauthorized(c)
		return
	}

	// generate session
	session, err := generateSession(userId, userEmail)
	if err != nil {
		log.Printf("%v", err)
		toUnauthorized(c)
		return
	}

	// create response
	sessionContainer := sessionContainerData{
		Session: session,
	}
	toSuccess(c, sessionContainer)
}
