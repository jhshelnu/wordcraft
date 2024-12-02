package words

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"strings"
)

const directory = "./data"

//go:generate go run golang.org/x/tools/cmd/stringer -type ChallengeDifficulty -trimprefix Challenge
type ChallengeDifficulty int

const (
	ChallengeEasy ChallengeDifficulty = iota
	ChallengeMedium
	ChallengeHard
)

var words = make(map[string]bool, 370_104)         // the number of words in word_list.txt
var challenges = make([]string, 0, 2_256)          // the number of challenges in challenge_list.txt
var suggestions = make(map[string][]string, 2_256) // the number of challenges in challenge_list.txt

func Init() error {
	err := processFile("word_list.txt", func(word string) {
		words[word] = true
	})
	if err != nil {
		return err
	}

	err = processFile("challenge_list.txt", func(line string) {
		tokens := strings.Split(line, ",")
		challenge := tokens[0]
		challengeSuggestions := tokens[1:]

		challenges = append(challenges, challenge)
		suggestions[challenge] = challengeSuggestions
	})

	if err != nil {
		return err
	}

	return nil
}

func IsValidWord(word string) bool {
	return words[word]
}

func GetChallenge(difficulty ChallengeDifficulty) string {
	third := len(challenges) / 3
	var low, high int // each difficulty bracket sets these, and the resulting challenge is in the range [low, high)
	switch difficulty {
	case ChallengeEasy:
		// bottom third
		low = 0
		high = third
	case ChallengeMedium:
		// middle third
		low = third
		high = 2 * third
	case ChallengeHard:
		// top third
		low = 2 * third
		high = len(challenges)
	}

	return challenges[rand.IntN(high-low)+low]
}

func GetChallengeSuggestions(challenge string) []string {
	return suggestions[challenge]
}

func processFile(fileName string, lineFn func(string)) error {
	file, err := os.Open(path.Join(directory, fileName))
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
