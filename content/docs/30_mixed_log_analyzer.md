---
title: "総合演習: ログ解析ツールの最適化"
weight: 300
---

## 概要

この演習では、pprofとruntime/traceの**両方**を使用して、実践的なログ解析ツールを最適化します。CPU負荷、メモリ使用、並行処理の全ての側面で問題を抱えたプログラムを、段階的に改善していきます。

**この演習で学べること**:
- pprofとtraceの使い分け
- CPU/メモリ/並行処理の問題を総合的に診断
- 実測に基づく最適化の実践
- 改善前後のパフォーマンス比較

---

## 対象ファイル

```
exercises/mixed/
├── main.go              # 問題を含むオリジナル版
├── main_fixed.go        # 最適化版（解答例）
├── README.md            # 演習の説明
├── RESULTS.md           # 分析結果と学習ポイント
├── generator/           # テストデータ生成ツール
│   └── generate.go
└── testdata/
    └── access.log       # 解析対象のログファイル
```

---

## セットアップ

### 1. テストデータの生成

まず、解析対象のログファイルを生成します。

```bash
cd exercises/mixed/generator
go run generate.go 100000 > ../testdata/access.log
```

**オプション**: より大きなログで負荷をかける場合:

```bash
# 100万行のログ（推奨）
go run generate.go 1000000 > ../testdata/access.log
```

生成されるログの形式:

```
192.168.1.100 - - [13/Dec/2025:10:30:45 +0900] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 45ms
```

### 2. プログラムの実行確認

```bash
cd exercises/mixed
go run main.go testdata/access.log
```

**期待される出力:**

```
=== Top 10 IP Addresses ===
192.168.1.100: 1234
...

=== Top 10 Paths ===
/api/users: 5678
...

=== Status Code Distribution ===
200: 80000
404: 15000
500: 5000

=== Average Response Time by Path ===
/api/users: 45.23ms
...

Processed 100000 lines in 770.3ms
```

---

## 演習の進め方

この演習は**3つのステップ**で進めます:

1. **pprofでボトルネックを特定**（CPU/メモリ）
2. **traceで並行処理の問題を発見**
3. **改善を実施し、効果を測定**

---

## ステップ1: pprofでCPUプロファイリング

### 1.1 CPUプロファイルの取得

```bash
go run main.go -cpuprofile=cpu.prof testdata/access.log
```

### 1.2 プロファイルの分析

```bash
go tool pprof -http=:9090 cpu.prof
```

ブラウザで http://localhost:9090 にアクセスし、以下のビューを確認してください。

### 1.3 発見すべき問題

#### 問題1: 正規表現の毎回コンパイル

**Flame Graphで確認**:
- `main.parseLine` が全体の**約35%**のCPU時間を消費
- そのほとんどが `regexp.compile` に費やされている

**原因**:

```go
// main.go:160-161
func parseLine(line string) *LogEntry {
    pattern := `^(\S+) - - \[([^\]]+)\]...`
    re := regexp.MustCompile(pattern)  // ❌ 毎回コンパイル
    // ...
}
```

**問題点**:
- ログの**1行ごと**に正規表現をコンパイル
- 100,000行なら100,000回のコンパイルが発生
- 正規表現は一度コンパイルすれば再利用可能

**Top viewで確認**:

```
      flat  flat%   sum%        cum   cum%
     0.99s 31.23% 31.23%      0.99s 31.23%  regexp.compile
```

- `flat` が大きい = この関数自体が重い
- `cum ≈ flat` = 呼び出し先はない（末端関数）

#### 問題2: Mutex Contention

**Flame Graphで確認**:
- `runtime.lock2` が全体の**約28%**のCPU時間を消費
- 並行処理でのロック競合が発生

**原因**:

```go
// main.go:185-200
func aggregator(results <-chan *LogEntry, stats *Stats, done chan<- bool) {
    for entry := range results {
        // ❌ 各フィールド更新ごとに個別ロック
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
        stats.PathResponseTimes[entry.Path] = append(...)
        stats.mu.Unlock()
    }
}
```

**問題点**:
- 1エントリにつき**4回のロック/アンロック**
- 100,000エントリなら**400,000回のロック操作**
- 複数ゴルーチンが同時にロックを取得しようとして競合

