---
title: "高度なプロファイリングテクニック"
weight: 70
---

このセクションでは、Go 1.21以降で導入された最新のプロファイリング技術と、実践的な最適化手法を紹介します。

---

## Flight Recorder（Go 1.25の新機能）

### 概要

Flight Recorderは、Go 1.25で正式に追加された機能で、**常時トレース収集を有効にしつつ、メモリ上の循環バッファに最新の数秒分のみを保持**します。

### 従来のトレースとの違い

| 項目 | 従来のトレース | Flight Recorder |
|------|--------------|-----------------|
| データ保存 | ファイル or ソケット | メモリ上の循環バッファ |
| オーバーヘッド | 10-20% CPU（Go 1.20以前） | 1-2% CPU（Go 1.21+） |
| 用途 | 計画的なプロファイリング | 問題発生時の事後分析 |
| データ量 | 全実行期間 | 直近数秒のみ |

### 使用例

```go
package main

import (
    "fmt"
    "os"
    "runtime/trace"
    "time"
)

func main() {
    // Flight Recorderを開始
    if err := trace.Start(os.Stderr); err != nil {
        panic(err)
    }
    defer trace.Stop()

    // アプリケーションコード
    for i := 0; i < 1000; i++ {
        processRequest(i)

        // 問題検出時にトレースをダンプ
        if detectProblem(i) {
            dumpTrace(i)
        }
    }
}

func detectProblem(i int) bool {
    // レイテンシスパイクなどの異常を検出
    // 例: リクエスト処理時間が閾値を超えた
    return i%100 == 99
}

func dumpTrace(requestID int) {
    filename := fmt.Sprintf("trace_problem_%d.out", requestID)
    f, err := os.Create(filename)
    if err != nil {
        return
    }
    defer f.Close()

    // 直近のトレースデータを書き出し
    trace.FlightRecorder(f)
    fmt.Fprintf(os.Stderr, "⚠️  トレースを保存しました: %s\n", filename)
}

func processRequest(id int) {
    // 何らかの処理
    time.Sleep(time.Millisecond)
}
```

### ユースケース

1. **本番環境での診断**
   - オーバーヘッドが低い（1-2%）ため、本番で常時有効化可能
   - 問題発生時に直近の実行状態を確認できる

2. **レイテンシスパイクの調査**
   - p99レイテンシが閾値を超えた瞬間のトレースを保存
   - 再現困難な一時的な問題の分析

3. **デバッグが困難な並行処理バグ**
   - デッドロックやレースコンディションの事後分析
   - ゴルーチンのスケジューリング問題の可視化

### 注意事項

- バッファサイズは環境変数 `GODEBUG=traceflight=SIZE` で調整可能
- デフォルトは数秒分（通常1-10MB程度）
- 本番環境では必ず問題検出時のみダンプするよう制御する

### 参考資料

