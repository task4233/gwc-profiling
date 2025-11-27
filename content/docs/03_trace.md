---
title: "Part 2: runtime/traceでの解析"
weight: 40
---

## runtime/traceで見つける問題

runtime/traceは**時系列での実行状態**を可視化するツールです。

**得意なこと**:
- ゴルーチンの生成・実行・ブロッキング
- 同期処理（Mutex、Channel）の待ち時間
- GCの発生タイミングと影響
- プロセッサ（P）の利用状況

**pprofとの違い**:
- pprof: 関数ごとの**累積**リソース消費
- trace: 時系列での**瞬間的な**実行状態

---

## 演習1: トレースの取得（5分）

### 1-1. サーバの起動

```bash
cd exercises

# トレース付きでサーバを起動
go run main.go -trace=trace.out
```

### 1-2. 負荷をかける

別のターミナルで：

```bash
cd client
go run test_client.go
```

**重要**: pprofの時と同じ負荷で実行してください。

### 1-3. サーバの停止

`Ctrl+C` で停止すると `trace.out` が保存されます。

```
✅ トレース保存完了
```

---

## 演習2: Goroutine Analysisで並行処理を確認（10分）

### 2-1. トレースビューアの起動

```bash
go tool trace trace.out
```

ブラウザが開き、以下のようなメニューが表示されます：

**runtime/trace の主要ビュー**:

| ビュー | 説明 | 用途 |
|--------|------|------|
| **View trace** | タイムラインビュー | ゴルーチンの実行状態を時系列で表示 |
| **Goroutine analysis** | ゴルーチン分析 | ゴルーチンの生成数・実行時間・待ち時間を集計 |
| **Network blocking profile** | ネットワークブロック | ネットワーク待ち時間の分析 |
| **Synchronization blocking profile** | 同期ブロック | Mutex/Channelでのブロック時間 |
| **Syscall blocking profile** | システムコールブロック | システムコール待ち時間 |
| **Scheduler latency profile** | スケジューラレイテンシ | ゴルーチンが実行可能になってから実際に実行されるまでの遅延 |

```
┌─────────────────────────────────────┐
│ Trace Viewer - Main Menu            │
├─────────────────────────────────────┤
│ • View trace ←──────────────────┐  │
│   └→ タイムラインビュー         │  │
│                                  │  │
│ • Goroutine analysis ←──────┐   │  │
│   └→ ゴルーチン統計         │   │  │
│                              ↓   ↓  │
│ • Synchronization blocking   最も重要│
│   └→ ミューテックス待ち             │
└─────────────────────────────────────┘
```

### 2-2. Goroutine Analysisを開く

メニューから **"Goroutine analysis"** をクリック

**質問**: 何個のゴルーチンが作られましたか？

{{% details "ヒント" %}}
表の上部に "Goroutines" の総数が表示されています。
数百〜数千個のゴルーチンが生成されていませんか？
{{% /details %}}

{{% details "発見すべきこと" %}}
- ファイル数 × リクエスト数分のゴルーチンが生成されている
- 多くのゴルーチンが短時間で終了している
- ゴルーチン生成のオーバーヘッドが大きい
{{% /details %}}

### 2-3. ゴルーチンの詳細を確認

特定のゴルーチンをクリックして詳細を見てみましょう。

**観察ポイント**:
- Execution time: 実際の実行時間
- Network wait: ネットワーク待ち
- Sync block: 同期待ち
- GC time: GC中の時間

**質問**: 実行時間と比べて待ち時間は長いですか？

---

## 演習3: Synchronization blockingを確認（10分）

### 3-1. メニューに戻る

ブラウザで前のページに戻り、メニューから **"Synchronization blocking profile"** をクリック

### 3-2. ブロッキングを確認

**質問**: どのミューテックスで最も待ち時間が長いですか？

{{% details "ヒント" %}}
グラフの中で `sync.(*Mutex).Lock` を探してください。
その親関数は何ですか？
{{% /details %}}

{{% details "発見すべきこと" %}}
- `main.searchFile` 内でのミューテックスロック
- グローバルな `resultsMu` での競合
- 全ゴルーチンが同じロックを取りに来ている
- コード: `main.go:175`付近
  ```go
  resultsMu.Lock()
  allResults = append(allResults, match)
  resultsMu.Unlock()  // ここで競合！
  ```
{{% /details %}}