**Graph viewで確認**:
- `aggregator` → `sync.(*Mutex).Lock` への太い矢印
- ロック操作が集中していることを示す

### 1.4 考察ポイント

{{% details "Question 1: なぜ正規表現の事前コンパイルが重要か？" %}}

正規表現のコンパイルは**構文解析と状態機械の構築**を伴う重い処理です。同じパターンを何度も使う場合、一度だけコンパイルしてキャッシュすることで:

- コンパイルのCPU時間を削減（1回のみ）
- メモリアロケーションを削減
- 関数の実行速度が大幅に向上

**ベストプラクティス**:
```go
// パッケージレベルで1回だけコンパイル
var logPattern = regexp.MustCompile(`^(\S+)...`)

func parseLine(line string) *LogEntry {
    // 事前コンパイルしたパターンを再利用
    matches := logPattern.FindStringSubmatch(line)
    // ...
}
```

{{% /details %}}

{{% details "Question 2: Mutex contentionを減らす方法は？" %}}

**戦略1: ロック粒度の拡大**
```go
// ❌ 悪い例: 細かいロック
stats.mu.Lock()
stats.IPCounts[entry.IP]++
stats.mu.Unlock()

// ✅ 良い例: まとめてロック
stats.mu.Lock()
stats.IPCounts[entry.IP]++
stats.PathCounts[entry.Path]++
stats.StatusCounts[entry.Status]++
stats.mu.Unlock()
```

**戦略2: ローカル集計 + 最後にマージ**（推奨）
```go
// ローカルマップで集計（ロック不要）
localIPCounts := make(map[string]int)
for entry := range results {
    localIPCounts[entry.IP]++
}

// 最後に1回だけロックしてマージ
stats.mu.Lock()
for k, v := range localIPCounts {
    stats.IPCounts[k] += v
}
stats.mu.Unlock()
```

**効果**: ロック回数が100,000回 → **1回**に削減

{{% /details %}}

---

## ステップ2: pprofでメモリプロファイリング

### 2.1 メモリプロファイルの取得

```bash
go run main.go -memprofile=mem.prof testdata/access.log
```

### 2.2 プロファイルの分析

```bash
go tool pprof -http=:9090 mem.prof
```

**SAMPLE dropdown**:
- `alloc_space`: 総アロケーション量（GC負荷の確認）
- `inuse_space`: 現在使用中のメモリ（メモリリークの確認）

### 2.3 発見すべき問題

#### 問題3: 不要な文字列アロケーション

**Top viewで確認**（`alloc_space`）:

```go
// main.go:132
line := scanner.Text() + ""  // ❌ 不要な文字列連結
```

**問題点**:
- `scanner.Text()` は既に文字列を返す
- `+ ""` による連結で**新しい文字列を毎回アロケーション**
- 100,000行なら100,000回の無駄なアロケーション

**修正**:
```go
lines <- scanner.Text()  // ✅ そのまま使用
```

#### 問題4: スライスの容量未指定

**Graph viewで確認**:
- `append` → `growslice` への矢印が太い

**原因**:

```go
// main.go:199
stats.PathResponseTimes[entry.Path] = append(stats.PathResponseTimes[entry.Path], entry.ResponseTime)
```

**問題点**:
- スライスの初期容量が0
- `append` のたびに容量が不足すると、より大きなスライスを新規アロケーションしてコピー
- 再アロケーションのコストが高い

**修正**:
```go
if _, ok := localPathResponseTimes[entry.Path]; !ok {
    localPathResponseTimes[entry.Path] = make([]int, 0, 100)  // ✅ 初期容量を指定
}
```

### 2.4 考察ポイント

{{% details "Question 3: alloc_space と inuse_space の使い分けは？" %}}

**alloc_space（累積アロケーション量）**:
- プログラム開始からの**総アロケーション量**
- GC負荷の確認に最適
- 「どこでメモリを割り当てているか」を特定

**inuse_space（使用中のメモリ）**:
- プロファイル取得時点で**保持しているメモリ**
- メモリリークの確認に最適
- 「解放されていないメモリ」を特定

**この演習では**:
- `alloc_space` を使用してGC負荷を確認
- 不要なアロケーションを削減することが目標

{{% /details %}}

---

