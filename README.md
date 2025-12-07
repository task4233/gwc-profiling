# Go プロファイリングワークショップ

> [Go Conference 2025](https://gwc.gocon.jp/2025/workshops/10_understanding_profiling/) ワークショップ用リポジトリ

## 概要

Go アプリケーションのプロファイリングには、一般的に pprof が利用されます。
しかし、pprof だけでは十分でないユースケースがあることをご存知でしょうか？

本ワークショップでは、pprof と runtime/trace の特性を理解することを目的とし、小さなアプリケーションコードをベースとした実践と気づきの言語化を通して、参加者全体で学びを深めていただく構成を想定しています。

## 事前準備

ワークショップに参加する前に、[セットアップガイド](content/docs/00_setup.md) に従って環境を構築してください。

```bash
# リポジトリのクローン
git clone https://github.com/task4233/gwc-profiling.git
cd gwc-profiling

# セットアップ確認
./scripts/doctor.sh
```

すべてのチェックが通れば準備完了です。

## ローカルでの起動

ワークショップドキュメントをローカルで確認する場合は、まずsubmoduleを初期化してから、Hugoサーバーを起動してください。

```bash
# submoduleの初期化（初回のみ）
git submodule update --init --recursive

# Hugoサーバーの起動
hugo server
```

ブラウザで `http://localhost:1313` にアクセスすると、ドキュメントを閲覧できます。

ファイルを編集すると、自動的にブラウザに反映されます。
