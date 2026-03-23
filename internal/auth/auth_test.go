package auth

import (
	"fmt"
	"testing"
	"time"
	"github.com/google/uuid"
	"net/http"
)

func TestPasswordHashing(t *testing.T) {

	password := "letmeinplease!"
	hash, err := HashPassword(password)
	fmt.Println(hash)
	if err != nil {
		 t.Errorf("Error hashing password")
	}

	isMatch, err := CheckPasswordHash(password, hash)
	if isMatch == false || err != nil {
		t.Errorf("Error matching password")
	}
}

func TestJWT(t *testing.T) {

	userID, _ := uuid.Parse("2b73d869-09e1-4c84-abaf-c4b57fa1194e")
	tokenSecret := "openSaysME!"
	expiresIn := time.Duration(1) * time.Second

	newToken, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Errorf("Error creating token")
	}

	_, err = ValidateJWT(newToken, tokenSecret)
	if err != nil {
		t.Errorf("Error matching token: %v", err)
	}
}

func TestGetBearerToken(t *testing.T) {

	// create faux header - type Header map[string][]string
	header := http.Header{
		"Authorization": []string{"MyBearerTolkenOfGreatLenght"},
	}
	_, err := GetBearerToken(header)
	if err != nil {
		t.Errorf("Error getting token: %v", err)
	}



}