---
title: "Part 2-1: Trace Basics"
weight: 160
---

## Trace基本操作の演習

### 演習の目的

基本的なtraceの使い方を学び、タイムラインビューでgoroutineの挙動、ブロッキング、GCを可視化します。

演習ディレクトリ: `exercises/trace/01-basics/`

### 問題の概要

このプログラムには、traceで可視化できる典型的な問題が含まれています：

1. **過剰なgoroutine生成**: 1000個のgoroutineを一度に作成
2. **チャネルでのブロッキング**: バッファなしチャネルで送受信が待機
3. **頻繁なGC**: 大量の短命なオブジェクト生成

---

## 演習手順

### ステップ1: トレースの取得

```bash
cd exercises/trace/01-basics/

# トレースを取得
go run main.go -trace=trace.out
```

実行すると、`trace.out`ファイルが生成されます。

### ステップ2: トレースビューアの起動

```bash
go tool trace trace.out
```

ブラウザが自動的に開き、複数のビューが表示されます。

---

## View trace（タイムラインビュー）

最も重要なビューです。プログラムの実行状態を時系列で可視化します。

### 画面構成

#### 横軸: 時間

- プログラムの開始からの経過時間
- マウスホイールやW/Sキーでズーム

#### 縦軸: 実行リソース

| 行 | 説明 |
|---|------|
| **Goroutines** | 各goroutineの状態（緑=実行、グレー=ブロック、黄=待機） |
| **Heap** | ヒープメモリサイズの推移 |
| **Threads** | OSスレッド（M）の数 |
| **GC** | GCイベント（マーク、スイープ） |
| **PROCS** | プロセッサ（P）ごとの実行状態 |

### キーボード操作

| キー | 動作 |
|------|------|
| `W` / `S` | ズームイン/アウト |
| `A` / `D` | 左右スクロール |
| `1` / `2` / `3` / `4` | 詳細レベル切り替え |
| マウスクリック | イベント詳細表示 |

### 問題版で観察できること

#### 1. 過剰なgoroutine生成

- **Goroutines**行が大量に表示される（1000個）
- 画面が goroutine で埋め尽くされる
- 多くのgoroutineが黄色（スケジュール待ち）

**問題点**:
- goroutineの生成・管理オーバーヘッド
- スケジューラの負荷
- メモリ消費

#### 2. チャネルでのブロッキング

- goroutineがグレーで長時間表示される
- チャネル送信/受信待ちでブロック

**問題点**:
- バッファなしチャネルによる同期待ち
- producer/consumer間の不均衡

#### 3. 頻繁なGC

- **GC**行に頻繁に"GC"マークが表示される
- GC中は全goroutineが一時停止（STW: Stop-The-World）

**問題点**:
- 大量の短命オブジェクトの割り当て
- GCのオーバーヘッド

---

## Goroutine analysis

各goroutineが何に時間を使っているかを集計します。

### 表示される指標

問題版で確認できる項目：

```
Goroutine analysis
Total: 10000 goroutines

Execution:        2000ms  (20%)
Sync block:       6000ms  (60%)
Scheduler wait:   2000ms  (20%)
```

| 項目 | 問題版 | 説明 |
|------|--------|------|
| **Execution** | 20% | 実際のCPU実行時間（低い） |
| **Sync block** | 60% | チャネル待ちでブロック（高い） |
| **Scheduler wait** | 20% | スケジュール待ち（高い） |

**分析**:
- Sync blockが多い → チャネルでのブロッキング問題
- Scheduler waitが多い → goroutineが多すぎる
- Executionが少ない → 実行効率が悪い

---

## Synchronization blocking profile

チャネルやmutexでのブロッキングをpprof形式で表示します。

```bash
# Synchronization blocking profile をクリック
```

問題版では、以下の関数でブロッキングが多いことが確認できます：

```
main.producer  (chan send)
main.consumer  (chan receive)
```

---

## Scheduler latency profile

goroutineがスケジュール待ちで費やした時間を表示します。

問題版では：
- 1000個のgoroutineが同時にスケジュール待ち
- CPUコア数（通常4-8個）に対してgoroutineが多すぎる

