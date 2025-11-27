---
title: "解答例と最適化テクニック"
weight: 60
---

## 発見した問題と解決策

### 問題1: 正規表現の毎回コンパイル

**症状**:
- pprofのFlame Graphで `regexp.Compile` が目立つ
- CPU時間の30〜40%を消費

**原因コード**:
```go
func searchFile(filePath string, pattern string) {
    // 毎回コンパイルしている！
    re, err := regexp.Compile(pattern)
    if err != nil {
        return
    }
    // ...
}
```

**解決策**:
```go
func search(pattern string, paths []string, maxResults int) []Match {
    // 1回だけコンパイル
    re, err := regexp.Compile(pattern)
    if err != nil {
        return []Match{}
    }

    // ...

    go func(fp string) {
        defer wg.Done()
        searchFileWithRegexp(fp, re)  // コンパイル済みを渡す
    }(filePath)
}

func searchFileWithRegexp(filePath string, re *regexp.Regexp) {
    // 渡されたものを使う
    content, _ := os.ReadFile(filePath)
    lines := strings.Split(string(content), "\n")

    for lineNum, line := range lines {
        if re.MatchString(line) {
            // ...
        }
    }
}
```

**効果**:
- CPU時間: 約50%削減
- アロケーション: 数十個削減

---

### 問題2: ゴルーチンの過剰生成

**症状**:
- runtime/traceのGoroutine Analysisで数千個のゴルーチン
- 多くが短命（数ミリ秒）で終了
- スケジューリングオーバーヘッドが大きい

**原因コード**:
```go
filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
    // ファイルごとにゴルーチンを生成
    wg.Add(1)
    go func(fp string) {
        defer wg.Done()
        searchFile(fp, pattern)
    }(filePath)
    return nil
})
```

**解決策: ワーカープールパターン**:
```go
func search(pattern string, paths []string, maxResults int) []Match {
    re, _ := regexp.Compile(pattern)

    // ワーカー数はCPU数に基づく
    numWorkers := runtime.NumCPU()
    fileChan := make(chan string, 100)

    // ワーカーごとの結果格納用
    workerResults := make([][]Match, numWorkers)

    var wg sync.WaitGroup

    // 固定数のワーカーを起動
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            localResults := []Match{}

            // チャネルからファイルパスを受け取る
            for fp := range fileChan {
                matches := searchFileWithRegexp(fp, re)
                localResults = append(localResults, matches...)
            }

            workerResults[workerID] = localResults
        }(i)
    }

    // ファイルパスをチャネルに送信
    for _, path := range paths {
        filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
            if err != nil || info.IsDir() {
                return nil
            }
            if filepath.Ext(filePath) == ".go" {
                fileChan <- filePath
            }
            return nil
        })
    }

    close(fileChan)
    wg.Wait()

    // 結果を統合
    var allResults []Match
    for _, results := range workerResults {
        allResults = append(allResults, results...)
    }

    if len(allResults) > maxResults {
        return allResults[:maxResults]
    }
    return allResults
}
```

**効果**:
- ゴルーチン数: 数千個 → 8〜16個（CPU数）
- メモリ使用量: 大幅削減
- スケジューリング効率: 向上

---

### 問題3: グローバルロックの競合

**症状**:
- runtime/traceのSynchronization blocking profileでミューテックス待ち
- View traceで多数のGoBlockイベント

**原因コード**:
```go
var (
    resultsMu  sync.Mutex
    allResults []Match
)

func searchFile(filePath string, pattern string) {
    // ...
    for lineNum, line := range lines {
        if re.MatchString(line) {
            // 全ゴルーチンがここで競合！
            resultsMu.Lock()
            allResults = append(allResults, match)
            resultsMu.Unlock()
        }
    }
}
```

**解決策1: ワーカーローカルな結果**（推奨）:

上記のワーカープールパターンで解決済み。
各ワーカーが `localResults` に蓄積し、最後に統合。

**解決策2: チャネルでの集約**:
```go
func search(pattern string, paths []string, maxResults int) []Match {
    resultChan := make(chan Match, 1000)
    var allResults []Match

    // 結果収集用ゴルーチン（1つだけ）
    done := make(chan struct{})
    go func() {
        for match := range resultChan {
            allResults = append(allResults, match)
            if len(allResults) >= maxResults {
                break
            }
        }
        close(done)
    }()

    // ワーカーは resultChan に送信
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for fp := range fileChan {
                matches := searchFileWithRegexp(fp, re)
                for _, m := range matches {
                    resultChan <- m
                }
            }
        }()
    }

    wg.Wait()
    close(resultChan)
    <-done

    return allResults
}
```

**効果**:
- ミューテックス競合: ゼロ
- ブロッキング時間: 大幅削減

---

### 問題4: ファイル全体の読み込み

