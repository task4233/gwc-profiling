package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	serverURL  = "http://localhost:8080"
	maxRetries = 30
)

type SearchRequest struct {
	Pattern    string   `json:"pattern"`
	Paths      []string `json:"paths"`
	MaxResults int      `json:"max_results,omitempty"`
}

type SearchResponse struct {
	Matches []Match `json:"matches"`
	Total   int     `json:"total"`
}

type Match struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func main() {
	fmt.Println("=== セットアップ確認 ===")
	fmt.Println()

	// サーバの起動待機
	fmt.Println("⏳ サーバの起動を待機中...")
	if !waitForServer(serverURL+"/health", 30*time.Second) {
		fmt.Println("❌ サーバが起動していません")
		fmt.Println()
		fmt.Println("以下のコマンドでサーバを起動してください:")
		fmt.Println("  cd exercises")
		fmt.Println("  go run main.go")
		os.Exit(1)
	}
	fmt.Println("✅ サーバが起動しました")
	fmt.Println()

	// 1. ヘルスチェック
	fmt.Println("[1/7] ヘルスチェック")
	if checkHealth() {
		fmt.Println("  ✅ GET /health - OK")
	} else {
		fmt.Println("  ❌ GET /health - Failed")
		os.Exit(1)
	}

	// 2. 検索エンドポイント
	fmt.Println("[2/7] 検索エンドポイント")
	if checkSearch() {
		fmt.Println("  ✅ POST /search - OK")
	} else {
		fmt.Println("  ❌ POST /search - Failed")
		os.Exit(1)
	}

	// 3. pprof エンドポイント
	fmt.Println("[3/7] pprof エンドポイント")
	if checkPprof() {
		fmt.Println("  ✅ GET /debug/pprof/ - OK")
		fmt.Printf("  📊 ブラウザで確認: %s/debug/pprof/\n", serverURL)
	} else {
		fmt.Println("  ❌ GET /debug/pprof/ - Failed")
		os.Exit(1)
	}

	// 4. プロファイル取得
	fmt.Println("[4/7] プロファイルエンドポイント")
	if checkProfileEndpoints() {
		fmt.Println("  ✅ CPU/メモリプロファイル - OK")
	} else {
		fmt.Println("  ⚠️  プロファイルエンドポイントに問題がありますが、続行できます")
	}

	// 5. CPU プロファイル保存
	fmt.Println("[5/7] CPU プロファイル取得")
	if saveCPUProfile() {
		fmt.Println("  ✅ cpu.prof を保存しました")
		fmt.Println("  📊 確認コマンド: go tool pprof -http=:9090 cpu.prof")
	} else {
		fmt.Println("  ⚠️  CPU プロファイルの保存に失敗しましたが、続行できます")
	}

	// 6. メモリプロファイル保存
	fmt.Println("[6/7] メモリプロファイル取得")
	if saveMemProfile() {
		fmt.Println("  ✅ heap.prof を保存しました")
		fmt.Println("  📊 確認コマンド: go tool pprof -http=:9090 heap.prof")
	} else {
		fmt.Println("  ⚠️  メモリプロファイルの保存に失敗しましたが、続行できます")
	}

	// 7. トレース保存
	fmt.Println("[7/7] トレース取得")
	if saveTrace() {
		fmt.Println("  ✅ trace.out を保存しました")
		fmt.Println("  📊 確認コマンド: go tool trace -http=:9090 trace.out")
	} else {
		fmt.Println("  ⚠️  トレースの保存に失敗しましたが、続行できます")
	}

	fmt.Println()
	fmt.Println("=== すべてのセットアップが完了しています 🎉 ===")
	fmt.Println()
	fmt.Println("次のステップ:")
	fmt.Println("  1. ブラウザで pprof UI を確認:")
	fmt.Printf("     %s/debug/pprof/\n", serverURL)
	fmt.Println()
	fmt.Println("  2. 保存したプロファイルを確認:")
	fmt.Println("     go tool pprof -http=:9090 cpu.prof")
	fmt.Println("     go tool pprof -http=:9090 heap.prof")
	fmt.Println()
	fmt.Println("  3. 保存したトレースを確認:")
	fmt.Println("     go tool trace -http=:9090 trace.out")
	fmt.Println()
	fmt.Println("  4. 負荷テストを実行:")
	fmt.Println("     go run test_client.go")
	fmt.Println()
}

func waitForServer(healthURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func checkHealth() bool {
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func checkSearch() bool {
	req := SearchRequest{
		Pattern:    "func main",
		Paths:      []string{".."},
		MaxResults: 5,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return false
	}

	resp, err := http.Post(serverURL+"/search", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return result.Total > 0
}

func checkPprof() bool {
	resp, err := http.Get(serverURL + "/debug/pprof/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// pprofのインデックスページには "Types of profiles available:" が含まれる
	return bytes.Contains(body, []byte("profiles"))
}

func checkProfileEndpoints() bool {
	endpoints := []string{
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
	}

	allOK := true
	for _, endpoint := range endpoints {
		resp, err := http.Get(serverURL + endpoint)
		if err != nil || resp.StatusCode != http.StatusOK {
			allOK = false
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return allOK
}

// saveProfile は指定されたURLからプロファイルデータを取得し、ファイルに保存します
func saveProfile(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("プロファイル取得エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP ステータスコード: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("データ読み込みエラー: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("ファイル書き込みエラー: %w", err)
	}

	return nil
}

// generateLoad はCPUプロファイリング中にサーバに負荷をかけます
func generateLoad(duration time.Duration) {
	deadline := time.Now().Add(duration)
	requestCount := 0

	for time.Now().Before(deadline) {
		// 検索リクエストを送信
		req := SearchRequest{
			Pattern:    "func", // よくあるパターンで検索
			Paths:      []string{".."},
			MaxResults: 100,
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			continue
		}

		resp, err := http.Post(serverURL+"/search", "application/json", bytes.NewBuffer(jsonData))
		if err == nil {
			io.ReadAll(resp.Body) // レスポンスを完全に読み込む
			resp.Body.Close()
			requestCount++
		}

		// 少し待機（リクエストを詰め込みすぎない）
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Printf("  💡 負荷生成完了: %d リクエスト送信\n", requestCount)
}

// saveCPUProfile はCPUプロファイルを取得して保存します
// プロファイリング中に負荷をかけることで、有意義なデータを取得します
func saveCPUProfile() bool {
	fmt.Println("  ⏳ CPU プロファイルを取得中... (5秒)")
	fmt.Println("  💡 サーバに負荷をかけています...")

	// ゴルーチンで負荷生成を開始
	go generateLoad(5 * time.Second)

	// CPU プロファイルを取得（この呼び出しは5秒間ブロックされる）
	url := serverURL + "/debug/pprof/profile?seconds=5"
	err := saveProfile(url, "cpu.prof")
	if err != nil {
		fmt.Printf("  ⚠️  エラー: %v\n", err)
		return false
	}
	return true
}

// saveMemProfile はメモリプロファイルを取得して保存します
func saveMemProfile() bool {
	url := serverURL + "/debug/pprof/heap"
	err := saveProfile(url, "heap.prof")
	if err != nil {
		fmt.Printf("  ⚠️  エラー: %v\n", err)
		return false
	}
	return true
}

// saveTrace はトレースを取得して保存します
func saveTrace() bool {
	fmt.Println("  ⏳ トレースを取得中... (5秒)")
	url := serverURL + "/debug/pprof/trace?seconds=5"
	err := saveProfile(url, "trace.out")
	if err != nil {
		fmt.Printf("  ⚠️  エラー: %v\n", err)
		return false
	}
	return true
}
