---
title: "セットアップガイド"
weight: 1
---

本ドキュメントでは、ワークショップに参加するために必要な環境構築手順を説明します。

---

## 1. 環境構築

### 1.1 セットアップ確認

まず、以下のスクリプトを実行して、必要なツールがインストールされているか確認します。

```bash
git clone https://github.com/task4233/gwc-profiling.git
cd gwc-profiling
./scripts/doctor.sh
```

**期待される出力:**

```
=== セットアップ確認 ===

[Git] ✓ インストール済み (2.x.x)
[Go] ✓ インストール済み (go1.25.x)
[Graphviz] ✓ インストール済み (x.x.x)

=== すべてのセットアップが完了しています 🎉 ===
```

すべてのチェックが ✓ となれば、**1.2 と 1.3 はスキップして「2. pprof と runtime/trace の動作確認」に進んでください**。

❌ が表示された場合は、次のセクションを参照してインストールしてください。

> [!NOTE]
> **Go 1.25以上を推奨する理由**
>
> - **Flight Recorder**: Go 1.25.0以降で利用可能な新機能。本番環境での継続的なトレースが可能
> - **最新のTrace機能**: Go 1.21以降でオーバーヘッドが1-2%に削減され、Go 1.22以降でスケーラビリティが向上
>
> Go 1.21-1.24でも本ワークショップの大部分は実施可能ですが、Flight Recorderの演習にはGo 1.25.0以降が必要です。

---

### 1.2 必要なツールのインストール

#### 動作環境

- macOS
- Linux
- Windows（WSL2 環境）

> [!WARNING]
> Windows ユーザーは環境差異を避けるため、WSL2 上の Linux 環境を使用してください。

---

{{% details "Git のインストール" %}}

### macOS

```bash
# Xcode Command Line Tools に含まれています
xcode-select --install

# または Homebrew を使用
brew install git
```

### Linux / WSL2

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install -y git

# Fedora
sudo dnf install -y git
```

### 確認

```bash
git --version
```

{{% /details %}}

---

{{% details "Go のインストール" %}}

Go 1.25 以上をインストールしてください。

### macOS

```bash
brew install go
```

または [公式ダウンロードページ](https://go.dev/dl/) からインストーラをダウンロード。

### Linux / WSL2

```bash
# バージョンは適宜置き換えてください
wget https://go.dev/dl/go1.25.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz

# PATH を設定
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 確認

```bash
go version
# go1.25 以上であること
```

{{% /details %}}

---

{{% details "Graphviz のインストール" %}}

pprof のグラフ可視化機能に必要です。

### macOS

```bash
brew install graphviz
```

### Linux / WSL2

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install -y graphviz

# Fedora
sudo dnf install -y graphviz
```

### 確認

```bash
dot -V
```

{{% /details %}}

---

### 1.3 WSL2 環境での注意事項

WSL2 環境では、pprof や trace の Web UI をホスト OS（Windows）のブラウザで閲覧する必要があります。

{{% details "WSL2 での Web UI アクセス方法" %}}

### pprof / trace Web UI へのアクセス

`-http` オプションで `0.0.0.0` を指定することで、ホスト OS からアクセスできます。

```bash
# pprof
go tool pprof -http=0.0.0.0:9090 cpu.prof

# trace
go tool trace -http=0.0.0.0:9090 trace.out
```

Windows 側のブラウザから `http://localhost:9090` でアクセスできます。

### localhost でアクセスできない場合

WSL2 の IP アドレスを使用してください。

```bash
ip addr show eth0 | grep inet
# 出力例: inet 172.xx.xx.xx/20 ...
```

表示された IP アドレスを使用して `http://172.xx.xx.xx:9090` でアクセスしてください。

{{% /details %}}

---

## 2. pprof と runtime/trace の動作確認

### 2.1 サーバの起動

```bash
cd gwc-profiling/exercises
go mod tidy
go run main.go
```

**起動メッセージ:**
```
🔍 File Search HTTP Server
📍 http://localhost:8080 で起動中...
📊 pprof: http://localhost:8080/debug/pprof/
```

---

### 2.2 setup_check.go の実行とプロファイル取得

別のターミナルを開いて：

```bash
cd gwc-profiling/exercises/client
go run setup_check.go
```

このスクリプトは以下を実行します：
- サーバの動作確認（ヘルスチェック、検索エンドポイント、pprof エンドポイント）
- **CPU プロファイルの取得と保存** (`cpu.prof`) - プロファイリング中にサーバへ負荷をかけます
- **メモリプロファイルの取得と保存** (`heap.prof`)
- **runtime/trace の取得と保存** (`trace.out`)