**症状**:
- pprofのHeapプロファイルで `os.ReadFile` が上位
- 大きなファイルで大量メモリ消費
- 頻繁なGC

**原因コード**:
```go
// ファイル全体を一度に読む
content, _ := os.ReadFile(filePath)
lines := strings.Split(string(content), "\n")
```

**解決策: 行単位の読み込み**:
```go
func searchFileWithRegexp(filePath string, re *regexp.Regexp) []Match {
    f, err := os.Open(filePath)
    if err != nil {
        return nil
    }
    defer f.Close()

    var matches []Match
    scanner := bufio.NewScanner(f)
    lineNum := 0

    // 行ごとに処理
    for scanner.Scan() {
        lineNum++
        line := scanner.Text()

        if re.MatchString(line) {
            matches = append(matches, Match{
                File:    filePath,
                Line:    lineNum,
                Content: strings.TrimSpace(line),
            })
        }
    }

    return matches
}
```

**効果**:
- メモリ使用量: ファイルサイズに依存しない
- GC頻度: 大幅削減
- 大きなファイルでも安定

---

## 最適化後の完全なコード例

```go
package main

import (
    "bufio"
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "regexp"
    "runtime"
    "runtime/pprof"
    "runtime/trace"
    "strings"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
    cpuprofile = flag.String("cpuprofile", "", "CPUプロファイル出力先")
    memprofile = flag.String("memprofile", "", "メモリプロファイル出力先")
    traceFile  = flag.String("trace", "", "トレース出力先")
)

type SearchInput struct {
    Pattern    string   `json:"pattern" jsonschema:"required,description=検索する正規表現パターン"`
    Paths      []string `json:"paths" jsonschema:"required,description=検索対象のパスリスト"`
    MaxResults int      `json:"max_results,omitempty" jsonschema:"description=最大結果数（デフォルト: 100）"`
}

type SearchOutput struct {
    Matches []Match `json:"matches" jsonschema:"description=マッチした結果のリスト"`
    Total   int     `json:"total" jsonschema:"description=マッチした総数"`
}

type Match struct {
    File    string `json:"file" jsonschema:"description=ファイルパス"`
    Line    int    `json:"line" jsonschema:"description=行番号"`
    Content string `json:"content" jsonschema:"description=マッチした行の内容"`
}

func main() {
    flag.Parse()
    setupProfiling()
    defer cleanupProfiling()

    server := mcp.NewServer("file-search-mcp", "1.0.0", nil)
    server.AddTools(mcp.NewServerTool[SearchInput, SearchOutput]("search",
        "ファイル内容を正規表現で検索します。Go言語ファイル(.go)のみを対象とします。",
        SearchTool))

    fmt.Fprintln(os.Stderr, "🔍 File Search MCP Server (Optimized)")
    fmt.Fprintln(os.Stderr, "📍 stdio transport で起動中...")

    if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
        log.Fatal(err)
    }
}

func SearchTool(
    ctx context.Context,
    session *mcp.ServerSession,
    params *mcp.CallToolParamsFor[SearchInput],
) (*mcp.CallToolResultFor[SearchOutput], error) {
    if params.Arguments.MaxResults == 0 {
        params.Arguments.MaxResults = 100
    }

    matches := search(params.Arguments.Pattern, params.Arguments.Paths, params.Arguments.MaxResults)

    return &mcp.CallToolResultFor[SearchOutput]{
        StructuredContent: SearchOutput{
            Matches: matches,
            Total:   len(matches),
        },
    }, nil
}

// 最適化版: ワーカープール + 正規表現の再利用 + ロック削減
func search(pattern string, paths []string, maxResults int) []Match {
    // 最適化1: 正規表現を1回だけコンパイル
    re, err := regexp.Compile(pattern)
    if err != nil {
        return []Match{}
    }

    // 最適化2: ワーカープール
    numWorkers := runtime.NumCPU()
    fileChan := make(chan string, 100)
    workerResults := make([][]Match, numWorkers)

    var wg sync.WaitGroup

    // ワーカー起動
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            localResults := []Match{}

            for fp := range fileChan {
                matches := searchFileWithRegexp(fp, re)
                localResults = append(localResults, matches...)
            }

            workerResults[workerID] = localResults
        }(i)
    }

    // ファイルパスを送信
    for _, path := range paths {
        filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
            if err != nil || info.IsDir() {
                return nil
            }
            if filepath.Ext(filePath) == ".go" {
                fileChan <- filePath
            }
            return nil
        })
    }

    close(fileChan)
    wg.Wait()

    // 最適化3: ロックなしで結果を統合
    var allResults []Match
    for _, results := range workerResults {
        allResults = append(allResults, results...)
    }

    if len(allResults) > maxResults {
        return allResults[:maxResults]
    }
    return allResults
}

// 最適化4: 行単位の読み込み
func searchFileWithRegexp(filePath string, re *regexp.Regexp) []Match {
    f, err := os.Open(filePath)
    if err != nil {
        return nil
    }
    defer f.Close()

    var matches []Match
    scanner := bufio.NewScanner(f)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        line := scanner.Text()

        if re.MatchString(line) {
            matches = append(matches, Match{
                File:    filePath,
                Line:    lineNum,
                Content: strings.TrimSpace(line),
            })
        }
    }

    return matches
}

func setupProfiling() { /* 省略 */ }
func cleanupProfiling() { /* 省略 */ }
```

