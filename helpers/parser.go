package helpers

import (
	"fmt"
	"strconv"
	"unicode"
)

func ParseNumbers(s string) (int, error) {
	var numStr string

	// Loop through the string and collect the leading digits
	for _, char := range s {
		if unicode.IsDigit(char) {
			numStr += string(char)
		} else {
			break
		}
	}

	// Convert the extracted digits into an integer
	if numStr == "" {
		return 0, fmt.Errorf("no numbers found at the beginning of the string")
	}

	number, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	return number, nil
}
