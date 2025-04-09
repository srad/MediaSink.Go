package utils

import (
	"fmt"
	"os"
)

func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, err
	}
	if err != nil {
		// handle unexpected error
		fmt.Println("Error:", err)
		return false, err
	}
	return info.IsDir(), nil
}
