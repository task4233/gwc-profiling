# File Search MCP Server - プロファイリング演習

このディレクトリには、pprofとruntime/traceを学ぶための演習用MCPサーバが含まれています。

## 概要

ファイル内容を正規表現で検索するシンプルなMCPサーバです。
このサーバには意図的にパフォーマンス上の問題が組み込まれており、pprofとruntime/traceを使って問題を発見・解決することが目的です。

## 組み込まれている問題

1. **CPU問題**（pprofで発見しやすい）
   - 正規表現を毎回コンパイル
   - 非効率な文字列処理

2. **メモリ問題**（pprofで発見しやすい）
   - ファイル全体をメモリに読み込み
   - 不要な文字列のコピー

3. **並行処理問題**（runtime/traceで発見しやすい）
   - ゴルーチンの無制限生成
   - グローバルロックでの競合

## セットアップ

```bash
# 依存関係のインストール
cd exercises
go mod download
```

## 使い方

### 1. サーバの起動（通常モード）

```bash
go run main.go
```

### 2. 負荷テストの実行

別のターミナルで：

```bash
cd client
go run test_client.go
```

### 3. CPUプロファイルの取得

```bash
# サーバ起動
go run main.go -cpuprofile=cpu.prof

# 別ターミナルで負荷テスト
cd client
go run test_client.go

# サーバをCtrl+Cで停止（プロファイル保存）

# 解析
go tool pprof -http=:8080 cpu.prof
```

### 4. runtime/traceの取得

```bash
# サーバ起動
go run main.go -trace=trace.out

# 別ターミナルで負荷テスト
cd client
go run test_client.go

# サーバをCtrl+Cで停止

# 解析
go tool trace trace.out
```

### 5. メモリプロファイルの取得

```bash
# サーバ起動
go run main.go -memprofile=mem.prof

# 負荷テスト実行後、Ctrl+Cで停止

# 解析
go tool pprof -http=:8080 mem.prof
```

## ワークショップの流れ

詳細は[ワークショップガイド](../content/docs/)を参照してください。

1. **Part 1: プログラム理解**（10分）
   - サーバの動作確認
   - コードの理解

2. **Part 2: pprofで解析**（25分）
   - CPUプロファイル
   - メモリプロファイル
   - ボトルネックの発見

3. **Part 3: runtime/traceで解析**（25分）
   - トレースの取得
   - ゴルーチンの可視化
   - pprofでは見えなかった問題の発見

4. **Part 4: 最適化**（30分）
   - 問題の修正
   - 効果測定
   - pprofとtraceの使い分け

## MCPツール仕様

### search

ファイル内容を正規表現で検索します。

**入力:**
```json
{
  "pattern": "func.*",
  "paths": ["."],
  "max_results": 100
}
```

**出力:**
```json
{
  "matches": [
    {
      "file": "main.go",
      "line": 42,
      "content": "func main() {"
    }
  ],
  "total": 1
}
```
