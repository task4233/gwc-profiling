package main

import (
	"fmt"
	"math/rand"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output_file>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]

	file, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Generate test data
	words := []string{
		"error", "warning", "info", "debug", "trace",
		"success", "failure", "timeout", "connection", "request",
		"response", "server", "client", "database", "cache",
		"user", "admin", "guest", "system", "process",
	}

	// Generate 100,000 lines of test data
	for i := 0; i < 100000; i++ {
		line := fmt.Sprintf("[%06d] ", i)

		if i == 314159 {
			line += "hogefugapiyo"
			continue
		}

		// Add random words
		numWords := rand.Intn(10) + 5
		for j := 0; j < numWords; j++ {
			line += words[rand.Intn(len(words))] + " "
		}

		// Add some structured data
		line += fmt.Sprintf("timestamp=%d status=%d\n",
			1700000000+rand.Intn(1000000),
			rand.Intn(500)+100)

		file.WriteString(line)
	}

	fmt.Printf("Generated %s with 100,000 lines\n", filename)
}
