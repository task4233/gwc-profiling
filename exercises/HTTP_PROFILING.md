# HTTPサーバ版でのプロファイリング手順

通常のHTTPサーバ実装を使って、pprofでの解析を正しく行う方法です。

## 特徴

- ✅ 1つのサーバプロセスで複数のリクエストを処理
- ✅ `regexp.Compile`や`searchFile`の処理が正しくプロファイルに記録される
- ✅ MCPプロトコルの複雑さがない、シンプルなHTTP実装

## 使い方

### 0. テストデータの作成（推奨）

プロファイルで問題を顕在化させるため、大量のGoファイルを作成します：

```bash
cd exercises
bash create_test_data.sh
```

これにより`testdata/large/`に100個のGoファイルが作成されます。

### 1. サーバの起動

ターミナル1で：

```bash
cd exercises
go run http_server.go -cpuprofile=cpu.prof
```

出力例：
```
🔍 File Search HTTP Server
📍 http://localhost:8080 で起動中...
📌 エンドポイント:
   POST /search - ファイル検索
   GET  /health - ヘルスチェック

📊 CPUプロファイル: cpu.prof
```

### 2. 負荷テストの実行

ターミナル2で：

```bash
cd exercises/client
go run http_test_client.go
```

出力例：
```
🚀 HTTP負荷テスト開始
   サーバ: http://localhost:8080/search
   同時接続数: 50
   総リクエスト数: 10000
Client 0: 20/200 リクエスト完了
Client 1: 20/200 リクエスト完了
...
✅ 負荷テスト完了
   成功: 10000/10000
   失敗: 0/10000
   所要時間: 45.2s
   スループット: 221.24 req/sec

📊 プロファイルを確認するには:
   1. サーバを Ctrl+C で停止
   2. go tool pprof -http=:8081 cpu.prof
```

**注意**: テストデータを作成した場合、より多くのファイルが検索されるため、処理時間が長くなります。

### 3. サーバの停止

ターミナル1で `Ctrl+C` を押すと、プロファイルが保存されます：

```
🛑 シグナルを受信しました。クリーンアップ中...
✅ CPUプロファイル保存完了
```

### 4. プロファイルの解析

```bash
cd exercises
go tool pprof -http=:8081 cpu.prof
```

ブラウザで http://localhost:8081 を開くと、以下が確認できます：

#### Flame Graphで確認すべきこと
- `regexp.Compile` が頻繁に呼ばれている
- `searchFile` 関数内での正規表現コンパイルのコスト
- `strings.Split` での文字列処理のコスト

#### Topビューで確認すべきこと
```
Showing nodes accounting for 2.5s, 80% of 3.1s total
      flat  flat%   sum%        cum   cum%
     1.2s 38.71% 38.71%      2.0s 64.52%  regexp.Compile
     0.5s 16.13% 54.84%      0.8s 25.81%  strings.Split
     0.3s  9.68% 64.52%      0.5s 16.13%  os.ReadFile
     0.2s  6.45% 70.97%      0.3s  9.68%  searchFile
```

## メモリプロファイルの取得

### 1. サーバの起動（メモリプロファイル有効）

```bash
go run http_server.go -memprofile=mem.prof
```

### 2. 負荷テストの実行

```bash
cd client
go run http_test_client.go
```

### 3. サーバの停止とプロファイル解析

`Ctrl+C` で停止後：

```bash
go tool pprof -http=:8081 mem.prof
```

## トレースの取得

### 1. サーバの起動（トレース有効）

```bash
go run http_server.go -trace=trace.out
```

### 2. 負荷テストの実行

```bash
cd client
go run http_test_client.go
```

### 3. サーバの停止とトレース解析

`Ctrl+C` で停止後：

```bash
go tool trace trace.out
```

## 複数のプロファイルを同時に取得

```bash
go run http_server.go -cpuprofile=cpu.prof -memprofile=mem.prof -trace=trace.out
```

## 比較: MCP版 vs HTTP版

| 項目 | MCP版 (main.go) | HTTP版 (http_server.go) |
|------|----------------|------------------------|
| プロトコル | stdio (JSON-RPC 2.0) | HTTP REST API |
| プロセス数 | 各リクエストごとに新規 | 1つのサーバプロセス |
| プロファイル | 起動コストが大半 | 実処理が記録される |
| 複雑さ | MCPプロトコル | シンプルなHTTP |
| 用途 | Claude Code等のツール連携 | 一般的なHTTPクライアント |

## トラブルシューティング

### サーバが起動しない

```bash
# ポートが使用中の場合
go run http_server.go -port=8081 -cpuprofile=cpu.prof
```

### クライアントが接続できない

```bash
# ヘルスチェックを確認
curl http://localhost:8080/health
```

### プロファイルに何も記録されない

- サーバを起動してから、必ず負荷テストを実行してください
- サーバを `Ctrl+C` で停止するまでプロファイルは保存されません
