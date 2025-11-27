package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"sync"
	"syscall"
)

var (
	httpPort   = flag.String("port", "8080", "HTTPサーバのポート番号")
	cpuprofile = flag.String("cpuprofile", "", "CPUプロファイル出力先")
	memprofile = flag.String("memprofile", "", "メモリプロファイル出力先")
	traceFile  = flag.String("trace", "", "トレース出力先")
)

// HTTPリクエストの構造
type SearchRequest struct {
	Pattern    string   `json:"pattern"`
	Paths      []string `json:"paths"`
	MaxResults int      `json:"max_results,omitempty"`
}

// HTTPレスポンスの構造
type SearchResponse struct {
	Matches []Match `json:"matches"`
	Total   int     `json:"total"`
}

type Match struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// グローバル変数（問題5: グローバルロックでの競合）
var (
	resultsMu  sync.Mutex
	allResults []Match
)

func main() {
	flag.Parse()

	// プロファイリング設定
	setupProfiling()
	defer cleanupProfiling()

	// シグナルハンドリング設定（Ctrl+C対応）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\n🛑 シグナルを受信しました。クリーンアップ中...")
		cleanupProfiling()
		os.Exit(0)
	}()

	// HTTPハンドラの設定
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	addr := ":" + *httpPort
	fmt.Fprintf(os.Stderr, "🔍 File Search HTTP Server\n")
	fmt.Fprintf(os.Stderr, "📍 http://localhost%s で起動中...\n", addr)
	fmt.Fprintf(os.Stderr, "📌 エンドポイント:\n")
	fmt.Fprintf(os.Stderr, "   POST /search - ファイル検索\n")
	fmt.Fprintf(os.Stderr, "   GET  /health - ヘルスチェック\n")
	fmt.Fprintf(os.Stderr, "\n")

	// サーバ起動
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// デフォルト値
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}

	// 検索実行
	matches := search(req.Pattern, req.Paths, req.MaxResults)

	response := SearchResponse{
		Matches: matches,
		Total:   len(matches),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 問題1: 正規表現を毎回コンパイル（CPU問題 - pprofで顕著）
// 問題3: ゴルーチンを無制限に生成（並行性問題 - traceで顕著）
func search(pattern string, paths []string, maxResults int) []Match {
	// グローバル結果をリセット
	resultsMu.Lock()
	allResults = []Match{}
	resultsMu.Unlock()

	var wg sync.WaitGroup

	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// .goファイルのみ対象
			if info.IsDir() || filepath.Ext(filePath) != ".go" {
				return nil
			}

			// 問題3: ファイルごとにゴルーチンを生成（無制限）
			wg.Add(1)
			go func(fp string) {
				defer wg.Done()
				searchFile(fp, pattern)
			}(filePath)

			return nil
		})

		if err != nil {
			log.Printf("Walk error: %v", err)
		}
	}

	wg.Wait()

	// 結果を制限
	resultsMu.Lock()
	defer resultsMu.Unlock()

	if len(allResults) > maxResults {
		return allResults[:maxResults]
	}
	return allResults
}

// 問題1: 正規表現を毎回コンパイル（CPU問題）
// 問題2: ファイル全体をメモリに読み込む（メモリ問題）
func searchFile(filePath string, pattern string) {
	// 問題1: 毎回コンパイル
	re, err := regexp.Compile(pattern)
	if err != nil {
		return
	}

	// 問題2: ファイル全体を読み込む
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	// 問題2続き: 文字列に変換（メモリコピー）
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		if re.MatchString(line) {
			match := Match{
				File:    filePath,
				Line:    lineNum + 1,
				Content: strings.TrimSpace(line),
			}

			// 問題5: グローバルロックで競合
			resultsMu.Lock()
			allResults = append(allResults, match)
			resultsMu.Unlock()
		}
	}
}

func setupProfiling() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("CPUプロファイル作成エラー:", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("CPUプロファイル開始エラー:", err)
		}
		fmt.Fprintf(os.Stderr, "📊 CPUプロファイル: %s\n", *cpuprofile)
	}

	if *traceFile != "" {
		f, err := os.Create(*traceFile)
		if err != nil {
			log.Fatal("トレースファイル作成エラー:", err)
		}
		if err := trace.Start(f); err != nil {
			log.Fatal("トレース開始エラー:", err)
		}
		fmt.Fprintf(os.Stderr, "📊 トレース: %s\n", *traceFile)
	}
}

func cleanupProfiling() {
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
		fmt.Fprintf(os.Stderr, "✅ CPUプロファイル保存完了\n")
	}

	if *traceFile != "" {
		trace.Stop()
		fmt.Fprintf(os.Stderr, "✅ トレース保存完了\n")
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("メモリプロファイル作成エラー:", err)
		}
		defer f.Close()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("メモリプロファイル書き込みエラー:", err)
		}
		fmt.Fprintf(os.Stderr, "✅ メモリプロファイル保存完了: %s\n", *memprofile)
	}
}