- [Flight Recorder in Go 1.25](https://go.dev/blog/flight-recorder)
- [More powerful Go execution traces](https://go.dev/blog/execution-traces-2024)
- [runtime/trace パッケージ](https://pkg.go.dev/runtime/trace)

---

## Profile-Guided Optimization (PGO)

### 概要

PGOは、**本番環境のプロファイルデータをコンパイラにフィードバックし、最適化を改善する手法**です。Go 1.21で正式導入され、Go 1.22では平均**2-14%の性能向上**が報告されています。

### 仕組み

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│ 本番環境で  │ --> │ プロファイル │ --> │ 最適化した  │
│ 実行        │     │ 収集         │     │ バイナリ    │
└─────────────┘     └──────────────┘     └─────────────┘
       ↑                                         │
       └─────────────────────────────────────────┘
              再デプロイして性能向上
```

1. 本番環境またはベンチマークからCPUプロファイルを収集
2. プロファイルを `default.pgo` としてメインパッケージに配置
3. 再ビルド時、コンパイラが自動的にPGOを適用

### 使用手順

#### Step 1: プロファイルの収集

```go
package main

import (
    "net/http"
    _ "net/http/pprof"  // 自動的に /debug/pprof エンドポイントを追加
)

func main() {
    // pprofエンドポイントを別ポートで公開（推奨）
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()

    // メインのアプリケーション
    startServer()
}
```

```bash
# 本番環境または代表的なワークロードから30秒間のCPUプロファイルを収集
curl -o cpu.pprof http://localhost:6060/debug/pprof/profile?seconds=30

# または、負荷テスト中に収集
go test -cpuprofile=cpu.pprof -bench=.
```

#### Step 2: プロファイルの配置

```bash
# default.pgo としてメインパッケージのディレクトリに配置
mv cpu.pprof ./default.pgo

# または、複数のプロファイルをマージ
go tool pprof -proto cpu1.pprof cpu2.pprof cpu3.pprof > default.pgo
```

#### Step 3: PGOを使用してビルド

```bash
# 自動的にdefault.pgoを検出して使用
go build

# または明示的に指定
go build -pgo=default.pgo

# PGOを無効化する場合
go build -pgo=off
```

### 最適化の種類

PGOは主に2つの最適化を実施します：

#### 1. インライン展開の改善

**従来のコンパイラ**:
- 関数サイズや複雑度で機械的にインライン化を判断
- 実行頻度は考慮されない

**PGO有効時**:
- プロファイルで頻繁に呼ばれる関数を優先的にインライン化
- コールグラフ全体を考慮した最適化

```go
// 頻繁に呼ばれる関数は、やや大きくてもインライン化される
func hotPath(x int) int {
    return compute(x) + validate(x) + transform(x)
}
```

#### 2. デバーチャライゼーション

**従来のコンパイラ**:
- インターフェース呼び出しは常に間接呼び出し

**PGO有効時**:
- プロファイルから具体的な型が判明した場合、直接呼び出しに変換

```go
type Processor interface {
    Process(data []byte) error
}

// PGOにより、実行時に常にFastProcessorが使われることが判明
// → インターフェース呼び出しを直接呼び出しに最適化
func handle(p Processor, data []byte) {
    p.Process(data)  // FastProcessor.Process に直接呼び出し
}
```

### 効果測定例

```bash
# PGOなしでビルド
go build -pgo=off -o server_nopgo

# PGOありでビルド
go build -pgo=default.pgo -o server_pgo

# ベンチマーク比較
go test -bench=. -count=10 > nopgo.txt
# バイナリを差し替え
go test -bench=. -count=10 > pgo.txt

# benchstatで統計的比較
benchstat nopgo.txt pgo.txt
```

**出力例**:
```
name              old time/op    new time/op    delta
HandleRequest-8     125μs ± 2%     108μs ± 1%   -13.60%  (p=0.000 n=10+10)
ProcessData-8       89.3μs ± 1%    83.1μs ± 2%    -6.94%  (p=0.000 n=10+10)
```

### ベストプラクティス

1. **代表的なワークロードのプロファイルを使用**
   - 開発環境ではなく、本番に近い負荷で収集
   - 複数のシナリオがあれば、プロファイルをマージ

2. **定期的にプロファイルを更新**
   - コードが変化したら再収集（月1回程度）
   - CIパイプラインに組み込むことも可能

3. **ビルド時間の増加を考慮**
   - PGO有効時は3-5%程度ビルド時間が増加
   - CI/CDパイプラインでは本番ビルドのみ有効化

### 注意事項

- プロファイルが古すぎると逆効果の可能性
- プロファイルのサイズは通常数MB（リポジトリに含めても問題なし）
- デバッグビルド（`-gcflags="-N -l"`）では効果がない

### 参考資料

- [Profile-guided optimization - Go公式](https://go.dev/doc/pgo)
- [Profile-guided optimization in Go 1.21](https://go.dev/blog/pgo)
- [Using PGO for your Go apps - Google Cloud](https://cloud.google.com/blog/products/application-development/using-profile-guided-optimization-for-your-go-apps)
- [A Deep Look Into Golang PGO](https://theyahya.com/posts/go-pgo/)

---

## pprof Labels によるコンテキスト別プロファイリング

### 問題

複数のエンドポイントやワーカーが同じ関数を呼ぶ場合、**どのコンテキストでボトルネックが発生しているか**分かりません。

```go
// この関数は複数のエンドポイントから呼ばれる
func processData(data []byte) error {
    // CPUプロファイルには "processData が重い" としか出ない
    // どのエンドポイントで重いのか不明
}
```

### 解決策: pprof Labels

**ラベル**を使うと、ゴルーチンにタグを付けてプロファイルを分類できます。

### 基本的な使い方

```go
package main

import (
    "context"
    "runtime/pprof"
)

func handleAPIRequest(endpoint string, data []byte) {
    // エンドポイント名をラベルとして設定
    labels := pprof.Labels("endpoint", endpoint, "method", "POST")
    pprof.Do(context.Background(), labels, func(ctx context.Context) {
        // この中の処理はラベル付きでプロファイルされる
        processRequest(ctx, data)

        // 新しいゴルーチンもラベルを継承
        go backgroundTask(ctx)  // "endpoint"と"method"ラベルを継承
    })
}

func processRequest(ctx context.Context, data []byte) {
    // CPUプロファイルにラベル情報が記録される
    result := heavyComputation(data)
    saveResult(result)
}
```

### 実践例: HTTPハンドラ

```go
package main

import (
    "context"
    "net/http"
    "runtime/pprof"
)

func withProfiling(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        labels := pprof.Labels(
            "endpoint", endpoint,
            "method", r.Method,
            "user_type", getUserType(r),
        )
        pprof.Do(r.Context(), labels, func(ctx context.Context) {
            // 新しいcontextを作成してハンドラに渡す
            handler(w, r.WithContext(ctx))
        })
    }
}

func main() {
    http.HandleFunc("/api/search", withProfiling("/api/search", searchHandler))
    http.HandleFunc("/api/users", withProfiling("/api/users", usersHandler))
    http.HandleFunc("/api/health", withProfiling("/api/health", healthHandler))

    http.ListenAndServe(":8080", nil)
}

func getUserType(r *http.Request) string {
    // ユーザータイプを判定（例: プレミアムユーザーか無料ユーザーか）
    token := r.Header.Get("Authorization")
    if isPremium(token) {
        return "premium"
    }
    return "free"
}
```

### プロファイル分析

```bash
# プロファイル取得
curl -o cpu.prof http://localhost:6060/debug/pprof/profile?seconds=30

# pprofで解析
go tool pprof cpu.prof
```

**pprof コマンド内で**:

```bash
# 利用可能なタグを表示
(pprof) tags
endpoint: /api/search (45.2%)
endpoint: /api/users (32.1%)
endpoint: /api/health (22.7%)

method: POST (78.3%)
method: GET (21.7%)

user_type: premium (15.8%)
user_type: free (84.2%)

# 特定のエンドポイントに絞り込み
(pprof) tagfocus=endpoint:/api/search
(pprof) top
Total: 226s
     89s 39.4%  main.searchIndex
     45s 19.9%  main.parseQuery
     38s 16.8%  regexp.(*Regexp).FindAllString
     ...

# 複数のタグで絞り込み
(pprof) tagfocus="endpoint:/api/search,user_type:premium"
(pprof) top
# プレミアムユーザーの/api/searchリクエストのみ

# タグで除外
(pprof) tagignore=endpoint:/api/health
# ヘルスチェックを除外して分析
```

### 高度な使い方: 動的なラベル

```go
func workerPool(ctx context.Context, jobs <-chan Job) {
    for job := range jobs {
        // ジョブごとに異なるラベルを設定
        labels := pprof.Labels(
            "worker_id", fmt.Sprintf("worker_%d", getWorkerID()),
            "job_type", job.Type,
            "priority", job.Priority,
        )

        pprof.Do(ctx, labels, func(ctx context.Context) {
            processJob(ctx, job)
        })
    }
}

// 分析時に「どのジョブタイプが重いか」が分かる
// (pprof) tags
// job_type: image_processing (65%)
// job_type: data_export (25%)
// job_type: email_sending (10%)
```

### ユースケース

1. **マイクロサービスのエンドポイント別分析**
   - どのAPIが最もCPUを消費しているか
   - エンドポイント別の最適化優先順位付け

2. **ワーカータイプ別のリソース消費**
   - バックグラウンドジョブの種類別分析
   - 重いジョブタイプの特定

3. **ユーザーセグメント別の負荷**
   - プレミアムユーザーと無料ユーザーの処理負荷比較
   - テナント別のリソース消費（マルチテナント環境）

### 注意事項

- **Go 1.17以前はバグあり**: ラベルが欠落する場合がある（システムコール中、C言語コード実行中など）
- **対応プロファイル**: CPU と goroutine プロファイルのみ（メモリプロファイルは非対応）
- **オーバーヘッド**: ラベル追加自体のオーバーヘッドは極めて小さい（通常無視できる）

### 参考資料

- [Profiler labels in Go - rakyll.org](https://rakyll.org/profiler-labels/)
- [runtime/pprof パッケージドキュメント](https://pkg.go.dev/runtime/pprof)
- [Proposal: Support for pprof profiler labels](https://go.googlesource.com/proposal/+/master/design/17280-profile-labels.md)

---

## benchstat による統計的ベンチマーク比較

### 問題: ベンチマークのノイズ

単一のベンチマーク実行では、以下の理由で結果が不安定です:

- CPU周波数のスケーリング（ターボブースト）
- バックグラウンドプロセスの影響
- GCのタイミングのばらつき
- メモリ配置の違い
- キャッシュの状態

**例**: 同じコードを10回実行
```
BenchmarkProcess-8   15234   78456 ns/op
BenchmarkProcess-8   14892   80123 ns/op  ← 2%遅い！
BenchmarkProcess-8   15678   76234 ns/op  ← 3%速い！
```

→ どれが「本当の性能」？

### 解決策: benchstat

benchstatは、**複数回のベンチマーク結果を統計的に比較**し、偶然による変動と本当の改善を区別します。

### インストール

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

### 基本的な使い方

#### Step 1: 複数回のベンチマーク実行

```bash
# 最適化前: 10回実行して結果を保存
go test -bench=BenchmarkSearch -count=10 -benchmem > old.txt

# コードを修正...

# 最適化後: 10回実行
go test -bench=BenchmarkSearch -count=10 -benchmem > new.txt
```

#### Step 2: 統計的比較

```bash
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

### 結果の読み方

各行の意味:

```
Search-8    113μs ± 2%    28μs ± 1%  -75.22%  (p=0.000 n=10+10)
  │         │      │       │     │      │        │       │
  │         │      │       │     │      │        │       └─ サンプル数（old=10, new=10）
  │         │      │       │     │      │        └─ p値（統計的有意性）
  │         │      │       │     │      └─ 改善率（マイナスは高速化）
  │         │      │       │     └─ 新バージョンの信頼区間
  │         │      │       └─ 新バージョンの中央値
  │         │      └─ 旧バージョンの信頼区間
  │         └─ 旧バージョンの中央値
  └─ ベンチマーク名
```

**重要な指標**:

- **± 2%**: 中央値からの95%信頼区間（変動幅が小さいほど安定）
- **-75.22%**: 改善率（マイナスは高速化、プラスは低速化）
- **p=0.000**: p値
  - **p < 0.05**: 統計的に有意な差あり ✅
  - **p ≥ 0.05**: 差は誤差範囲の可能性 ⚠️
- **(n=10+10)**: 各10回のサンプルから計算

### 統計的有意性の理解

#### 有意な差がある場合

```
Search-8    100μs ± 1%     80μs ± 2%  -20.00%  (p=0.000 n=10+10)
                                               ↑ p < 0.05 → 本当に速くなった！
```

#### 有意な差がない場合

```
Search-8    100μs ± 5%    102μs ± 4%   +2.00%  (p=0.234 n=10+10)
                                               ↑ p > 0.05 → ノイズの範囲

または

Search-8    100μs ± 3%    100μs ± 2%     ~     (p=0.684 n=10+10)
                                        ↑ チルダ = 差なし
```

→ 最適化は効果がなかった（または測定誤差に埋もれている）

### 実践例: 正規表現の最適化

```bash
# 1. ベースライン測定（最適化前）
$ go test -bench=BenchmarkRegexp -count=20 > old.txt

# 2. コードを修正（正規表現を事前コンパイル）

# 3. 最適化後の測定
$ go test -bench=BenchmarkRegexp -count=20 > new.txt

# 4. 統計的比較
$ benchstat old.txt new.txt
name            old time/op    new time/op    delta
RegexpMatch-8     15.2μs ± 3%     8.1μs ± 2%  -46.71%  (p=0.000 n=20+20)

name            old alloc/op   new alloc/op   delta
RegexpMatch-8     8.45kB ± 0%    0.23kB ± 0%  -97.28%  (p=0.000 n=20+20)

name            old allocs/op  new allocs/op  delta
RegexpMatch-8       23.0 ± 0%       3.0 ± 0%  -86.96%  (p=0.000 n=20+20)
```

**結論**:
- 実行時間が46.7%削減（p=0.000 → 確実に改善）
- メモリ割り当てが97.3%削減（劇的な改善！）

### ベストプラクティス

#### 1. サンプル数の推奨

```bash
# 最低10回、理想は20回
go test -bench=. -count=20
```

| サンプル数 | 信頼性 | 実行時間 |
|----------|-------|---------|
| 5回 | 低い | 短い |
| 10回 | 普通 | 適度 |
| 20回 | 高い（推奨） | やや長い |
| 50回 | 非常に高い | 長い |

#### 2. 環境の固定

```bash
# CPU周波数を固定（Linuxの例）
sudo cpupower frequency-set --governor performance

# 他のプロセスを停止
# ブラウザ、IDEなどを終了

# ベンチマーク実行
go test -bench=. -count=20

# 終わったら元に戻す
sudo cpupower frequency-set --governor powersave
```

#### 3. 複数のベンチマークを比較

```bash
# すべてのベンチマークを比較
benchstat old.txt new.txt

# 特定のベンチマークのみ
benchstat -filter=Search old.txt new.txt

# 改善率でソート
benchstat -sort=delta old.txt new.txt
```

#### 4. 複数テストの罠に注意

**やってはいけない例**:
```bash
# 1回目: 差がない
$ benchstat old.txt new.txt
Search-8    100μs    102μs    ~     (p=0.234)

# もう一度実行...
$ go test -bench=. -count=10 > new.txt
$ benchstat old.txt new.txt
Search-8    100μs    99μs    -1%   (p=0.048)  ← やった！

# さらにもう一度...
$ go test -bench=. -count=10 > new.txt
$ benchstat old.txt new.txt
Search-8    100μs    103μs    ~     (p=0.156)  ← あれ？
```

**問題**: p値が0.05未満になるまで再実行を繰り返すと、偽陽性（Type I エラー）の確率が増加

**正しいアプローチ**:
1. サンプル数を決める（例: 20回）
2. 一度だけ実行
3. 結果を受け入れる
4. 差がなければ「効果なし」と結論づける

### 高度な使い方

#### プロジェクションとフィルタリング（Go 1.21+）

```bash
# ベンチマーク名に条件を埋め込む
# BenchmarkSearch/size=small-8
# BenchmarkSearch/size=large-8
# BenchmarkSearch/type=regex-8
# BenchmarkSearch/type=literal-8

# サイズ別に比較
benchstat -filter=size=small old.txt new.txt
benchstat -filter=size=large old.txt new.txt

# タイプ別に比較
benchstat -filter=type=regex old.txt new.txt
```

### 参考資料

- [benchstat コマンドドキュメント](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
- [Leveraging benchstat Projections in Go Benchmark Analysis](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/)
- [Benchmarking in Go: A Comprehensive Handbook](https://betterstack.com/community/guides/scaling-go/golang-benchmarking/)

---

## GOMEMLIMIT と GC チューニング

### 従来の手法: メモリバラスト（非推奨 → 廃止）

Go 1.19以前、GCの頻度を減らすために**メモリバラスト**という技法が使われていました:

```go
// 古い手法（もう使わない！）
var ballast = make([]byte, 10<<30) // 10GBのダミー配列
```

**問題点**:
- コードが汚い
- メモリ使用量の見積もりが難しい
- コンテナ環境で誤動作の可能性

**Go 1.19以降**: `GOMEMLIMIT` を使用してください。

### 現代的な手法: GOMEMLIMIT (Go 1.19+)

`GOMEMLIMIT`は、**プログラムが使用できる最大メモリ量**を設定します。

### 基本的な使い方

```bash
# 8GBのメモリ上限を設定
GOMEMLIMIT=8GiB go run main.go

# コード内で設定
package main

import (
    "runtime/debug"
)

func main() {
    // 7GBに設定（コンテナの8GBメモリの87.5%）
    debug.SetMemoryLimit(7 * 1024 * 1024 * 1024)

    // アプリケーションコード
}
```

### GOGC との組み合わせ

| 設定 | GC発生条件 | メモリ使用量 | CPU使用量 | ユースケース |
|------|----------|------------|----------|------------|
| **GOGC=100**（デフォルト） | ヒープが2倍 | 適度 | 適度 | 一般的なアプリ |
| **GOGC=200** | ヒープが3倍 | 高い | 低い（GC頻度↓） | バッチ処理 |
| **GOGC=50** | ヒープが1.5倍 | 低い | 高い（GC頻度↑） | 低レイテンシアプリ |
| **GOMEMLIMIT=8GiB** | メモリ上限到達時 | 上限まで使用 | 最適化される | メモリ余裕がある環境 |
| **GOGC=off GOMEMLIMIT=8GiB** | メモリ上限のみ | 上限まで使用 | 最小（GC最小化） | 高スループット重視 |

### 推奨設定パターン

#### パターン1: 高スループットアプリケーション

```bash
# GCを最小化してスループット最大化
GOGC=off GOMEMLIMIT=7GiB ./myapp
```

**効果**:
- GCは絶対必要な時のみ実行
- CPU時間をGCではなくアプリケーションに使用
- メモリを最大限活用

**注意**:
- メモリ使用量が上限に近づくとGCが頻繁に発生する可能性
- コンテナメモリの70-90%を上限に設定（OOM防止）

#### パターン2: 低レイテンシアプリケーション

```bash
# GCを小刻みに実行してSTW時間を短縮
GOGC=50 GOMEMLIMIT=6GiB ./myapp
```

**効果**:
- GC一回あたりの停止時間が短い
- レイテンシのばらつきが小さい
- P99レイテンシの改善

#### パターン3: バランス型（推奨）

```bash
# デフォルトのGOGCとGOMEMLIMITを併用
GOGC=100 GOMEMLIMIT=7GiB ./myapp
```

**効果**:
- ヒープが2倍になるか、上限に達したらGC
- ほとんどのアプリケーションで適切

### 実践例: Kubernetes環境

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: app
    image: myapp:latest
    resources:
      requests:
        memory: "8Gi"
      limits:
        memory: "8Gi"
    env:
    - name: GOMEMLIMIT
      value: "7GiB"  # メモリリミットの87.5%
    - name: GOGC
      value: "100"
```

**計算根拠**:
- コンテナメモリ: 8GiB
- GOMEMLIMIT: 7GiB (87.5%)
- 余裕: 1GiB（他のメモリ使用分とバッファ）

### GC統計の確認

```bash
# GCトレースを有効化
GODEBUG=gctrace=1 GOMEMLIMIT=7GiB ./myapp
```

**出力例**:
```
gc 1 @0.002s 3%: 0.018+1.2+0.004 ms clock, 0.14+0.35/1.0/0.82+0.036 ms cpu, 4->4->1 MB, 5 MB goal, 8 P
gc 2 @0.005s 5%: 0.021+1.5+0.005 ms clock, 0.17+0.42/1.2/0.95+0.042 ms cpu, 4->5->2 MB, 6 MB goal, 8 P
...
```

**読み方**:
```
gc 1 @0.002s 3%: 0.018+1.2+0.004 ms clock, ...
│    │        │   └─ GC各フェーズの時間（STW+Mark+STW）
│    │        └─ 起動からのGC累積時間比率
│    └─ 起動からの経過時間
└─ GC番号
```

### チューニングの手順

1. **現状把握**: `gctrace=1` でGC頻度と停止時間を測定
2. **目標設定**:
   - スループット重視 → GC頻度を減らす
   - レイテンシ重視 → GC一回の時間を減らす
3. **調整**: GOMEMLIMIT と GOGC を変更
4. **測定**: 本番またはベンチマークで効果確認
5. **繰り返し**: 最適値を見つける

### 注意事項

- **OOMに注意**: GOMEMLIMITをコンテナメモリと同じに設定しない（余裕を残す）
- **段階的に調整**: 一度に大きく変えず、10-20%ずつ調整
- **モニタリング必須**: Prometheus等でメモリ使用量を監視

### 参考資料

- [GOMEMLIMIT is a game changer for high-memory applications](https://weaviate.io/blog/gomemlimit-a-game-changer-for-high-memory-applications)
- [Memory ballast and gc tuner are history](https://www.sobyte.net/post/2022-06/memory-ballast-gc-tuner/)
- [Go Optimization Guide: Memory Efficiency and GC](https://goperf.dev/01-common-patterns/gc/)
- [Tuning Go's GOGC: A Practical Guide](https://dev.to/jones_charles_ad50858dbc0/tuning-gos-gogc-a-practical-guide-with-real-world-examples-4a00)

---

## Escape Analysis（エスケープ解析）の理解

### 概要

Escape Analysisは、Goコンパイラが**変数をスタックとヒープのどちらに割り当てるかを決定する**最適化手法です。

### スタック vs ヒープ

| 割り当て先 | 速度 | GC | 解放タイミング | サイズ制限 |
|----------|-----|----|-----------|---------
| **スタック** | 非常に高速 | 不要 | 関数終了時に自動 | 小さい（通常〜数KB） |
| **ヒープ** | 遅い | 必要 | GCが管理 | 大きい（GB単位も可） |

**最適化の目標**: できるだけスタックに割り当てる

### エスケープする条件

変数が以下の場合、**ヒープにエスケープ**します:

#### 1. 関数の戻り値としてポインタを返す

```go
// NG: ヒープにエスケープ
func createUser(name string) *User {
    u := User{Name: name}
    return &u  // uのアドレスを返す → ヒープへ
}

// OK: スタックに割り当て
func createUser(name string) User {
    return User{Name: name}  // 値を返す → スタック
}
```

#### 2. インターフェースに代入される

```go
func process(v interface{}) {
    // vはヒープに割り当てられる
}

func main() {
    x := 42
    process(x)  // xはヒープにエスケープ
}
```

#### 3. クロージャでキャプチャされる

```go
func createCounter() func() int {
    count := 0  // countはヒープにエスケープ
    return func() int {
        count++
        return count
    }
}
```

#### 4. サイズが不明または大きすぎる

```go
func large() {
    // 大きすぎる（通常64KB以上）
    var huge [100000]int  // ヒープにエスケープ
}

func dynamic(n int) {
    // コンパイル時にサイズ不明
    s := make([]int, n)  // ヒープにエスケープ
}
```

#### 5. スライスやマップに格納される

```go
func store() {
    users := make([]*User, 0)
    u := User{Name: "Alice"}
    users = append(users, &u)  // uはヒープにエスケープ
}
```

### Escape Analysisの確認

```bash
go build -gcflags="-m" main.go
```

**出力例**:

```go
package main

func sum(a, b int) int {
    result := a + b
    return result
}

func createSlice(size int) []int {
    s := make([]int, size)
    return s
}

func main() {
    x := sum(1, 2)
    s := createSlice(10)
    println(x, len(s))
}
```

```bash
$ go build -gcflags="-m" main.go
# command-line-arguments
./main.go:3:6: can inline sum
./main.go:8:6: can inline createSlice
./main.go:13:6: can inline main
./main.go:14:10: inlining call to sum
./main.go:15:18: inlining call to createSlice
./main.go:9:11: make([]int, size) escapes to heap
./main.go:16:9: ... argument does not escape
```

**読み方**:
- `can inline`: インライン化可能
- `escapes to heap`: ヒープにエスケープ
- `does not escape`: エスケープしない（スタック）

### 実践例: エスケープを避ける最適化

#### 例1: ポインタを返さない

**Before（遅い）**:
```go
func processRequest(data []byte) *Result {
    r := Result{
        Status: "ok",
        Data:   transform(data),
    }
    return &r  // ヒープにエスケープ
}

// escape analysis:
// ./main.go:2:6: moved to heap: r
```

**After（速い）**:
```go
func processRequest(data []byte) Result {
    return Result{
        Status: "ok",
        Data:   transform(data),
    }  // スタックに割り当て
}

// escape analysis:
// ./main.go:1:6: can inline processRequest
```

**効果**: GC圧力の削減、メモリ割り当ての高速化

#### 例2: sync.Pool で再利用

**Before（遅い）**:
```go
func handle(w http.ResponseWriter, r *http.Request) {
    buf := new(bytes.Buffer)  // 毎回ヒープに割り当て
    writeResponse(buf, getData())
    w.Write(buf.Bytes())
}
```

**After（速い）**:
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func handle(w http.ResponseWriter, r *http.Request) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer bufferPool.Put(buf)
    buf.Reset()

    writeResponse(buf, getData())
    w.Write(buf.Bytes())
}
```

**効果**:
- ヒープ割り当てがほぼゼロ
- GC圧力の大幅削減

#### 例3: スライスの事前割り当て

**Before（遅い）**:
```go
func collect(n int) []Item {
    var items []Item  // 容量0から開始
    for i := 0; i < n; i++ {
        items = append(items, Item{ID: i})
        // 容量不足時に再割り当て → 複数回のヒープ割り当て
    }
    return items
}
```

**After（速い）**:
```go
func collect(n int) []Item {
    items := make([]Item, 0, n)  // 事前に容量確保
    for i := 0; i < n; i++ {
        items = append(items, Item{ID: i})
        // 再割り当てなし → 1回のヒープ割り当てのみ
    }
    return items
}
```

**効果**:
- ヒープ割り当て回数の削減
- メモリコピーの削減

### ベンチマークで確認

```go
func BenchmarkWithEscape(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _ = createUserPtr("Alice")  // ポインタ返す
    }
}

func BenchmarkWithoutEscape(b *testing.B) {
    for i := 0; i < b.N; i++ {
        _ = createUserValue("Alice")  // 値返す
    }
}
```

```bash
$ go test -bench=. -benchmem
BenchmarkWithEscape-8       50000000    25.3 ns/op    32 B/op    1 allocs/op
BenchmarkWithoutEscape-8   200000000     6.2 ns/op     0 B/op    0 allocs/op
```

→ エスケープを避けることで**4倍高速化、メモリ割り当てゼロ**

### 注意事項

1. **過度な最適化は避ける**: 可読性を犠牲にしない
2. **プロファイル駆動**: 実際にボトルネックか確認してから最適化
3. **コンパイラを信頼**: 多くの場合、コンパイラの判断は正しい

### 参考資料

- [Go Optimization Guide: Stack Allocations and Escape Analysis](https://goperf.dev/01-common-patterns/stack-alloc/)
- [Understanding Escape Analysis in Go](https://medium.com/@pranoy1998k/understanding-escape-analysis-in-go-b2db76be58f0)
- [Go Compiler's Escape Analysis: Boosting Performance](https://medium.com/@001gouthi/go-compilers-escape-analysis-boosting-performance-d5b8e1e2ab7d)
- [Go Wiki: Compiler And Runtime Optimizations](https://go.dev/wiki/CompilerOptimizations)

---

## まとめ

このセクションで紹介した高度なテクニックを活用することで、より効率的なプロファイリングと最適化が可能になります。

### 推奨される学習順序

1. **基礎**: pprof と runtime/trace の基本（Part 1-3）
2. **統計**: benchstat で正確な効果測定
3. **分析**: pprof Labels でコンテキスト別分析
4. **最適化**: Escape Analysis と GOMEMLIMIT でメモリ効率改善
5. **本番適用**: Flight Recorder で本番診断
6. **長期最適化**: PGO で継続的な性能向上

### 次のステップ

- 実際のプロジェクトでこれらの技術を試す
- 本番環境にFlight RecorderとPGOを導入
- チームでベンチマーク文化を醸成（benchstat活用）

---

お疲れさまでした！