**pprofとの違い**:
- pprof: ロック取得の**回数**は分かる
- trace: ロック待ちの**時間**が分かる ✅

---

## 演習4: GCの影響を確認（10分）

### 4-1. View traceを開く

メニューから **"View trace"** をクリック

**注意**: 初回表示に時間がかかる場合があります。

### 4-2. タイムラインを確認

**View traceの画面構成**:

```
┌──────────────────────────────────────────────────────────────┐
│ Time (ms)   0    100   200   300   400   500   600   700    │ ← 時間軸
├──────────────────────────────────────────────────────────────┤
│ Goroutines  ▂▂▂█████▂▂▂▂███▂▂▂▂▂▂████▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂  │ ← 全ゴルーチン数
├──────────────────────────────────────────────────────────────┤
│ Heap        ▁▂▃▄▅▆▇█████████▇▆▅▄▃▂▁▁▁▂▃▄▅▆▇████▇▆▅▄▃▂▁  │ ← ヒープサイズ
├──────────────────────────────────────────────────────────────┤
│ Threads     ▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂  │ ← スレッド数
├──────────────────────────────────────────────────────────────┤
│ GC          │      GC      │           │      GC      │     │ ← GCイベント
├──────────────────────────────────────────────────────────────┤
│ Proc 0      ████─────███████──────██████───────███████─     │ ← CPU 0
│ Proc 1      ███████────█████████───────██████████──────     │ ← CPU 1
│ Proc 2      ──────██████──────██████████───────████████     │ ← CPU 2
│   ...                                                        │
└──────────────────────────────────────────────────────────────┘
```

**各要素の意味**:
- **Goroutines**: ゴルーチンの総数（スパイク＝大量生成）
- **Heap**: ヒープメモリ使用量（急上昇＝メモリリーク候補）
- **Threads**: OSスレッド数（通常は安定）
- **GC**: ガベージコレクションの発生（緑のバー）
- **Proc N**: 各CPU（P）の状態
  - **色付き**: ゴルーチン実行中
  - **白い部分**: アイドル（待機中）
  - **細い緑線**: GC実行中

**操作方法**:
- `W`/`S`: ズームイン/アウト
- `A`/`D`: 左右にスクロール
- マウスドラッグ: 移動
- クリック: 詳細情報を表示

### 4-3. GCを探す

下部の "GC" という緑色のバーを探してください。

**質問**: GCはどのくらいの頻度で発生していますか？

{{% details "ヒント" %}}
タイムラインを横にスクロールして、GCバーの間隔を確認してください。
数秒に1回？もっと頻繁？
{{% /details %}}

{{% details "発見すべきこと" %}}
- 頻繁なGC（数百ミリ秒に1回程度）
- GC中は全プロセッサが停止（Stop The World）
- 原因: 大量のメモリ割り当て（pprofで発見した問題）
{{% /details %}}

### 4-4. プロセッサの利用状況

上部のP0, P1, P2...のタイムラインを確認

**質問**: 全てのプロセッサが均等に使われていますか？

{{% details "観察ポイント" %}}
- 白い部分: アイドル状態
- 色のついた部分: ゴルーチン実行中
- 緑の細い線: GC中

理想的には全プロセッサが常に動いているはずですが、
- ゴルーチンが多すぎて切り替えコストが高い
- ミューテックス待ちでアイドル時間が発生
{{% /details %}}

---

## 演習5: ゴルーチンのライフサイクルを追う（5分）

### 5-1. View traceで特定のゴルーチンを選択

タイムライン上のゴルーチン（細い横線）をクリック

**表示される情報**:
- Start: 開始時刻
- Duration: 実行時間
- Events: イベント一覧

### 5-2. イベントを確認

Events欄を見ると、ゴルーチンの状態遷移が分かります：

```
GoCreate     - ゴルーチン生成
GoStart      - 実行開始
GoSysCall    - システムコール（ファイル読み込み等）
GoBlock      - ブロック（ミューテックス待ち）
GoUnblock    - ブロック解除
GoEnd        - 終了
```

**質問**: ゴルーチンは何にブロックされていますか？

{{% details "発見すべきこと" %}}
- GoBlock: sync.Mutex.Lock での待ち
- 短命なゴルーチンが大量に生成・破棄されている
- ワーカープールを使えば改善できそう
{{% /details %}}

---

## pprofでは見えなかった問題（まとめ）

### 1. ゴルーチンの過剰生成

