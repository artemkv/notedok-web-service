package app

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/golang-jwt/jwt"
	"github.com/lestrrat-go/jwx/jwk"
)

var cognitoKeysUrl = "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_oDBGh8hef/.well-known/jwks.json"
var tokenIssuer = "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_oDBGh8hef"
var tokenAudiences = []string{"171uojgfrbv775ultuqk12os85", "7e381s8r9gd2dntnuchems6epv"}

var keySet jwk.Set

func init() {
	var err error
	keySet, err = jwk.Fetch(context.Background(), cognitoKeysUrl)
	if err != nil {
		log.Fatalf("Could not retrieve Cognito keys")
	}
}

type parsedTokenData struct {
	UserId string
	EMail  string
}

type cognitoIdTokenClaims struct {
	TokenUse string `json:"token_use"`
	Email    string `json:"email"`
	jwt.StandardClaims
}

// See https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-tokens-verifying-a-jwt.html
func parseAndValidateIdToken(idToken string) (*parsedTokenData, error) {
	// validates token expiration date
	token, err := jwt.ParseWithClaims(idToken, &cognitoIdTokenClaims{}, keyFunc)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*cognitoIdTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("could not retrieve standard claims")
	}

	// The audience (aud) claim should match the app client ID that was created in the Amazon Cognito user pool
	if !slices.Contains(tokenAudiences, claims.Audience) {
		return nil, fmt.Errorf("wrong value of audience: %s", claims.Audience)
	}
	// The issuer (iss) claim should match your user pool
	if claims.Issuer != tokenIssuer {
		return nil, fmt.Errorf("wrong value of issuer: %s", claims.Issuer)
	}
	// Check the token_use claim, if you are only using the ID token, its value must be id
	if claims.TokenUse != "id" {
		return nil, fmt.Errorf("wrong value of token_use: %s", claims.TokenUse)
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
