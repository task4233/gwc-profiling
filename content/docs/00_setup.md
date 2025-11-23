---
title: "セットアップガイド"
weight: 10
---

本ドキュメントでは、ワークショップに参加するために必要な環境構築手順を説明します。

## セットアップ完了の判定

以下のスクリプトを実行し、すべてのチェックが通れば準備完了です。

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

## 動作環境

- macOS
- Linux
- Windows（WSL2 環境）

> [!WARNING]
> Windows ユーザーは環境差異を避けるため、WSL2 上の Linux 環境を使用してください。

---

## インストール手順

以下は各ツールのインストール手順です。すでにインストール済みの場合はスキップしてください。

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

## WSL2 環境での注意事項

WSL2 環境では、pprof や trace の Web UI をホスト OS（Windows）のブラウザで閲覧する必要があります。

{{% details "WSL2 での Web UI アクセス方法" %}}

### pprof / trace Web UI へのアクセス

`-http` オプションで `0.0.0.0` を指定することで、ホスト OS からアクセスできます。

```bash
# pprof
go tool pprof -http=0.0.0.0:8080 profile.pb.gz

# trace
go tool trace -http=0.0.0.0:8080 trace.out
```

Windows 側のブラウザから `http://localhost:8080` でアクセスできます。

### localhost でアクセスできない場合

WSL2 の IP アドレスを使用してください。

```bash
ip addr show eth0 | grep inet
# 出力例: inet 172.xx.xx.xx/20 ...
```

表示された IP アドレスを使用して `http://172.xx.xx.xx:8080` でアクセスしてください。

{{% /details %}}
