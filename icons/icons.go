package icons

import (
	"fmt"
	"math/rand/v2"
	"os"
)

const iconDirectory = "./static/icons"

var iconNames = make([]string, 0, 9) // current number of available icons

func Init() error {
	dirEntries, err := os.ReadDir(iconDirectory)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", iconDirectory, err)
	}

	for _, file := range dirEntries {
		iconNames = append(iconNames, file.Name())
	}

	return nil
}

func GetRandomIconName() string {
	return iconNames[rand.IntN(len(iconNames))]
}
