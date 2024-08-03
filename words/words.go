package words

var words = make(map[string]bool)

func Init() error {
	words["cat"] = true
	return nil
}

func IsValidWord(word string) bool {
	return words[word]
}
