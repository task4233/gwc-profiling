# Go プロファイリングワークショップ

[![Deploy to Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/task4233/gwc-profiling)

> [Go Conference 2025](https://gwc.gocon.jp/2025/workshops/10_understanding_profiling/) ワークショップ用リポジトリ

## 概要

Go アプリケーションのプロファイリングには、一般的に pprof が利用されます。
しかし、pprof だけでは十分でないユースケースがあることをご存知でしょうか？

本ワークショップでは、pprof と runtime/trace の特性を理解することを目的とし、小さなアプリケーションコードをベースとした実践と気づきの言語化を通して、参加者全体で学びを深めていただく構成を想定しています。

## 事前準備

ワークショップに参加する前に、[セットアップガイド](docs/00_setup.md) に従って環境を構築してください。

```bash
# リポジトリのクローン
git clone https://github.com/task4233/gwc-profiling.git
cd gwc-profiling

# セットアップ確認
./scripts/doctor.sh
```

すべてのチェックが通れば準備完了です。
