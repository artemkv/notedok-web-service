package app

import (
	"encoding/json"
	"fmt"
	"time"
)

var SESSION_DURATION = time.Duration(60) * time.Minute

type sessionData struct {
	UserId  string `json:"uid" binding:"required"`
	Email   string `json:"email" binding:"required"`
	Expires string `json:"exp" binding:"required"`
}

func generateSession(userId string, userEmail string) ([]byte, error) {
	if userId == "" {
		return nil, fmt.Errorf("userId is empty")
	}
	if userEmail == "" {
		return nil, fmt.Errorf("userEmail is empty")
	}

	session := sessionData{
		UserId:  userId,
		Email:   userEmail,
		Expires: time.Now().Add(SESSION_DURATION).UTC().Format(time.RFC3339),
	}
	sessionJson, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	encrypted, err := encrypt(sessionJson)
	if err != nil {
		return nil, err
	}

	return encrypted, nil
}

func parseEncryptedSession(encryptedSession []byte) (*sessionData, error) {
	decrypted, err := decrypt(encryptedSession)
	if err != nil {
		return nil, err
	}

	var session sessionData
	err = json.Unmarshal(decrypted, &session)
	if err != nil {
		return nil, err
	}

	exp, err := time.Parse(time.RFC3339, session.Expires)
	if err != nil {
		return nil, err
	}
	if time.Now().After(exp) {
		return nil, fmt.Errorf("session has expired, expiration time: %s", session.Expires)
	}

	if session.UserId == "" {
		return nil, fmt.Errorf("userId is empty")
	}
	if session.Email == "" {
		return nil, fmt.Errorf("userEmail is empty")
	}

	return &session, nil
}
