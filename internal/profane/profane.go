package profane

import (
	"strings"
)


func RemoveProfane(chirpText string) string {
	
	badWords := [3]string {"kerfuffle", "sharbert", "fornax"}

	stringArray := strings.Fields(chirpText)

	// replace bad words
	for n, word := range(stringArray){
		for _, badword := range(badWords){
			if strings.ToLower(word) == badword {
				stringArray[n] = "****"
			}		
		}
	}

	// join array into string and return
	cleanedString := strings.Join(stringArray, " ")
	
	return cleanedString
}

// func Join(elems []string, sep string) string
// func Fields(s string) []string
// BingoBongo kerfuffle sharbert fornax shoopdydoo!