**期待される出力:**
```
=== セットアップ確認 ===

⏳ サーバの起動を待機中...
✅ サーバが起動しました

[1/7] ヘルスチェック
  ✅ GET /health - OK
[2/7] 検索エンドポイント
  ✅ POST /search - OK
[3/7] pprof エンドポイント
  ✅ GET /debug/pprof/ - OK
  📊 ブラウザで確認: http://localhost:8080/debug/pprof/
[4/7] プロファイルエンドポイント
  ✅ CPU/メモリプロファイル - OK
[5/7] CPU プロファイル取得
  ⏳ CPU プロファイルを取得中... (5秒)
  💡 サーバに負荷をかけています...
  💡 負荷生成完了: 95 リクエスト送信
  ✅ cpu.prof を保存しました
  📊 確認コマンド: go tool pprof -http=:9090 cpu.prof
[6/7] メモリプロファイル取得
  ✅ heap.prof を保存しました
  📊 確認コマンド: go tool pprof -http=:9090 heap.prof
[7/7] トレース取得
  ⏳ トレースを取得中... (5秒)
  ✅ trace.out を保存しました
  📊 確認コマンド: go tool trace -http=:9090 trace.out

=== すべてのセットアップが完了しています 🎉 ===
```

すべてのチェックが ✅ となれば成功です。

---

### 2.3 Web UI での確認

`setup_check.go` の実行により、以下のファイルが保存されています：
- `cpu.prof` - CPU プロファイル
- `heap.prof` - メモリプロファイル
- `trace.out` - runtime/trace

これらのファイルを使って、Web UI で動作を確認します。

#### CPU プロファイルの確認

```bash
go tool pprof -http=:9090 cpu.prof
```

ブラウザで http://localhost:9090 にアクセスすると、pprofのグラフィカルUIが表示されます。

**主な機能:**
- **Graph**: 関数呼び出しの依存関係を視覚化
- **Flame Graph**: CPU 使用率を炎のように表示
- **Top**: CPU 使用率の高い関数をリスト表示
- **Source**: ソースコードと対応付けて表示

#### メモリプロファイルの確認

```bash
go tool pprof -http=:9090 heap.prof
```

ブラウザで http://localhost:9090 にアクセスすると、メモリ使用状況が表示されます。

#### トレースの確認

```bash
go tool trace -http=:9090 trace.out
```

ブラウザで http://localhost:9090 にアクセスすると、以下のようなメニューが表示されます：

**利用可能な分析:**
- **View trace**: タイムラインビューで詳細な実行フローを確認
- **Goroutine analysis**: ゴルーチンの動作を分析
- **Network blocking profile**: ネットワークブロッキングを分析
- **Synchronization blocking profile**: 同期ブロッキングを分析
- **Syscall blocking profile**: システムコールブロッキングを分析
- **Scheduler latency profile**: スケジューラの遅延を分析

各項目をクリックして、グラフや詳細情報が表示されれば成功です。

> [!TIP]
> WSL2 環境の場合は、`-http=0.0.0.0:9090` を使用してください（1.3 参照）。

---

### 2.4 UI スクリーンショット

以下は、pprof と runtime/trace の Web UI の表示例です。

#### pprof Web UI

##### Graph View

`go tool pprof -http=:9090 cpu.prof` を実行すると、以下のようなグラフビューが表示されます：

![pprof Graph View](../images/pprof-graph.png)

関数呼び出しの依存関係と、各関数の CPU 使用率が視覚化されます。

##### Flame Graph

CPU 使用率を視覚的に把握できる Flame Graph ビュー：

![pprof Flame Graph](../images/pprof-flame.png)

横幅が広いほど、その関数がCPU時間を多く消費していることを示します。

##### Top View

CPU 使用率の高い関数を一覧表示：

![pprof Top View](../images/pprof-top.png)

各関数の実行時間とその割合が確認できます。

---

#### runtime/trace Web UI

##### トップページ

`go tool trace -http=:9090 trace.out` を実行すると、以下のようなメニューが表示されます：

![trace Top Page](../images/trace-top.png)

##### Timeline View

プログラムの実行をタイムラインで視覚化：

![trace Timeline View](../images/trace-timeline.png)

各ゴルーチンの実行、待機、システムコールなどが時系列で表示されます。

##### Goroutine Analysis

ゴルーチンの実行状況を詳細に分析：

![trace Goroutine Analysis](../images/trace-goroutine.png)

ゴルーチンごとの実行時間や待機時間を確認できます。

---

## 3. 参考資料

### 公式ドキュメント

- [Diagnostics - The Go Programming Language](https://go.dev/doc/diagnostics)
- [pprof - runtime/pprof package](https://pkg.go.dev/runtime/pprof)
- [runtime/trace package](https://pkg.go.dev/runtime/trace)
- [Graphviz 公式サイト](https://graphviz.org/)

### チュートリアル・記事

- [Profiling Go Programs - The Go Blog](https://go.dev/blog/pprof)
- [Go Execution Tracer - The Go Blog](https://go.dev/blog/execution-tracer)
- [pprof User Guide](https://github.com/google/pprof/blob/main/doc/README.md)

### ツール

- [google/pprof - GitHub](https://github.com/google/pprof)
- [go tool pprof - Command Documentation](https://pkg.go.dev/cmd/pprof)
- [go tool trace - Command Documentation](https://pkg.go.dev/cmd/trace)

---

これでセットアップは完了です！ワークショップを開始する準備が整いました。
当日は会場でお会いしましょう！
