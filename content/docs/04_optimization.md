---
title: "Part 3: 統合的な最適化"
weight: 50
---

## 発見した問題の整理

pprofとruntime/traceで発見した問題を整理します。

### pprofで発見

| 問題 | 場所 | 影響 | ツール |
|------|------|------|--------|
| 正規表現の毎回コンパイル | `searchFile` | CPU | pprof |
| ファイル全体の読み込み | `searchFile` | メモリ | pprof |
| 文字列のコピー | `strings.Split` | メモリ | pprof |

### runtime/traceで発見

| 問題 | 場所 | 影響 | ツール |
|------|------|------|--------|
| ゴルーチンの過剰生成 | `search` | スケジューリング | trace |
| グローバルロックの競合 | `searchFile` | 並行性 | trace |
| 頻繁なGC | 全体 | レイテンシ | trace |

---

## 演習1: 問題の優先順位付け（5分）

### どの問題から修正すべきか？

グループで議論してください：

**判断基準**:
1. **影響の大きさ**: パフォーマンスへのインパクト
2. **修正の容易さ**: 実装コストと副作用
3. **効果の確実性**: 測定可能な改善

{{% details "推奨する優先順位" %}}

**優先度1: 正規表現の事前コンパイル**
- 理由: 実装が簡単で効果が確実
- 影響: CPUの大幅削減
- リスク: ほぼなし

**優先度2: ワーカープールの導入**
- 理由: ゴルーチン数を制限できる
- 影響: メモリとスケジューリング改善
- リスク: 並行度の調整が必要

**優先度3: ロックの粒度改善**
- 理由: 競合を減らせる
- 影響: 並行性の向上
- リスク: 実装ミスでデータ競合の可能性

**優先度4: ストリーミング読み込み**
- 理由: メモリ使用量を削減
- 影響: GC圧力の軽減
- リスク: 実装が複雑

{{% /details %}}

---

## 演習2: 正規表現の最適化（10分）

### 2-1. 問題の確認

現在のコード (`main.go:150`付近):

```go
func searchFile(filePath string, pattern string) {
    // 問題: 毎回コンパイル！
    re, err := regexp.Compile(pattern)
    if err != nil {
        return
    }
    // ...
}
```

### 2-2. 修正方法

**アプローチ**: 正規表現を一度だけコンパイルして再利用

```go
func search(pattern string, paths []string, maxResults int) []Match {
    // 修正: 最初に1回だけコンパイル
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
    // コンパイル済みのものを使う
    content, _ := os.ReadFile(filePath)
    // ...
}
```

### 2-3. 効果測定

修正後、ベンチマークを実行：

```bash
# 修正前
go test -bench=BenchmarkSearch -benchmem
# BenchmarkSearch-8   11354   113337 ns/op   9428 B/op   46 allocs/op

# 修正後に期待される改善
# BenchmarkSearch-8   20000    60000 ns/op   9428 B/op   40 allocs/op
# → 約2倍高速化、アロケーション削減
```

---

## 演習3: ワーカープールの導入（15分）

### 3-1. 問題の確認

現在のコード:

```go
// 問題: ファイル数分のゴルーチン生成
filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
    wg.Add(1)
    go func(fp string) {
        defer wg.Done()
        searchFile(fp, pattern)
    }(filePath)
    return nil
})
```

### 3-2. 修正方法

**アプローチ**: 固定数のワーカーでファイルを処理

```go
func search(pattern string, paths []string, maxResults int) []Match {
    re, _ := regexp.Compile(pattern)

    // ワーカープール
    numWorkers := runtime.NumCPU()
    fileChan := make(chan string, 100)

    var wg sync.WaitGroup

    // ワーカー起動
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for fp := range fileChan {
                searchFileWithRegexp(fp, re)
            }
        }()
    }

    // ファイルを送信
    filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
        if /* .goファイル */ {
            fileChan <- filePath  // チャネルに送信
        }
        return nil
    })

    close(fileChan)
    wg.Wait()

    return allResults
}
```

### 3-3. 効果測定

```bash
go test -bench=BenchmarkSearch -benchmem
```

**期待される改善**:
- ゴルーチン数: 数千個 → CPU数（8〜16個）
- メモリ使用量: 削減
- スケジューリング効率: 向上

---

## 演習4: ロックの粒度改善（10分）

### 4-1. 問題の確認

現在のコード:

```go
// 問題: グローバルロックで全員が待つ
resultsMu.Lock()
allResults = append(allResults, match)
resultsMu.Unlock()
```

### 4-2. 修正方法

**アプローチ1**: ワーカーごとにローカルスライスを使う