**pprof**:
- ❌ 何個のゴルーチンが作られたか分からない
- ❌ ゴルーチンの寿命が分からない

**trace**:
- ✅ 数百〜数千個のゴルーチンが生成されていることが分かる
- ✅ ほとんどが短命（数ミリ秒）で終了していることが分かる
- ✅ ワーカープール導入の根拠になる

### 2. ミューテックスでの競合

**pprof**:
- ⚠️ ロック取得の回数は分かる
- ❌ 待ち時間の長さは分からない

**trace**:
- ✅ ロック待ちの時間が可視化される
- ✅ どのミューテックスがボトルネックか分かる
- ✅ 複数ゴルーチンが同時に待っている様子が見える

### 3. GCの影響

**pprof**:
- ⚠️ メモリ割り当ての量は分かる
- ❌ GCのタイミングと影響は分からない

**trace**:
- ✅ GCの発生頻度が分かる
- ✅ GC中の停止時間が分かる
- ✅ GCがアプリケーション全体に与える影響が見える

---

## 発見した問題（まとめ）

### 並行処理の問題

1. **ゴルーチンの無制限生成**
   - 場所: `main.go:120`
   - 影響: メモリ消費、スケジューリングオーバーヘッド
   - コード:
   ```go
   // ファイルごとにゴルーチンを生成
   wg.Add(1)
   go func(fp string) {
       defer wg.Done()
       searchFile(fp, pattern)
   }(filePath)
   ```

2. **グローバルロックでの競合**
   - 場所: `main.go:175`
   - 影響: 全ゴルーチンが直列化される
   - コード:
   ```go
   resultsMu.Lock()
   allResults = append(allResults, match)
   resultsMu.Unlock()  // 全員がここで待つ！
   ```

3. **頻繁なGC**
   - 原因: pprofで発見した大量のメモリ割り当て
   - 影響: 数百ミリ秒ごとのStop The World

---

## 気づきの共有（5分）

グループで以下を共有してください：

1. runtime/traceで発見した問題は何でしたか？
2. pprofでは見えなかった問題はありましたか？
3. View traceのタイムラインから何が読み取れましたか？

---

## 発展: Flight Recorder（Go 1.25の新機能）

### 本番環境での常時トレース

従来のトレースは計画的なプロファイリングに適していますが、**本番環境で問題が発生した瞬間のトレース**を取得するのは困難でした。

Go 1.25で導入された**Flight Recorder**は、この問題を解決します：

- **常時トレース収集**: オーバーヘッド1-2%で常時有効化可能
- **循環バッファ**: メモリ上の最新数秒分のみを保持
- **問題検出時にダンプ**: レイテンシスパイクやエラー発生時に保存

### 簡単な使用例

```go
import "runtime/trace"

func main() {
    trace.Start(os.Stderr)
    defer trace.Stop()

    // 問題検出時
    if detectLatencySpike() {
        f, _ := os.Create("problem_trace.out")
        trace.FlightRecorder(f)
        f.Close()
    }
}
```

詳細は [高度なプロファイリングテクニック: Flight Recorder](../06_advanced_techniques#flight-recordergo-125の新機能) を参照してください。

---

## 参考資料

### 公式ドキュメント

- [Execution Tracer - Go公式ブログ](https://go.dev/blog/execution-tracer)
- [More powerful Go execution traces - Go 1.21+の改善](https://go.dev/blog/execution-traces-2024)
- [Flight Recorder in Go 1.25](https://go.dev/blog/flight-recorder)
- [runtime/trace パッケージ](https://pkg.go.dev/runtime/trace)

### トレース解析手法

- [Go execution tracer design document](https://go.googlesource.com/proposal/+/ac09a140c3d26f8bb62cbad8969c8b154f93ead6/design/60773-execution-tracer-overhaul.md)
- [Debugging performance issues in Go programs - GopherCon EU 2019](https://www.youtube.com/watch?v=6A-h-A_1nws)

### GopherCon トーク

- [GopherCon 2021: Felix Geisendörfer - The Busy Developer's Guide to Go Profiling, Tracing and Observability](https://www.youtube.com/watch?v=7hJz_WOx8JU)
- [Rhys Hiltner - An Introduction to "go tool trace"](https://www.youtube.com/watch?v=V74JnrGTwKA)

---

## 次のステップ

[Part 3: 統合的な最適化](../04_optimization) に進み、pprofとtraceの両方の情報を使って問題を修正します。
