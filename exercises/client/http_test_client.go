package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	const (
		serverURL   = "http://localhost:8080/search"
		concurrency = 50  // 同時接続数（大幅増加）
		requests    = 200 // 各クライアントが送るリクエスト数（大幅増加）
	)

	fmt.Printf("🚀 HTTP負荷テスト開始\n")
	fmt.Printf("   サーバ: %s\n", serverURL)
	fmt.Printf("   同時接続数: %d\n", concurrency)
	fmt.Printf("   総リクエスト数: %d\n", concurrency*requests)

	// ヘルスチェック
	if !checkHealth() {
		fmt.Println("❌ サーバが起動していません。以下のコマンドでサーバを起動してください:")
		fmt.Println("   cd exercises")
		fmt.Println("   go run http_server.go -cpuprofile=cpu.prof")
		return
	}

	start := time.Now()

	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < requests; j++ {
				if sendSearchRequest(id, serverURL) {
					mu.Lock()
					successCount++
					mu.Unlock()
				} else {
					mu.Lock()
					errorCount++
					mu.Unlock()
				}

				if (j+1)%20 == 0 {
					fmt.Printf("Client %d: %d/%d リクエスト完了\n", id, j+1+id, requests)
				}
			}
		}(i)
	}

	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("\n✅ 負荷テスト完了\n")
	fmt.Printf("   成功: %d/%d\n", successCount, concurrency*requests)
	fmt.Printf("   失敗: %d/%d\n", errorCount, concurrency*requests)
	fmt.Printf("   所要時間: %v\n", elapsed)
	fmt.Printf("   スループット: %.2f req/sec\n", float64(successCount)/elapsed.Seconds())
	fmt.Printf("\n📊 プロファイルを確認するには:\n")
	fmt.Printf("   1. サーバを Ctrl+C で停止\n")
	fmt.Printf("   2. go tool pprof -http=:8081 cpu.prof\n")
}

func checkHealth() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

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

func sendSearchRequest(clientID int, serverURL string) bool {
	// 検索リクエストを作成（より複雑な正規表現パターン）
	req := SearchRequest{
		Pattern:    "(func|type|struct|interface)\\s+\\w+.*",
		Paths:      []string{".", "..", "../testdata"}, // より広い範囲を検索
		MaxResults: 100,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		log.Printf("Client %d: JSON marshal error: %v", clientID, err)
		return false
	}

	// HTTPリクエスト送信
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Client %d: HTTP error: %v", clientID, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Client %d: HTTP status %d: %s", clientID, resp.StatusCode, string(body))
		return false
	}

	// レスポンスをパース
	var response SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Client %d: Response decode error: %v", clientID, err)
		return false
	}

	return true
}
