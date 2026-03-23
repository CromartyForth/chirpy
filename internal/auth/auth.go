package auth

import (
	"fmt"
	"time"
	"net/http"
	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
)

func HashPassword(password string) (string, error){
	if password == "" {
		return "", fmt.Errorf("No password supplied")
	}
	myhash, err :=argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}
	return myhash, nil
}

func CheckPasswordHash(password string, hash string) (bool, error) {
	
	isMatch, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return isMatch, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	myLittleSecret := []byte(tokenSecret)

	// claims - type Claims
	claims := jwt.RegisteredClaims{
		Issuer: "chirpy-access",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject: userID.String(),
	}


	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(myLittleSecret)
	if err != nil {
		return "", err
	}
	
	return ss, nil
}



func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	myClaims := jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, &myClaims, func (token *jwt.Token) (any, error){
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	} 
		
	id, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, err
	}
	return uuidID, nil
	
}

func GetBearerToken(headers http.Header) (string, error){
	return "", nil
}