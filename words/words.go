package words

var words = make(map[string]bool)
var challenges = make([]string, 0, 10)

func Init() {
	// init the words
	words["cat"] = true

	// init the challenges
	challenges = append(challenges, "ca")
}

func IsValidWord(word string) bool {
	return words[word]
}

func GetChallenge() string {
	return challenges[0]
}