## ステップ3: runtime/traceで並行処理を分析

### 3.1 トレースの取得

```bash
go run main.go -trace=trace.out testdata/access.log
```

### 3.2 トレースの分析

```bash
go tool trace trace.out
```

ブラウザで http://localhost:9090 にアクセスし、以下を確認してください。

### 3.3 View trace（タイムライン）

**確認ポイント**:

1. **PROCS**: プロセッサの利用状況
   - 全てのPが常に稼働しているか？
   - アイドル時間が多くないか？

2. **Goroutines**: ゴルーチンの状態
   - 緑（Running）: 実行中
   - グレー（Blocked）: ブロック中
   - 黄色（Runnable）: スケジュール待ち

3. **GC**: ガベージコレクションの頻度
   - STW（Stop-The-World）の影響は？

### 3.4 発見すべき問題

#### 問題5: 無バッファチャネルによるブロッキング

**View traceで確認**:
- ゴルーチンがグレー（Blocked）になっている箇所が多数
- チャネルの送受信でブロックしている

**原因**:

```go
// main.go:111, 114
lines := make(chan string)            // ❌ 無バッファ
results := make(chan *LogEntry)       // ❌ 無バッファ
```

**問題点**:
- 送信側がバッファがいっぱいになると受信側を待つ
- 受信側がバッファが空になると送信側を待つ
- 頻繁なブロッキングでCPU利用効率が低下

**Goroutine analysisで確認**:

```
main.worker:
  Execution time: 3.126s
  Sync block time: 1.523s  # ← ブロッキング時間が長い
```

**修正**:
```go
// バッファサイズをワーカー数の2倍に設定
lines := make(chan string, numWorkers*2)
results := make(chan *LogEntry, numWorkers*2)
```

#### 問題6: 過剰なワーカー数

**View traceで確認**:
- 各Procで頻繁なゴルーチン切り替え
- 黄色（Runnable）のゴルーチンが常に待機している

**原因**:

```go
// main.go:53
workers := flag.Int("workers", 100, "number of worker goroutines")  // ❌ 多すぎる
```

**問題点**:
- デフォルトで**100ワーカー**は多すぎる
- CPUコア数（例: 8コア）を大幅に超える
- 頻繁なコンテキストスイッチのオーバーヘッド
- スケジューラの負荷が増加

**Goroutine analysisで確認**:

```
main.worker: 100 goroutines
  Scheduler wait: 2.345s  # ← スケジュール待ち時間が長い
```

**修正**:
```go
// CPUコア数程度に設定（デフォルト4）
workers := flag.Int("workers", 4, "number of worker goroutines")
```

**最適なワーカー数の目安**:
- **I/O bound**: CPUコア数の2-4倍
- **CPU bound**: CPUコア数と同じ程度
- このケースは**CPU bound**なので、4-8程度が最適

### 3.5 実験: ワーカー数を変えて比較

```bash
# 2ワーカー
go run main.go -workers=2 -trace=trace_2workers.out testdata/access.log

# 4ワーカー
go run main.go -workers=4 -trace=trace_4workers.out testdata/access.log

# 8ワーカー
go run main.go -workers=8 -trace=trace_8workers.out testdata/access.log

# 100ワーカー（デフォルト）
go run main.go -workers=100 -trace=trace_100workers.out testdata/access.log
```

**それぞれのトレースを比較して観察**:
- ゴルーチンのブロッキング時間
- スケジューラ待ち時間
- Procの利用効率

### 3.6 考察ポイント

{{% details "Question 4: バッファ付きチャネルはどう効果があるのか？" %}}

**無バッファチャネルの動作**:
```go
ch := make(chan int)
ch <- 1  // ❌ 受信側がreadyになるまでブロック
```

**バッファ付きチャネルの動作**:
```go
ch := make(chan int, 10)
ch <- 1  // ✅ バッファに余裕があればブロックしない
ch <- 2
ch <- 3  // バッファに入るのでブロックせずに進む
```

**効果**:
1. **送信側のブロッキング削減**: バッファが満杯でなければすぐに次の処理へ
2. **受信側のブロッキング削減**: バッファが空でなければすぐに読み取れる
3. **スループット向上**: ブロッキングが減り、CPU利用効率が向上

