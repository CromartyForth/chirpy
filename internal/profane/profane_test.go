package profane

import (
	"testing"
)

func TestRemoveProfane(t *testing.T) {

	tests := []struct {
		name string
		input string
		want string
	}{
		{
			"all words", 
			"BingoBongo kerfuffle sharbert fornax shoopdydoo!", 
			"BingoBongo **** **** **** shoopdydoo!",
		 },
		 {
			"capitalised",
			"BinGoBonGo KERfuffle",
			"BinGoBonGo ****",
		 },
		 {
			"empty string",
			"",
			"",
		 },
		 {
			"exclamations are exempt",
			"kerfuffle! sharbert! fornax!",
			"kerfuffle! sharbert! fornax!",
		 },

	}

	for _, item := range tests {// Loop over each test case
        t.Run(item.name, func(t *testing.T) {// Run each case as a subtest

			got := RemoveProfane(item.input)
    		if got != item.want {
        		t.Errorf("RemoveProfane('%s') = %s; wanted %s", item.input, got, item.want)
			}
		})
	}
}