```go
func search(pattern string, paths []string, maxResults int) []Match {
    // ワーカーごとの結果
    workerResults := make([][]Match, numWorkers)

    // ワーカー起動
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            localResults := []Match{}  // ローカル

            for fp := range fileChan {
                matches := searchFileWithRegexp(fp, re)
                localResults = append(localResults, matches...)
            }

            workerResults[workerID] = localResults  // ロック不要
        }(i)
    }

    // 最後に結合
    var allResults []Match
    for _, results := range workerResults {
        allResults = append(allResults, results...)
    }

    return allResults
}
```

**アプローチ2**: チャネルで結果を集約

```go
resultChan := make(chan Match, 1000)

go func() {
    for match := range resultChan {
        allResults = append(allResults, match)  // 1つのゴルーチンだけ
    }
}()
```

### 4-3. 効果確認

runtime/traceで確認:
- Synchronization blocking profileでミューテックス待ちが減少
- View traceでGoBlockイベントが減少

---

## 演習5: benchstatによる統計的な効果測定（10分）

### 5-1. ベンチマークのノイズ問題

単一のベンチマーク実行では、結果が不安定です：

```bash
# 1回目
BenchmarkSearch-8   11354   113337 ns/op

# 2回目（同じコード）
BenchmarkSearch-8   11892   110123 ns/op  ← 3%速い？

# 3回目（同じコード）
BenchmarkSearch-8   10678   118234 ns/op  ← 4%遅い？
```

**問題**: どれが「本当の性能」？改善したと言えるのは何%から？

### 5-2. benchstatのインストールと使用

```bash
# インストール
go install golang.org/x/perf/cmd/benchstat@latest

# 最適化前: 10回実行
go test -bench=BenchmarkSearch -count=10 -benchmem > old.txt

# コードを修正...

# 最適化後: 10回実行
go test -bench=BenchmarkSearch -count=10 -benchmem > new.txt

# 統計的比較
benchstat old.txt new.txt
```

**出力例**:

```
name              old time/op    new time/op    delta
Search-8            113μs ± 2%      28μs ± 1%  -75.22%  (p=0.000 n=10+10)

name              old alloc/op   new alloc/op   delta
Search-8           9.42kB ± 0%    4.20kB ± 0%  -55.41%  (p=0.000 n=10+10)

name              old allocs/op  new allocs/op  delta
Search-8             46.0 ± 0%      18.0 ± 0%  -60.87%  (p=0.000 n=10+10)
```

**読み方**:
- **±2%**: 中央値からの95%信頼区間（変動幅）
- **-75.22%**: 改善率（マイナスは高速化）
- **p=0.000**: p値（< 0.05なら統計的に有意）
- **n=10+10**: 各10回のサンプル数

### 5-3. 統計的有意性の判断

#### ✅ 有意な改善

```
Search-8    113μs ± 2%     28μs ± 1%  -75.22%  (p=0.000 n=10+10)
                                                ↑ p < 0.05 → 本当に速くなった！
```

#### ⚠️ 有意でない（ノイズの範囲）

```
Search-8    113μs ± 5%    110μs ± 4%   -2.65%  (p=0.234 n=10+10)
                                                ↑ p > 0.05 → 誤差の範囲

または

Search-8    113μs ± 3%    113μs ± 2%     ~     (p=0.684 n=10+10)
                                        ↑ チルダ = 差なし
```

→ 最適化は効果がなかった（または測定誤差に埋もれている）

### 5-4. ベストプラクティス

1. **最低10回、理想は20回実行**
   ```bash
   go test -bench=. -count=20
   ```

2. **環境を固定**
   ```bash
   # CPU周波数を固定（Linux）
   sudo cpupower frequency-set --governor performance

   # ベンチマーク実行
   go test -bench=. -count=20

   # 終わったら元に戻す
   sudo cpupower frequency-set --governor powersave
   ```

3. **複数テストの罠に注意**
   - p < 0.05になるまで再実行を繰り返さない（偽陽性の原因）
   - 一度だけ実行して結果を受け入れる