**適切なバッファサイズ**:
- 小さすぎる: ブロッキングが頻発
- 大きすぎる: メモリ消費が増加
- **経験則**: ワーカー数の1-2倍程度

{{% /details %}}

{{% details "Question 5: ワーカー数が多すぎる/少なすぎるとどうなるか？" %}}

**多すぎる場合（100ワーカー）**:
- ✗ 頻繁なコンテキストスイッチ
- ✗ スケジューラのオーバーヘッド
- ✗ キャッシュミスの増加
- ✗ メモリ消費の増加（ゴルーチンのスタック）

**少なすぎる場合（1-2ワーカー）**:
- ✗ CPUコアが遊んでいる
- ✗ 並列性が活かせない
- ✗ スループットが低い

**適切な場合（4-8ワーカー）**:
- ✓ 全CPUコアを効率的に利用
- ✓ オーバーヘッドが少ない
- ✓ 最高のスループット

**この演習の結果**:
- 100ワーカー: 770ms
- 4ワーカー: **127ms**（6.1倍高速化）

{{% /details %}}

---

## ステップ4: 最適化の実施

### 4.1 発見した問題のまとめ

| 問題 | 発見ツール | 影響 | 対策 |
|------|-----------|------|------|
| **正規表現の毎回コンパイル** | pprof CPU | CPU時間の35% | 事前コンパイル |
| **Mutex contention** | pprof CPU | CPU時間の28% | ローカル集計 |
| **不要な文字列アロケーション** | pprof Memory | GC負荷 | 文字列連結削除 |
| **スライス再アロケーション** | pprof Memory | GC負荷 | 初期容量指定 |
| **無バッファチャネル** | trace | ブロッキング | バッファ追加 |
| **過剰なワーカー数** | trace | スケジューラ負荷 | 4ワーカーに削減 |

### 4.2 最適化版の確認

`main_fixed.go` には、全ての問題を修正した最適化版が含まれています。

**主な修正箇所**:

1. **正規表現の事前コンパイル** (main_fixed.go:50)
```go
var logPattern = regexp.MustCompile(`^(\S+)...`)
```

2. **ローカル集計** (main_fixed.go:184-214)
```go
localIPCounts := make(map[string]int)
for entry := range results {
    localIPCounts[entry.IP]++
}
stats.mu.Lock()
for k, v := range localIPCounts {
    stats.IPCounts[k] += v
}
stats.mu.Unlock()
```

3. **バッファ付きチャネル** (main_fixed.go:115-118)
```go
lines := make(chan string, numWorkers*2)
results := make(chan *LogEntry, numWorkers*2)
```

4. **適切なワーカー数** (main_fixed.go:57)
```go
workers := flag.Int("workers", 4, "number of worker goroutines")
```

5. **初期容量の指定** (main_fixed.go:42-45, 194-196)
```go
IPCounts: make(map[string]int, 100),
// ...
if _, ok := localPathResponseTimes[entry.Path]; !ok {
    localPathResponseTimes[entry.Path] = make([]int, 0, 100)
}
```

### 4.3 パフォーマンス比較

```bash
# 修正前
time go run main.go testdata/access.log

# 修正後
time go run main_fixed.go testdata/access.log
```

**結果例（100,000行）**:

| 指標 | 修正前 | 修正後 | 改善率 |
|------|--------|--------|--------|
| **実行時間** | 770.3ms | 126.7ms | **6.1倍高速化** |
| **CPU時間** | 3.69s | 0.64s | **5.8倍削減** |
| **ワーカー数** | 100 | 4 | - |

### 4.4 プロファイル比較

**修正前後のCPUプロファイル比較**:

```bash
# 修正前のプロファイル取得
go run main.go -cpuprofile=cpu_before.prof testdata/access.log

# 修正後のプロファイル取得
go run main_fixed.go -cpuprofile=cpu_after.prof testdata/access.log

# 差分比較
go tool pprof -http=:9090 -base=cpu_before.prof cpu_after.prof
```

**比較のポイント**:
- `regexp.compile` の時間が**ほぼ0**に
- `runtime.lock2` の時間が**大幅に削減**
- 全体のCPU時間が**約1/6**に削減

**修正前後のトレース比較**:

```bash
go run main.go -trace=trace_before.out testdata/access.log
go run main_fixed.go -trace=trace_after.out testdata/access.log
```