---

## 改善版の実行

### ステップ3: 改善版のトレース取得

```bash
go run main_fixed.go -trace=trace_fixed.out
go tool trace trace_fixed.out
```

### 改善内容

#### 1. ワーカープールパターン

```go
// Before: 過剰なgoroutine生成
for i := 0; i < 1000; i++ {
    go worker()
}

// After: ワーカープール（固定数）
const numWorkers = 10
for i := 0; i < numWorkers; i++ {
    go worker(jobs)
}
```

#### 2. バッファ付きチャネル

```go
// Before: バッファなし
ch := make(chan Task)

// After: バッファあり
ch := make(chan Task, 100)
```

#### 3. sync.Poolでオブジェクト再利用

```go
// Before: 毎回新規作成
buf := make([]byte, 1024)

// After: sync.Poolで再利用
var bufPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 1024)
    },
}

buf := bufPool.Get().([]byte)
defer bufPool.Put(buf)
```

---

## Before/After 比較

### View traceでの比較

2つのブラウザタブで開いて比較します：

```bash
# タブ1: 問題版
go tool trace trace.out

# タブ2: 改善版
go tool trace trace_fixed.out
```

| 項目 | 問題版 | 改善版 |
|------|--------|--------|
| **Goroutine数** | 1000+ | 10-20 |
| **ブロッキング** | 長い（グレーが多い） | 短い（緑が多い） |
| **GC頻度** | 頻繁 | まれ |
| **CPU使用率** | 低い（黄色が多い） | 高い（緑が多い） |

### Goroutine analysisでの比較

| 項目 | 問題版 | 改善版 |
|------|--------|--------|
| **Execution** | 20% | 80% |
| **Sync block** | 60% | 10% |
| **Scheduler wait** | 20% | 5% |

**改善効果**:
- Executionが増加 → 実行効率向上
- Sync blockが減少 → ブロッキング削減
- Scheduler waitが減少 → スケジューラ負荷軽減

---

## 典型的な問題パターンの検出

### パターン1: Goroutineリーク

**症状**:
- 時間が経過してもgoroutine数が減らない
- ブロックされたままのgoroutineが存在

**View traceでの確認**:
- Goroutines行でグレーのままの部分
- 終了しないgoroutine

**原因**:
- チャネルを`close`していない
- contextのキャンセルが伝播していない

### パターン2: チャネルブロッキング

**症状**:
- Sync blockが長い
- グレーの部分が多い

**View traceでの確認**:
- goroutineがチャネル送受信で長時間待機

**解決策**:
- バッファ付きチャネル
- 複数のconsumer起動

### パターン3: Mutex競合

**症状**:
- Sync blockでmutex待ち
- 複数goroutineが同じmutexで待機

**View traceでの確認**:
- 同時に多数のgoroutineがブロック

**解決策**:
- RWMutex
- ロック範囲の縮小
- Sharding（細粒度ロック）

### パターン4: 頻繁なGC

**症状**:
- GCマークが頻繁に表示
- GC pause時間が長い

**View traceでの確認**:
- **GC**行に頻繁にイベント
- GC中は全goroutineが停止

**解決策**:
- アロケーション削減
- sync.Pool
- 事前メモリ確保

---

## トラブルシューティング

### トレースが開かない

```bash
# ポートを変更
go tool trace -http=localhost:8081 trace.out
```

### ファイルが大きすぎる

```bash
# トレース時間を短くする
# main.goを編集して処理を減らす
```

### ブラウザが重い

- 短い時間範囲だけズームインする
- W/Sキーで適切なズームレベルを見つける

---

## まとめ

Trace Basicsを通じて学んだこと：

1. **View trace**: タイムラインでgoroutineの挙動を可視化
2. **Goroutine analysis**: 時間の使われ方を集計
3. **Blocking profile**: ブロッキング箇所の特定
4. **改善検証**: Before/After比較で効果を確認

次は[Trace Annotation]({{< relref "17_trace_annotation.md" >}})でTask/Regionを使ったカスタマイズを学びます。
