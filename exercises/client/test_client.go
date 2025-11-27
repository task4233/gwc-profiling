package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

func main() {
	const (
		concurrency = 10 // 同時接続数
		requests    = 50 // 各クライアントが送るリクエスト数
	)

	fmt.Printf("🚀 MCP負荷テスト開始\n")
	fmt.Printf("   同時接続数: %d\n", concurrency)
	fmt.Printf("   総リクエスト数: %d\n", concurrency*requests)

	start := time.Now()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < requests; j++ {
				if sendSearchRequest(id) {
					mu.Lock()
					successCount++
					mu.Unlock()
				}

				if (j+1)%10 == 0 {
					fmt.Printf("Client %d: %d/%d リクエスト完了\n", id, j+1, requests)
				}
			}
		}(i)
	}

	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("\n✅ 負荷テスト完了\n")
	fmt.Printf("   成功: %d/%d\n", successCount, concurrency*requests)
	fmt.Printf("   所要時間: %v\n", elapsed)
	fmt.Printf("   スループット: %.2f req/sec\n", float64(successCount)/elapsed.Seconds())
}

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type CallToolParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type SearchInput struct {
	Pattern    string   `json:"pattern"`
	Paths      []string `json:"paths"`
	MaxResults int      `json:"max_results,omitempty"`
}

type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func sendSearchRequest(clientID int) bool {
	// サーバプロセスを起動（個別のプロファイルファイルを指定）
	profileName := fmt.Sprintf("../cpu_%d_%d.prof", clientID, time.Now().UnixNano())
	cmd := exec.Command("go", "run", "../main.go", "-cpuprofile="+profileName)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Client %d: stdin error: %v", clientID, err)
		return false
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Client %d: stdout error: %v", clientID, err)
		return false
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Client %d: start error: %v", clientID, err)
		return false
	}

	// MCP初期化リクエスト
	initReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "file-search-client",
				"version": "1.0.0",
			},
		},
	}

	reqBytes, _ := json.Marshal(initReq)
	reqBytes = append(reqBytes, '\n')
	stdin.Write(reqBytes)

	// 初期化レスポンスを読んで確認
	reader := bufio.NewReader(stdout)
	initRespBytes, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("Client %d: 初期化レスポンス読み込みエラー: %v", clientID, err)
		stdin.Close()
		cmd.Wait()
		return false
	}

	var initResp MCPResponse
	if err := json.Unmarshal(initRespBytes, &initResp); err != nil {
		log.Printf("Client %d: 初期化レスポンス解析エラー: %v", clientID, err)
		stdin.Close()
		cmd.Wait()
		return false
	}

	if initResp.Error != nil {
		log.Printf("Client %d: 初期化エラー: %s", clientID, initResp.Error.Message)
		stdin.Close()
		cmd.Wait()
		return false
	}

	// tools/callリクエスト
	toolReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: CallToolParams{
			Name: "search",
			Arguments: SearchInput{
				Pattern:    "func.*",
				Paths:      []string{".."},
				MaxResults: 50,
			},
		},
	}

	toolBytes, _ := json.Marshal(toolReq)
	toolBytes = append(toolBytes, '\n')
	stdin.Write(toolBytes)

	// tools/callレスポンスを読んで確認
	toolRespBytes, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("Client %d: 検索レスポンス読み込みエラー: %v", clientID, err)
		stdin.Close()
		cmd.Wait()
		return false
	}

	var toolResp MCPResponse
	if err := json.Unmarshal(toolRespBytes, &toolResp); err != nil {
		log.Printf("Client %d: 検索レスポンス解析エラー: %v", clientID, err)
		stdin.Close()
		cmd.Wait()
		return false
	}

	if toolResp.Error != nil {
		log.Printf("Client %d: 検索エラー: %s", clientID, toolResp.Error.Message)
		stdin.Close()
		cmd.Wait()
		return false
	}

	// 検索結果を解析して表示
	var result ToolResult
	if err := json.Unmarshal(toolResp.Result, &result); err != nil {
		log.Printf("Client %d: 結果解析エラー: %v", clientID, err)
		stdin.Close()
		cmd.Wait()
		return false
	}

	// 検索結果を表示
	fmt.Printf("\n📊 Client %d - 検索結果:\n", clientID)
	for i, content := range result.Content {
		if content.Type == "text" {
			// 長いテキストは最初の200文字だけ表示
			text := content.Text
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			fmt.Printf("   [%d] %s\n", i+1, text)
		}
	}
	fmt.Printf("   合計: %d 件のコンテンツ\n\n", len(result.Content))

	stdin.Close()
	cmd.Wait()

	return true
}
