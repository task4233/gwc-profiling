---
title: "はじめに"
weight: 1
bookFlatSection: true
---

# Go プロファイリングワークショップ

> [!NOTE]
> 本資料は [Go Conference 2025](https://gwc.gocon.jp/2025/workshops/10_understanding_profiling/) のワークショップ用に作成されました。

Go アプリケーションのプロファイリングには、一般的に pprof が利用されます。
しかし、pprof だけでは十分でないユースケースがあることをご存知でしょうか？

本ワークショップでは、pprof と runtime/trace の特性を理解することを目的とし、小さなアプリケーションコードをベースとした実践と気づきの言語化を通して、参加者全体で学びを深めていただく構成を想定しています。

## 対象者

本ワークショップは **Go の中〜上級者** を対象としています。

- Go で Web アプリケーションや CLI ツールを開発した経験がある方
- パフォーマンス改善に興味があり、プロファイリングの基礎を学びたい方
- pprof を使ったことはあるが、より深く理解したい方

## 必要要件

| ツール | バージョン | 確認コマンド |
|--------|-----------|-------------|
| Git | 任意 | `git --version` |
| Go | 1.25 以上 | `go version` |
| Graphviz | 任意 | `dot -V` |

## セットアップ確認

以下のコマンドを実行し、すべてのチェックが通れば準備完了です。

```bash
git clone https://github.com/task4233/gwc-profiling.git
cd gwc-profiling
./scripts/doctor.sh
```

詳細なインストール手順が必要な場合は、サイドバーの「セットアップガイド」を参照してください。

## ワークショップの内容

1. **プロファイリングツールの導入と計測**
   - ボトルネックを含むデモアプリケーションへの pprof / runtime/trace の導入
2. **知見の言語化と共有**
   - pprof と runtime/trace を導入して得られた知見の言語化と、参加者間での共有
3. **高度なテクニック（時間が余った場合）**
   - Profile-Guided Optimization (PGO)
   - Flight Recorder（**Go 1.25.0以降**の新機能）
   - benchstatによる統計的な効果測定

---

## 参考資料

### 公式ドキュメント
- [Go Diagnostics](https://go.dev/doc/diagnostics) - プロファイリングツールの総合ガイド
- [Profiling Go Programs](https://go.dev/blog/pprof) - pprof の基本
- [Execution Tracer](https://go.dev/blog/execution-tracer) - runtime/trace の基本

### 新機能（Go 1.21+）
- [Profile-Guided Optimization](https://go.dev/doc/pgo) - PGOの公式ガイド
- [Flight Recorder in Go 1.25](https://go.dev/blog/flight-recorder) - 本番診断の新手法
- [More powerful Go execution traces](https://go.dev/blog/execution-traces-2024) - トレース機能の改善

### 学習リソース
- [Go Optimization Guide](https://goperf.dev/) - 包括的な最適化ガイド
- [GopherCon Talks](https://www.youtube.com/@GopherConEurope) - プロファイリング関連のトーク多数
