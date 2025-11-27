package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"sync"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "CPUプロファイル出力先")
	memprofile = flag.String("memprofile", "", "メモリプロファイル出力先")
	traceFile  = flag.String("trace", "", "トレース出力先")
)

// ツールの入力定義
type SearchInput struct {
	Pattern    string   `json:"pattern" jsonschema:"required,description=検索する正規表現パターン"`
	Paths      []string `json:"paths" jsonschema:"required,description=検索対象のパスリスト"`
	MaxResults int      `json:"max_results,omitempty" jsonschema:"description=最大結果数（デフォルト: 100）"`
}

// ツールの出力定義
type SearchOutput struct {
	Matches []Match `json:"matches" jsonschema:"description=マッチした結果のリスト"`
	Total   int     `json:"total" jsonschema:"description=マッチした総数"`
}

type Match struct {
	File    string `json:"file" jsonschema:"description=ファイルパス"`
	Line    int    `json:"line" jsonschema:"description=行番号"`
	Content string `json:"content" jsonschema:"description=マッチした行の内容"`
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

	// MCPサーバの作成
	server := mcp.NewServer("file-search-mcp", "1.0.0", nil)

	// ツールの追加
	server.AddTools(mcp.NewServerTool[SearchInput, SearchOutput]("search",
		"ファイル内容を正規表現で検索します。Go言語ファイル(.go)のみを対象とします。",
		SearchTool))

	fmt.Fprintln(os.Stderr, "🔍 File Search MCP Server")
	fmt.Fprintln(os.Stderr, "📍 stdio transport で起動中...")
	fmt.Fprintln(os.Stderr, "")

	// stdioトランスポートで実行
	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		log.Fatal(err)
	}
}

// SearchTool - ファイル検索ツールの実装
func SearchTool(
	ctx context.Context,
	session *mcp.ServerSession,
	params *mcp.CallToolParamsFor[SearchInput],
) (*mcp.CallToolResultFor[SearchOutput], error) {
	// デフォルト値
	if params.Arguments.MaxResults == 0 {
		params.Arguments.MaxResults = 100
	}

	// 検索実行
	matches := search(params.Arguments.Pattern, params.Arguments.Paths, params.Arguments.MaxResults)

	output := SearchOutput{
		Matches: matches,
		Total:   len(matches),
	}

	// JSONにシリアライズ
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("JSON serialization error: %w", err)
	}

	result := &mcp.CallToolResultFor[SearchOutput]{
		StructuredContent: output,
	}

	// テキストコンテンツを追加
	result.Content = []mcp.Content{
		&mcp.TextContent{
			Text: string(jsonBytes),
		},
	}

	return result, nil
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
