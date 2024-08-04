package words

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
)

var words = make(map[string]bool, 370_104) // the number of words in word_list.txt
var challenges = make([]string, 0)         // todo: allocate the appropriate capacity here once the challenges are determined

func Init() error {
	err := processFile("word_list.txt", func(word string) {
		words[word] = true
	})
	if err != nil {
		return err
	}

	err = processFile("challenge_list.txt", func(challenge string) {
		challenges = append(challenges, challenge)
	})
	if err != nil {
		return err
	}

	return nil
}

func IsValidWord(word string) bool {
	return words[word]
}

func GetChallenge() string {
	return challenges[rand.IntN(len(challenges))]
}

func processFile(fileName string, lineFn func(string)) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to process file %s: %w", fileName, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineFn(scanner.Text())
	}

	return nil
}
