package app

import (
	"encoding/base64"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type handlerFuncWithAuth func(*gin.Context, string, string)

type sessionHeaderData struct {
	XSession string `header:"x-session"`
}

func withAuthentication(handler handlerFuncWithAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionHeader := sessionHeaderData{}
		if err := c.ShouldBindHeader(&sessionHeader); err != nil {
			log.Printf("%v", err)
			toUnauthorized(c)
			return
		}

		base64Session := sessionHeader.XSession
		if base64Session == "" {
			log.Printf("'x-session' header is empty")
			toUnauthorized(c)
			return
		}

		encryptedSession, err := base64.StdEncoding.DecodeString(base64Session)
		if err != nil {
			log.Printf("'x-session' is not base64 encoded string")
			toUnauthorized(c)
			return
		}

		session, err := parseEncryptedSession(encryptedSession)
		if err != nil {
			log.Printf("%v", err)
			toUnauthorized(c)
			return
		}

		handler(c, session.UserId, session.Email)
	}
}
