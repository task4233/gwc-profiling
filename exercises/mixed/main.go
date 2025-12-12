package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"sync"
	"time"
)

// LogEntry represents a parsed log line
type LogEntry struct {
	IP           string
	Timestamp    string
	Method       string
	Path         string
	Status       int
	ResponseTime int
}

// Stats holds aggregated statistics
type Stats struct {
	mu                sync.Mutex
	IPCounts          map[string]int
	PathCounts        map[string]int
	StatusCounts      map[int]int
	PathResponseTimes map[string][]int
	TotalLines        int
}

func NewStats() *Stats {
	return &Stats{
		IPCounts:          make(map[string]int),
		PathCounts:        make(map[string]int),
		StatusCounts:      make(map[int]int),
		PathResponseTimes: make(map[string][]int),
	}
}

func main() {
	var (
		cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
		memprofile = flag.String("memprofile", "", "write memory profile to file")
		tracefile  = flag.String("trace", "", "write trace to file")
		workers    = flag.Int("workers", 100, "number of worker goroutines")
	)
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Usage: log-analyzer [flags] <logfile>")
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *tracefile != "" {
		f, err := os.Create(*tracefile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		trace.Start(f)
		defer trace.Stop()
	}

	start := time.Now()

	stats := analyze(flag.Arg(0), *workers)
	printStats(stats)

	elapsed := time.Since(start)
	fmt.Printf("\nProcessed %d lines in %s\n", stats.TotalLines, elapsed)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		runtime.GC()
		pprof.WriteHeapProfile(f)
	}
}

// analyze processes the log file with intentional performance issues
func analyze(filename string, numWorkers int) *Stats {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	stats := NewStats()

	// Problem 1: Unbuffered channel causes blocking
	lines := make(chan string)

	// Problem 2: Unbuffered results channel
	results := make(chan *LogEntry)

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(lines, results, &wg)
	}

	// Start aggregator
	done := make(chan bool)
	go aggregator(results, stats, done)

	// Read lines
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Problem 3: String concatenation causes allocation
		line := scanner.Text() + ""
		lines <- line
		stats.TotalLines++
	}
	close(lines)

	wg.Wait()
	close(results)
	<-done

	return stats
}

// worker parses log lines with intentional inefficiencies
func worker(lines <-chan string, results chan<- *LogEntry, wg *sync.WaitGroup) {
	defer wg.Done()

	for line := range lines {
		entry := parseLine(line)
		if entry != nil {
			results <- entry
		}
	}
}

// parseLine has intentional performance issues
func parseLine(line string) *LogEntry {
	// Problem 4: Regex compiled on every call
	pattern := `^(\S+) - - \[([^\]]+)\] "(\S+) (\S+) HTTP/1.1" (\d+) \d+ "[^"]*" "[^"]*" (\d+)ms$`
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(line)
	if len(matches) != 7 {
		return nil
	}

	status, _ := strconv.Atoi(matches[5])
	responseTime, _ := strconv.Atoi(matches[6])

	return &LogEntry{
		IP:           matches[1],
		Timestamp:    matches[2],
		Method:       matches[3],
		Path:         matches[4],
		Status:       status,
		ResponseTime: responseTime,
	}
}

// aggregator collects results with mutex contention
func aggregator(results <-chan *LogEntry, stats *Stats, done chan<- bool) {
	for entry := range results {
		// Problem 5: Heavy mutex contention - locking for each field update
		stats.mu.Lock()
		stats.IPCounts[entry.IP]++
		stats.mu.Unlock()

		stats.mu.Lock()
		stats.PathCounts[entry.Path]++
		stats.mu.Unlock()

		stats.mu.Lock()
		stats.StatusCounts[entry.Status]++
		stats.mu.Unlock()

		stats.mu.Lock()
		// Problem 6: Inefficient slice append without capacity
		stats.PathResponseTimes[entry.Path] = append(stats.PathResponseTimes[entry.Path], entry.ResponseTime)
		stats.mu.Unlock()
	}
	done <- true
}

func printStats(stats *Stats) {
	fmt.Println("\n=== Top 10 IP Addresses ===")
	printTopN(stats.IPCounts, 10)

	fmt.Println("\n=== Top 10 Paths ===")
	printTopN(stats.PathCounts, 10)

	fmt.Println("\n=== Status Code Distribution ===")
	for status, count := range stats.StatusCounts {
		fmt.Printf("%d: %d\n", status, count)
	}

	fmt.Println("\n=== Average Response Time by Path ===")
	for path, times := range stats.PathResponseTimes {
		if len(times) > 0 {
			avg := average(times)
			fmt.Printf("%s: %.2fms\n", path, avg)
		}
	}
}

func printTopN(m map[string]int, n int) {
	type kv struct {
		Key   string
		Value int
	}

	var pairs []kv
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Value > pairs[j].Value
	})

	for i := 0; i < n && i < len(pairs); i++ {
		fmt.Printf("%s: %d\n", pairs[i].Key, pairs[i].Value)
	}
}

func average(nums []int) float64 {
	sum := 0
	for _, n := range nums {
		sum += n
	}
	return float64(sum) / float64(len(nums))
}