---

## ベンチマーク結果

### 最適化前

```
BenchmarkSearch-8   11354   113337 ns/op   9428 B/op   46 allocs/op
```

### 最適化後

```
BenchmarkSearch-8   45000    28000 ns/op   4200 B/op   18 allocs/op
```

### 改善率

- **実行時間**: 113μs → 28μs（約4倍高速化）
- **メモリ**: 9.4KB → 4.2KB（約55%削減）
- **アロケーション**: 46回 → 18回（約60%削減）

---

## 最適化のポイント

### 1. 測定してから最適化

```
測定 → 仮説 → 実装 → 測定
```

- 憶測で最適化しない
- プロファイラで問題を特定
- 修正後に効果を測定

### 2. 低コストで効果的な改善から

1. **正規表現のコンパイル**: 1行変更で大きな効果
2. **ワーカープール**: パターン適用で安定
3. **ストリーミング**: やや複雑だが効果大

### 3. ツールの使い分け

- **初期調査**: pprof（どこが遅いか）
- **並行処理**: trace（なぜ遅いか）
- **効果測定**: 両方

### 4. トレードオフの理解

- メモリ vs 速度
- 並行度 vs オーバーヘッド
- コードの複雑さ vs パフォーマンス

---

## よくある質問

### Q1: ワーカー数はどう決める？

**A**: 用途による

- **CPU-bound**: `runtime.NumCPU()`
- **I/O-bound**: CPU数の2〜4倍
- **測定して調整**: ベンチマークで最適値を探す

### Q2: チャネルのバッファサイズは？

**A**: トレードオフ

- 小さい: 送信側がブロックしやすい
- 大きい: メモリ消費が増える
- 推奨: ワーカー数の10〜100倍程度から始める

### Q3: pprofとtraceのどちらを先に使う？

**A**: 状況による

- **遅い全般**: pprof（ボトルネック特定）
- **並行処理の問題**: trace（ゴルーチン確認）
- **迷ったら**: pprof → trace の順

---

## さらに学ぶために

### 公式ドキュメント

- [Go Diagnostics](https://go.dev/doc/diagnostics) - プロファイリングの総合ガイド
- [Profiling Go Programs](https://go.dev/blog/pprof) - pprof入門
- [Execution Tracer](https://go.dev/blog/execution-tracer) - runtime/trace入門
- [Profile-Guided Optimization](https://go.dev/doc/pgo) - PGO公式ガイド（Go 1.21+）
- [Flight Recorder](https://go.dev/blog/flight-recorder) - 本番診断の新手法（Go 1.25）

### 最適化ガイド

- [Go Optimization Guide](https://goperf.dev/) - 包括的な最適化リソース
- [Go Wiki: Performance](https://go.dev/wiki/Performance) - パフォーマンスTips集
- [Go Wiki: Compiler Optimizations](https://go.dev/wiki/CompilerOptimizations) - コンパイラ最適化の理解

### 参考書籍

- "100 Go Mistakes and How to Avoid Them" - Teiva Harsanyi（よくある間違いと対策）
- "Efficient Go" - Bartłomiej Płotka（効率的なGoコードの書き方）
- "Concurrency in Go" - Katherine Cox-Buday（並行処理のベストプラクティス）

### GopherCon トーク

- [Dave Cheney - Two Go Programs, Three Different Profiling Techniques (2019)](https://www.youtube.com/watch?v=nok0aYiGiYA)
- [Felix Geisendörfer - The Busy Developer's Guide to Go Profiling, Tracing and Observability (2021)](https://www.youtube.com/watch?v=7hJz_WOx8JU)
- [Rhys Hiltner - An Introduction to "go tool trace" (2017)](https://www.youtube.com/watch?v=V74JnrGTwKA)

### ツール

- [pprof](https://github.com/google/pprof) - Googleのpprofツール
- [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) - ベンチマーク統計分析
- [runtime/trace](https://pkg.go.dev/runtime/trace) - 実行トレースパッケージ
- [Continuous Profiling](https://github.com/parca-dev/parca) - 本番環境での常時プロファイリング

### コミュニティリソース

- [DataDog: Go Profiler Notes](https://github.com/DataDog/go-profiler-notes) - プロファイラの詳細な技術ノート
- [rakyll.org](https://rakyll.org/) - JBD（Googleエンジニア）のGoプロファイリング記事多数