詳細は [高度なプロファイリングテクニック: benchstat](../06_advanced_techniques#benchstat-による統計的ベンチマーク比較) を参照してください。

---

## 演習6: トレースでの効果確認（5分）

### 6-1. 最適化前後のトレース比較

**トレース比較**:

```bash
# 最適化前
go run main.go -trace=trace_before.out
# (負荷テスト実行)

# 最適化後
go run main.go -trace=trace_after.out
# (負荷テスト実行)

# 比較
go tool trace trace_before.out  # ゴルーチン: 数千個
go tool trace trace_after.out   # ゴルーチン: 10個程度
```

### 5-2. 負荷テストでの比較

```bash
cd client
go run test_client.go
```

**測定項目**:
- スループット (req/sec)
- レイテンシ (所要時間)
- メモリ使用量

---

## pprofとtraceの使い分け（まとめ）

### 視覚的な違い

```
pprof (CPU Profile)              runtime/trace
───────────────────              ──────────────
関数単位の集計                    時系列での実行状態
      ↓                                ↓

main.searchFile  84.37%           Time →
  ├─ os.ReadFile 39.07%          ┌─────────────────┐
  ├─ regexp.*    23.36%          │ G1 ████──█████  │ ← ゴルーチン1
  └─ mutex.*     17.00%          │ G2 ──████────██ │ ← ゴルーチン2
                                  │ G3 ████████──── │ ← ゴルーチン3
「どこで」時間を使っているか       │ GC │    │      │ ← GC発生
                                  └─────────────────┘
                                 「いつ」「なぜ」遅いか
```

### 問題発見フェーズ

| 症状 | 使うツール | 理由 |
|------|-----------|------|
| 遅い | pprof (CPU) | ボトルネックの関数を特定 |
| メモリを食う | pprof (Heap) | 割り当ての多い箇所を特定 |
| 並行処理が遅い | trace | ゴルーチン数、ブロッキング |
| レイテンシが不安定 | trace | GCの影響、待ち時間 |

### 最適化検証フェーズ

| 最適化内容 | 測定ツール | 確認項目 |
|----------|----------|---------|
| アルゴリズム改善 | pprof | CPU時間の削減 |
| メモリ削減 | pprof + trace | 割り当て削減、GC頻度 |
| 並行度調整 | trace | ゴルーチン数、P利用率 |
| ロック改善 | trace | ブロッキング時間 |

### 両方使うべき場面

- GC問題: pprofでメモリ割り当てを見つけ、traceでGC頻度を確認
- 並行処理: pprofで関数のコストを見て、traceで並行度を確認
- 総合最適化: pprofで優先度付け、traceで効果測定

---

## 気づきの共有（10分）

### グループディスカッション

1. **最も効果的だった最適化は何でしたか？**
2. **pprofとtraceをどう使い分けましたか？**
3. **実際のプロジェクトで応用できそうですか？**

### 全体共有

各グループの気づきを共有してください。

---

## 持ち帰りのポイント

1. **pprofとtraceは補完関係**
   - pprof: 「どこで」リソースを使っているか
   - trace: 「いつ」「なぜ」問題が起きているか

2. **測定→仮説→検証のサイクル**
   - 先に測定して問題を特定
   - 仮説を立てて修正
   - 再測定で効果を確認

3. **問題の種類でツールを選ぶ**
   - CPU/メモリ → pprof
   - 並行処理/GC → trace
   - 迷ったら両方

---

## 参考資料

- [Go公式ブログ: Profiling Go Programs](https://go.dev/blog/pprof)
- [Go公式ブログ: Execution Tracer](https://go.dev/blog/execution-tracer)
- [解答例とベンチマーク](../05_solutions)

---

## 参考資料

### 公式ドキュメント

- [Diagnostics - Go公式](https://go.dev/doc/diagnostics)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [Execution Tracer](https://go.dev/blog/execution-tracer)

### 最適化手法

- [Go Optimization Guide](https://goperf.dev/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Wiki: Performance](https://go.dev/wiki/Performance)
- [Go Wiki: Compiler Optimizations](https://go.dev/wiki/CompilerOptimizations)

### ベンチマーキング

- [benchstat documentation](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
- [Benchmarking in Go: A Comprehensive Handbook](https://betterstack.com/community/guides/scaling-go/golang-benchmarking/)
- [Leveraging benchstat Projections](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/)

### GC チューニング

- [GOMEMLIMIT is a game changer](https://weaviate.io/blog/gomemlimit-a-game-changer-for-high-memory-applications)
- [Go Optimization Guide: Memory Efficiency and GC](https://goperf.dev/01-common-patterns/gc/)
- [A Guide to the Go Garbage Collector](https://go.dev/doc/gc-guide)

### 書籍

- "100 Go Mistakes and How to Avoid Them" - Teiva Harsanyi
- "Efficient Go" - Bartłomiej Płotka

---

お疲れさまでした！

次のステップとして、[高度なプロファイリングテクニック](../06_advanced_techniques) で、PGO、Flight Recorder、Escape Analysisなどのさらに高度な技術を学ぶことができます。
