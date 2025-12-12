package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime/pprof"
	"strings"
)

const numOfWords = 1000000

func main() {
	f, err := os.Create("cpu2.prof")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	defer pprof.StopCPUProfile()

	text := buildText(numOfWords)
	wordCount := countWords(text)

	fmt.Printf("Processed %d unique words\n", len(wordCount))
	fmt.Printf("Total text length: %d characters\n", len(text))
}

func buildText(n int) string {
	words := []string{
		"Alpha", "Bravo", "Charlie", "Delta", "Echo",
		"Foxtrot", "Golf", "Hotel", "India", "Juliett",
		"Kilo", "Lima", "Mike", "November", "Oscar",
		"Papa", "Quebec", "Romeo", "Sierra", "Tango",
		"Uniform", "Victor", "Whiskey", "X-ray", "Yankee", "Zulu",
	}
	var result string

	sb := strings.Builder{}
	sb.Grow(n * len(words))

	for i := 0; i < n; i++ {
		word := words[i%len(words)]
		sb.WriteString(word)
		sb.WriteString(" ")
	}

	return result
}

func countWords(text string) map[string]int {
	wordCount := make(map[string]int)
	words := splitWords(text)

	for _, word := range words {
		if isValidWord(word) {
			wordCount[word]++
		}
	}

	return wordCount
}

func splitWords(text string) []string {
	return strings.Fields(text)
}

func isValidWord(word string) bool {
	re := regexp.MustCompile(`^[a-zA-Z]+$`)
	return re.MatchString(word)
}
