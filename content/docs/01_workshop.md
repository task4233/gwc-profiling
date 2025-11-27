---
title: "ワークショップ概要"
weight: 20
---

## Go プロファイリングワークショップ

### 目的

pprofとruntime/traceの**特性を理解**し、それぞれのツールが得意な問題を実践的に学びます。

**重要**: このワークショップでは、同じプログラムを異なるツールで解析することで、各ツールの強みと使い分けを体験します。

### タイムスケジュール（120分）

| 時間 | 内容 | 形式 |
|------|------|------|
| 10分 | 導入・環境確認 | 講義 |
| 10分 | プログラム理解 | ハンズオン |
| 25分 | **Part 1: pprofで解析** | 講義5分 + 実践20分 |
| 25分 | **Part 2: runtime/traceで解析** | 講義5分 + 実践20分 |
| 30分 | **Part 3: 統合的な最適化** | 講義5分 + 実践25分 |
| 20分 | まとめ・気づきの共有 | ディスカッション |

---

## 対象プログラム

**ファイル検索MCPサーバ** - Go言語ファイルを正規表現で検索するMCPサーバ

### 主要機能

`search` ツール:
- 入力: 正規表現パターン、検索パス、最大結果数
- 出力: マッチしたファイル、行番号、内容

### 組み込まれている問題

このプログラムには意図的に以下の問題が含まれています：

#### 1. CPU問題（pprofで発見しやすい）
- 正規表現を毎回コンパイル
- 非効率な文字列処理

#### 2. メモリ問題（pprofで発見しやすい）
- ファイル全体をメモリに読み込み
- 不要な文字列のコピー

#### 3. 並行処理問題（runtime/traceで発見しやすい）
- ゴルーチンの無制限生成
- グローバルロックでの競合

#### 4. GC圧力（runtime/traceで可視化しやすい）
- 大量のメモリ割り当てによる頻繁なGC

---

## ワークショップの流れ

### Part 1: pprofで解析（25分）

**目標**: CPUとメモリのボトルネックを特定する

1. CPUプロファイルの取得と解析
2. メモリプロファイルの取得と解析
3. ボトルネックの特定

**発見すべきこと**:
- どの関数がCPU時間を消費しているか
- どこでメモリ割り当てが多いか
- 正規表現コンパイルのコスト

### Part 2: runtime/traceで解析（25分）

**目標**: 並行処理の問題を可視化する

1. トレースの取得
2. Goroutine Analysisでの確認
3. 同期ブロッキングの発見

**発見すべきこと**:
- ゴルーチンの生成数と寿命
- ミューテックスでのブロッキング
- GCの頻度と影響
- **pprofでは見えなかった問題**

### Part 3: 統合的な最適化（30分）

**目標**: 両ツールの情報を活用して最適化する

1. 問題の優先順位付け
2. 修正の実施
3. 効果測定

**学ぶこと**:
- pprofとtraceの使い分け
- 最適化のアプローチ
- トレードオフの判断

---

## 環境確認

ワークショップ開始前に以下を確認してください：

```bash
# リポジトリのクローン
git clone https://github.com/task4233/gwc-profiling.git
cd gwc-profiling/exercises

# 依存関係のインストール
go mod download

# テストの実行
go test -v

# サーバのビルド
go build -o file-search-mcp main.go
```

すべて成功すれば準備完了です。

---

## 参考資料

このワークショップの内容をより深く理解するための参考資料：

### 基本
- [Go Diagnostics](https://go.dev/doc/diagnostics) - プロファイリングの総合ガイド
- [Profiling Go Programs](https://go.dev/blog/pprof) - pprof入門
- [Execution Tracer](https://go.dev/blog/execution-tracer) - runtime/trace入門

### 推奨GopherConトーク
- [Dave Cheney - Two Go Programs, Three Different Profiling Techniques (2019)](https://www.youtube.com/watch?v=nok0aYiGiYA)
- [Felix Geisendörfer - The Busy Developer's Guide to Go Profiling, Tracing and Observability (2021)](https://www.youtube.com/watch?v=7hJz_WOx8JU)

---

## 次のステップ

- [Part 1: pprofでの解析](../02_pprof) - CPUとメモリのボトルネック発見
- [Part 2: runtime/traceでの解析](../03_trace) - 並行処理の問題可視化
- [Part 3: 統合的な最適化](../04_optimization) - 問題の修正と効果測定
- [高度なプロファイリングテクニック](../06_advanced_techniques) - PGO、Flight Recorder、Escape Analysisなど
