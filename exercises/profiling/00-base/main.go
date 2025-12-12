package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime/trace"
	"strings"
	"sync"
)

var re *regexp.Regexp

func _main() {
	pattern := os.Args[1]
	filename := os.Args[2]

	re = regexp.MustCompile(pattern)
	// Read entire file at once (inefficient for large files)
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Convert to string (creates a copy)
	content := string(data)

	// Split into lines inefficiently
	lines := strings.Split(content, "\n")

	wg := sync.WaitGroup{}

	// Process each line
	for _, line := range lines {
		wg.Add(1)
		go func(line string) {
			defer wg.Done()
			if matchLine(pattern, line) {
				fmt.Println(line)
			}
		}(line)
	}
	wg.Wait()
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <pattern> <file>\n", os.Args[0])
		os.Exit(1)
	}

	// f, err := os.Create("base.prof")
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
	// 	os.Exit(1)
	// }
	// defer f.Close()
	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	fmt.Fprintf(os.Stderr, "Error starting CPU profile: %v\n", err)
	// 	os.Exit(1)
	// }
	// defer pprof.StopCPUProfile()

	f, err := os.Create("trace.out")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := trace.Start(f); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting trace: %v\n", err)
		os.Exit(1)
	}

	defer trace.Stop()

	ctx, task := trace.NewTask(context.Background(), "main")
	defer task.End()

	trace.WithRegion(ctx, "dummyMain", func() {
		_main()
	})
}

func matchLine(pattern, line string) bool {
	region := trace.StartRegion(context.Background(), "matchLine")
	defer region.End()

	// // Compile regex every time (very inefficient!)
	// re, err := regexp.Compile(pattern)
	// if err != nil {
	// 	return false
	// }

	// Create unnecessary copies
	// lineCopy := strings.Clone(line)
	// patternCopy := strings.Clone(pattern)

	// Do some unnecessary work
	// _ = strings.ToUpper(lineCopy)
	// _ = strings.ToLower(patternCopy)

	return re.MatchString(line)
}