**View traceで比較**:
- ゴルーチン数が100 → 4に削減
- ブロッキング（グレー）が大幅に減少
- Procの利用効率が向上

---

## 学習ポイント

### pprofの強み

✓ **CPU/メモリのホットスポット特定**
- 関数単位でどこがボトルネックか数値で明確に
- flat/cumでコストの内訳を理解
- Flame Graphで直感的に把握

✓ **定量的な改善効果測定**
- Before/After比較で改善を実測
- 最適化の優先順位付け

### runtime/traceの強み

✓ **並行処理の可視化**
- ゴルーチンの実際の動作を時系列で確認
- ブロッキングの発生箇所を特定
- スケジューラの挙動を理解

✓ **チャネル・Mutexの問題発見**
- チャネルのブロッキングを視覚的に確認
- ワーカー数の適切さを評価

### 両方を組み合わせる価値

1. **pprofで「何が遅いか」を特定**
   - 正規表現のコンパイル: 35%
   - Mutex contention: 28%

2. **traceで「なぜ遅いか」を理解**
   - 無バッファチャネルでブロック
   - 過剰なワーカー数でスケジューラ負荷

3. **総合的な最適化戦略**
   - 両方の知見を組み合わせて6倍の高速化を達成

---

## 発展課題

### チャレンジ1: より大きなログファイルで実験

```bash
# 1000万行のログ
cd generator
go run generate.go 10000000 > ../testdata/access_large.log

# 分析
cd ..
go run main.go -cpuprofile=cpu_large.prof -trace=trace_large.out testdata/access_large.log
```

**観察ポイント**:
- スケーラビリティは維持されているか？
- 新たなボトルネックは発生していないか？

### チャレンジ2: 自分なりの最適化

`main.go` をコピーして、独自の最適化を試してみましょう:

```bash
cp main.go main_custom.go
# main_custom.go を編集
go run main_custom.go -cpuprofile=cpu_custom.prof -trace=trace_custom.out testdata/access.log
```

**試せる最適化**:
- sync.Poolで一時オブジェクトを再利用
- Worker Poolパターンの実装
- チャネルの代わりにsync.WaitGroupとスライス
- 並列読み込み（複数goroutineでファイル分割読み込み）

### チャレンジ3: Block & Mutex Profileの活用

```bash
# Block Profile
go run main.go -blockprofile=block.prof testdata/access.log
go tool pprof -http=:9090 block.prof

# Mutex Profile（コード修正が必要）
# main関数に追加:
# runtime.SetMutexProfileFraction(1)
go run main.go -mutexprofile=mutex.prof testdata/access.log
go tool pprof -http=:9090 mutex.prof
```

**確認ポイント**:
- どこでブロッキングが発生しているか？
- Mutexの競合はどの程度か？

---

## まとめ

この演習を通じて、以下のスキルを習得しました:

### 診断スキル
- ✅ pprofでCPU/メモリのボトルネックを特定
- ✅ traceで並行処理の問題を可視化
- ✅ 複数のツールを組み合わせた総合的な分析

### 最適化スキル
- ✅ 正規表現の事前コンパイル
- ✅ ローカル集計でMutex contentionを削減
- ✅ バッファ付きチャネルでブロッキングを削減
- ✅ 適切なワーカー数の選定

### 測定スキル
- ✅ Before/After比較で効果を定量化
- ✅ プロファイル差分で改善箇所を確認

**最終結果**: **6.1倍の高速化**を達成

---

## 参考資料

### 公式ドキュメント
- [runtime/pprof Package](https://pkg.go.dev/runtime/pprof)
- [net/http/pprof Package](https://pkg.go.dev/net/http/pprof)
- [runtime/trace Package](https://pkg.go.dev/runtime/trace)

### 関連ページ
- [Part 1: Profiling]({{< relref "10_profiling.md" >}})
- [Part 2: Tracing]({{< relref "15_trace.md" >}})
- [Part 3: ProfilingとTraceの比較]({{< relref "19_comparison.md" >}})
- [Part 4: 本番運用のTips]({{< relref "20_tips.md" >}})

次は[本番運用のTips]({{< relref "20_tips.md" >}})で本番環境での運用を学びます。
