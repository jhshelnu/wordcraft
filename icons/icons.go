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

func GetShuffledIconNames() []string {
	iconNamesShuffled := make([]string, len(iconNames))
	copy(iconNamesShuffled, iconNames)

	rand.Shuffle(len(iconNamesShuffled), func(i, j int) {
		iconNamesShuffled[i], iconNamesShuffled[j] = iconNamesShuffled[j], iconNamesShuffled[i]
	})

	return iconNamesShuffled
}
