package app

import (
	"context"
	"fmt"
	"log"

	"github.com/golang-jwt/jwt"
	"github.com/lestrrat-go/jwx/jwk"
)

// TODO: use your own config
var googleApisKeysUrl = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"
var tokenIssuer = ""
var tokenAudience = ""

var keySet jwk.Set

func init() {
	var err error
	keySet, err = jwk.Fetch(context.Background(), googleApisKeysUrl)
	if err != nil {
		log.Fatalf("Could not retrieve Google API keys")
	}
}

type parsedTokenData struct {
	UserId string
	EMail  string
}

type firebaseIdTokenClaims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

func parseAndValidateIdToken(idToken string) (*parsedTokenData, error) {
	// validates token expiration date
	token, err := jwt.ParseWithClaims(idToken, &firebaseIdTokenClaims{}, keyFunc)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*firebaseIdTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("could not retrieve standard claims")
	}

	// The audience (aud) claim should match the app client ID that was created in the Firebase
	if claims.Audience != tokenAudience {
		return nil, fmt.Errorf("wrong value of audience: %s", claims.Audience)
	}
	// The issuer (iss) claim should match your user pool
	if claims.Issuer != tokenIssuer {
		return nil, fmt.Errorf("wrong value of issuer: %s", claims.Issuer)
	}

	userId := claims.Subject
	if userId == "" {
		return nil, fmt.Errorf("user id not found in claims")
	}
	email := claims.Email
	if email == "" {
		return nil, fmt.Errorf("email id not found in claims")
	}

	parsedToken := &parsedTokenData{
		UserId: userId,
		EMail:  email,
	}
	return parsedToken, nil
}

func keyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("could not find value for the property 'kid' in header")
	}
	key, ok := keySet.LookupKeyID(kid)
	if !ok {
		return nil, fmt.Errorf("could not find key matching 'kid' '%v' in header", kid)
	}

	var rawKey interface{}
	err := key.Raw(&rawKey)
	return rawKey, err
}
