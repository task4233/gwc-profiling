package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"
)

func TestMCPServerIntegration(t *testing.T) {
	// サーバプロセスを起動
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// 初期化リクエストを送信
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBytes, _ := json.Marshal(initReq)
	reqBytes = append(reqBytes, '\n')

	if _, err := stdin.Write(reqBytes); err != nil {
		t.Fatalf("Failed to write init request: %v", err)
	}

	// レスポンスを読む
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read init response: %v", err)
	}

	var initResp map[string]interface{}
	if err := json.Unmarshal(line, &initResp); err != nil {
		t.Fatalf("Failed to parse init response: %v", err)
	}

	// initialized通知を送信
	notifReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notifBytes, _ := json.Marshal(notifReq)
	notifBytes = append(notifBytes, '\n')
	stdin.Write(notifBytes)

	// tools/list リクエストを送信
	listToolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	listBytes, _ := json.Marshal(listToolsReq)
	listBytes = append(listBytes, '\n')

	if _, err := stdin.Write(listBytes); err != nil {
		t.Fatalf("Failed to write tools/list request: %v", err)
	}

	// レスポンスを読む
	line, err = reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read tools/list response: %v", err)
	}

	var listResp map[string]interface{}
	if err := json.Unmarshal(line, &listResp); err != nil {
		t.Fatalf("Failed to parse tools/list response: %v", err)
	}

	// ツールが登録されているか確認
	result, ok := listResp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("No result in tools/list response")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("No tools in result")
	}

	if len(tools) == 0 {
		t.Fatal("No tools registered")
	}

	// searchツールが存在するか確認
	found := false
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		if toolMap["name"] == "search" {
			found = true
			break
		}
	}

	if !found {
		t.Error("search tool not found")
	}

	t.Log("✅ MCP server integration test passed")
}

func TestMCPServerSearchTool(t *testing.T) {
	// サーバプロセスを起動
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// 初期化
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBytes, _ := json.Marshal(initReq)
	reqBytes = append(reqBytes, '\n')
	stdin.Write(reqBytes)
	reader.ReadBytes('\n')

	// initialized通知
	notifReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifBytes, _ := json.Marshal(notifReq)
	notifBytes = append(notifBytes, '\n')
	stdin.Write(notifBytes)

	// tools/call リクエスト
	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "search",
			"arguments": map[string]interface{}{
				"pattern":     "func",
				"paths":       []string{"testdata"},
				"max_results": 10,
			},
		},
	}

	callBytes, _ := json.Marshal(callReq)
	callBytes = append(callBytes, '\n')

	if _, err := stdin.Write(callBytes); err != nil {
		t.Fatalf("Failed to write tools/call request: %v", err)
	}

	// レスポンスを読む
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read tools/call response: %v", err)
	}

	var callResp map[string]interface{}
	if err := json.Unmarshal(line, &callResp); err != nil {
		t.Fatalf("Failed to parse tools/call response: %v", err)
	}

	// 結果を確認
	result, ok := callResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("No result in tools/call response: %+v", callResp)
	}

	// デバッグ: レスポンスの内容を表示
	t.Logf("Result: %+v", result)

	// structuredContentまたはcontentを確認
	var matchCount int
	if structuredContent, ok := result["structuredContent"].(map[string]interface{}); ok {
		if matches, ok := structuredContent["matches"].([]interface{}); ok {
			matchCount = len(matches)
		}
	} else if content, ok := result["content"].([]interface{}); ok {
		matchCount = len(content)
	}

	if matchCount == 0 {
		t.Error("No matches found")
	} else {
		t.Logf("✅ Found %d matches", matchCount)
	}
}
