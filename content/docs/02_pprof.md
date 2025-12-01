---
title: "Part 1: pprofでの解析"
weight: 30
---

> **⚠️ 重要な注意事項**
>
> このドキュメントに記載されている具体的な数値（CPU時間、メモリ割り当て量など）は、**参考例**です。以下の点にご注意ください：
>
> 1. **環境依存**: 実際の数値は、実行環境（CPU、メモリ、ディスク速度）、テストデータサイズ、負荷の強さによって大きく異なります
> 2. **サンプリング精度**: CPUプロファイリングはサンプリングベースのため、実行ごとに数値が変動します
> 3. **学習目的**: 重要なのは絶対値ではなく、**どの関数がボトルネックか**、**どのように分析するか**という手法です
>
> **推奨アプローチ**:
> - ドキュメントの数値は「このような結果が得られる可能性がある」という例として理解してください
> - 実際の環境で取得したプロファイルを使って、同様の分析手法を適用してください
> - メモリプロファイルのデータは比較的正確ですが、CPUプロファイルは環境によって大きく異なる場合があります

## pprofで見つける問題

pprofは**関数単位のリソース消費**を可視化するツールです。

**得意なこと**:
- CPU時間を消費している関数の特定
- メモリ割り当てが多い箇所の発見
- 関数呼び出しの統計情報

**苦手なこと**:
- ゴルーチン間の実行タイミング
- 同期処理のブロッキング時間の詳細な可視化（標準のCPU Profileでは不可。Block ProfileやMutex Profileを別途有効化する必要がある）
- GCの影響の詳細（トレースが必要）

> **補足**: pprofには複数のプロファイルタイプがあります
> - **CPU Profile**: CPU実行時間（デフォルト）
> - **Heap Profile**: メモリ割り当て
> - **Block Profile**: 同期プリミティブでのブロック時間（要有効化）
> - **Mutex Profile**: ミューテックス競合時間（要有効化）
> - **Goroutine Profile**: ゴルーチンのスタックトレース

---

## 演習1: CPUプロファイルの取得（10分）

### 1-1. サーバの起動

```bash
cd exercises

# HTTPサーバモード: CPUプロファイル付きでサーバを起動
go run http_server.go -cpuprofile=cpu.prof

# または、MCPサーバモード（main.go）でも可能:
# go run main.go -cpuprofile=cpu.prof
```

**注意**: このドキュメントの解析結果は `http_server.go` を使用した場合のものです。

### 1-2. 負荷をかける

別のターミナルで：

```bash
cd exercises/client
go run test_client.go
```

**出力例**:
```
🚀 MCP負荷テスト開始
   同時接続数: 10
   総リクエスト数: 500
...
✅ 負荷テスト完了
   成功: 500/500
   所要時間: 15s
   スループット: 33.33 req/sec
```

### 1-3. サーバの停止

サーバのターミナルで `Ctrl+C` を押すと、`cpu.prof` が保存されます。

```
✅ CPUプロファイル保存完了
```

---

## 演習2: CPUプロファイルの解析（10分）

### 2-1. Web UIで可視化

```bash
go tool pprof -http=:8080 cpu.prof
```

ブラウザで http://localhost:8080 を開きます。

### 2-2. 観察ポイント

#### Flame Graph（フレームグラフ）を確認

pprof Web UIでは、以下のようなビューが利用できます：

**主要なビュー**:
- **Top**: 関数ごとのCPU時間ランキング
- **Graph**: 関数呼び出しグラフ（呼び出し関係を矢印で表示）
- **Flame Graph**: フレームグラフ（横幅＝CPU時間）
- **Source**: ソースコード上での時間分布

![pprof Web UIのビュー選択](https://go.dev/blog/pprof/pprof-makecalls.png)
*pprofのGraph表示例（[Go公式ブログより](https://go.dev/blog/pprof)）*

**質問**: どの関数が最もCPU時間を消費していますか？

{{% details "ヒント" %}}
上部の View から "Flame Graph" を選択してください。
横幅が広い関数ほどCPU時間を消費しています。

**Flame Graphの読み方**:
- **横軸（幅）**: その関数が消費したCPU時間の割合
- **縦軸（高さ）**: 呼び出しスタックの深さ
- **色**: 関数を区別するためのもの（意味はない）
- クリックでその関数にズームイン可能
{{% /details %}}

{{% details "基本: 発見すべきこと" %}}
**注意**: 以下の数値は参考例です。実際の環境では大きく異なる可能性があります。

**最もCPU時間を消費している関数（例）:**
- **`main.searchFile`** - 全体の **84.37%（452.19秒）** ← 参考値

**主要なボトルネック（例）:**
- ファイルI/O: 約209秒 ← 参考値
- 正規表現処理: 約140秒 ← 参考値
- ロック競合: 約92秒 ← 参考値

**実際の環境では**:
- 負荷が軽い場合、総サンプル時間が数秒以下になることがあります
- その場合でも、**相対的な割合**（どの関数が何%を占めているか）が重要です
- `main.searchFile`が最大のボトルネックであることは変わりません
{{% /details %}}

{{% details "詳細: 関数ごとの内訳" %}}
**注意**: 以下の数値は高負荷環境での参考例です。実際の環境では大きく異なります。

**searchFile内で呼ばれている関数の詳細（参考例）:**

| 処理 | CPU時間（参考） | 割合 | 説明 |
|------|---------|------|------|
| `os.ReadFile` | 209.37秒 | 39.07% | ファイル全体の読み込み |
| `regexp.(*Regexp).tryBacktrack` | 125.19秒 | 23.36% | 正規表現のバックトラック処理 |
| `re.MatchString` | 140.53秒 | (累積) | 正規表現マッチング全体 |
| mutex lock/unlock | 約92秒 | 17% | 並列処理での競合 |
| `regexp.Compile` | 8.82秒 | 1.65% | 正規表現のコンパイル |

**重要な発見:**
- `regexp.Compile` がファイルごとに呼ばれている（同じパターンを毎回再コンパイル）
- 正規表現のバックトラック処理が予想以上に重い
- 複数ゴルーチンがグローバルロックで競合している
{{% /details %}}

#### Top関数を確認

View → Top を選択

**注意**: 以下は参考例です。実際の出力は環境によって異なります。

```
【参考例】
Showing nodes accounting for 504.51s, 94.14% of 535.93s total
      flat  flat%   sum%        cum   cum%
   236.14s 44.06% 44.06%    236.26s 44.08%  syscall.syscall
    75.43s 14.07% 58.14%    125.19s 23.36%  regexp.(*Regexp).tryBacktrack
    74.77s 13.95% 72.09%     74.77s 13.95%  runtime.usleep
    31.18s  5.82% 77.91%     31.40s  5.86%  regexp.(*bitState).shouldVisit
     ...
     0.40s 0.075% 93.60%    452.19s 84.37%  main.searchFile
     ...
     0.04s 0.0075% 94.06%      8.82s  1.65%  regexp.compile
     ...
         0     0% 94.14%    209.37s 39.07%  os.ReadFile
```

**質問**: `flat` と `cum` の違いは何ですか？

{{% details "ヒント" %}}
- `flat`: その関数自体で消費した時間
- `cum`: その関数と呼び出した関数を含む累積時間
{{% /details %}}

{{% details "実際の解析結果" %}}
**`main.searchFile` の例:**
- `flat`: 0.40s → searchFile自体のコード実行時間
- `cum`: 452.19s → searchFileが呼び出した全ての関数を含む時間（全体の84.37%！）

この差分 (452.19s - 0.40s) が、searchFile内から呼び出された関数の時間です。

**主要なボトルネック:**
1. `os.ReadFile`: 209.37s (39.07%) - ファイル全体の読み込み
2. `regexp.(*Regexp).tryBacktrack`: 125.19s (23.36%) - 正規表現のバックトラック
3. mutex lock/unlock: 約92s (17%) - グローバルロックでの競合
4. `regexp.compile`: 8.82s (1.65%) - 毎回の正規表現コンパイル
{{% /details %}}

---

## 演習3: メモリプロファイルの取得（10分）

### 3-1. サーバの起動

```bash
go run http_server.go -memprofile=mem.prof
```

### 3-2. 負荷をかける

```bash
cd client
go run test_client.go
```

### 3-3. サーバの停止

`Ctrl+C` で停止すると `mem.prof` が保存されます。

---

## 演習4: メモリプロファイルの解析（10分）

### 4-1. Web UIで可視化

```bash
go tool pprof -http=:8080 mem.prof
```

### 4-2. 観察ポイント

#### Alloc Space（割り当て量）を確認

上部の "SAMPLE" から "alloc_space" を選択

**メモリプロファイルの主要メトリクス**:

| メトリクス | 意味 | 用途 |
|----------|------|------|
| **alloc_space** | 累積割り当て量 | 「どこで大量にメモリを割り当てているか」を発見 |
| **alloc_objects** | 累積オブジェクト数 | 「どこで頻繁に割り当てが発生しているか」を発見 |
| **inuse_space** | 現在使用中のメモリ | 「どこがメモリを保持しているか」を発見 |
| **inuse_objects** | 現在存在するオブジェクト数 | メモリリーク調査 |

```
累積割り当て量（alloc_space）が大きい
        ↓
GCが頻繁に動作する可能性
        ↓
CPU時間の消費増加
```

**質問**: どの関数が最もメモリを割り当てていますか？

{{% details "基本: 発見すべきこと" %}}
**最もメモリを割り当てている関数:**
- **`main.searchFile`** - **51.48GB（全体の90.28%）**

**主要な割り当て箇所:**
- 正規表現コンパイル: 約16GB
- 文字列分割: 約16GB
- ファイル読み込み: 約9GB
- 結果スライス: 約9GB
{{% /details %}}

{{% details "詳細: メモリ割り当ての内訳" %}}
**searchFile内での割り当て詳細:**

| 処理 | メモリ割り当て | 割合 | 説明 |
|------|---------------|------|------|
| `regexp.Compile` | 16.28GB | 28.54% | 正規表現コンパイル |
| `strings.Split` | 16.23GB | - | 行分割（string変換8.20GB + スライス8.03GB） |
| `allResults = append()` | 9.03GB | - | 結果スライスへの追加 |
| `os.ReadFile` | 8.91GB | 15.62% | ファイル全体読み込み |

**その他の主要な割り当て（内部処理）:**
- `regexp/syntax.(*compiler).inst`: 11.75GB (20.12%)
- `os.readFileContents`: 8.42GB (14.42%)
- `strings.genSplit`: 8.23GB (14.09%)

**重要な発見:**
- ファイル全体を一度にメモリに読み込んでいる
- `[]byte` → `string` 変換でメモリコピーが発生
- 正規表現コンパイルが予想以上にメモリを消費
- 結果スライスの容量拡張で再割り当てが発生
{{% /details %}}

#### Inuse Space（使用中のメモリ）を確認

SAMPLE → "inuse_space" を選択

**質問**: 使用中のメモリと割り当て量の違いは何ですか？

{{% details "ヒント" %}}
- alloc_space: 累積の割り当て量（GCで回収されたものも含む）
- inuse_space: 現在使用中のメモリ
- 差分が大きい = GCが頻繁に発生している可能性
{{% /details %}}

{{% details "実際の解析結果" %}}
**比較結果:**

| メトリクス | 値 | 意味 |
|----------|-----|------|
| alloc_space（累積割り当て量） | **57.02GB** | プログラム実行中に割り当てられた全メモリ |
| inuse_space（使用中のメモリ） | **11.74MB** | プロファイル取得時点で実際に使用中 |
| **差分（GCで回収）** | **約57GB** | GCで回収されたメモリ量 |

**`main.searchFile`の詳細:**

| 行 | 処理 | alloc_space | inuse_space | GC回収量 |
|----|------|-------------|-------------|----------|
| 174 | regexp.Compile | 16.28GB | 0 | 16.28GB |
| 180 | os.ReadFile | 8.91GB | 0 | 8.91GB |
| 186 | strings.Split | 16.23GB | 1.51MB | 約16.23GB |
| 198 | append(allResults) | 9.03GB | 708KB | 約9.03GB |

**考察:**
```
GC回収率 = (57.02GB - 11.74MB) / 57.02GB ≈ 99.98%
メモリ効率 = 11.74MB / 57.02GB ≈ 0.02%
```

**これが意味すること:**
- ほぼすべてのメモリ（99.98%）が短命オブジェクト
- 実効的に使われているのはわずか0.02%のみ
- GCが非常に頻繁に動作 → CPU時間も消費
- 大量の割り当て/回収サイクルがパフォーマンスに悪影響

**改善の方向性:**
- ストリーミング処理でメモリ割り当てを削減
- 正規表現の再利用でコンパイルコストを削減
- スライス容量の事前確保で再割り当てを防ぐ
{{% /details %}}

---

## 発見したボトルネック（まとめ）

**注意**: 以下の具体的な数値は参考例です。実際の環境では異なります。重要なのは、**どの種類の処理がボトルネックになっているか**を理解することです。

### CPU問題（参考例）

1. **正規表現のバックトラック処理**
   - 関数: `regexp.(*Regexp).tryBacktrack`
   - 影響: **125.19秒（23.36%）** - 最大のCPU消費
   - 原因: 複雑な正規表現パターンでのマッチング処理
   - 場所: `http_server.go:189` (`re.MatchString(line)`)
   - **累積**: 正規表現マッチング全体で **140.53秒**

2. **正規表現の毎回コンパイル**
   - 場所: `http_server.go:174` (`searchFile`関数)
   - 影響: **8.82秒（1.65%）** - ファイル数分のコンパイルコスト
   - コード:
   ```go
   re, err := regexp.Compile(pattern)  // 毎回コンパイル！
   ```

3. **グローバルロックでの競合**
   - 場所: `http_server.go:197-199`
   - 影響: **約92秒（17%）** - 並列処理のボトルネック
   - コード:
   ```go
   resultsMu.Lock()           // 17.35秒（Lock取得までの待ち時間）
   allResults = append(...)   // 1.14秒（実際の処理）
   resultsMu.Unlock()         // 73.90秒（全ゴルーチンでのunlock処理合計）
   ```
   - 説明: 複数のゴルーチンが同時にロック取得を試みるため、待ち時間が累積
   - 改善策: チャネルを使った結果収集、またはローカルバッファでバッチ処理

### メモリ問題

1. **正規表現の毎回コンパイル**
   - 場所: `http_server.go:174`
   - 影響: **16.28GB（28.54%）** - 最大のメモリ割り当て
   - コード:
   ```go
   re, err := regexp.Compile(pattern)  // 全ファイル処理で累積16.28GB割り当て
   ```
   - 改善策: 正規表現を事前コンパイルして再利用

2. **文字列の分割とコピー**
   - 場所: `http_server.go:186`
   - 影響: **16.23GB** - 行分割での大量割り当て
   - コード:
   ```go
   lines := strings.Split(string(content), "\n")
   // string(content): 8.20GB ([]byte → string変換)
   // strings.Split: 8.03GB (行スライス作成)
   ```
   - 改善策: ストリーミング処理（`bufio.Scanner`）への変更

3. **結果スライスへの追加**
   - 場所: `http_server.go:198`
   - 影響: **9.03GB** - スライス容量拡張による再割り当て
   - コード:
   ```go
   allResults = append(allResults, match)  // 累積9.03GB（容量不足時に再割り当て）
   ```
   - 改善策: 事前容量確保またはローカルバッファ使用

4. **ファイル全体の読み込み**
   - 場所: `http_server.go:180`
   - 影響: **8.91GB（15.62%）** + **209.37秒（39.07%のCPU時間）**
   - コード:
   ```go
   content, err := os.ReadFile(filePath)  // 全部読む！
   ```
   - 改善策: ストリーミング処理（`bufio.Scanner`）への変更

**メモリ効率の深刻な問題:**
- **累積割り当て**: 57.02GB
- **使用中メモリ**: 11.74MB
- **GC回収率**: 99.98% → ほぼすべてのメモリが無駄な一時割り当て
- **実効効率**: 0.02% → メモリ使用が極めて非効率

### 全体的な考察

**重要**: 以下の数値は高負荷環境での参考例です。実際の環境では大きく異なる可能性があります。

**パフォーマンス特性（参考例）:**
- **実行時間**: 127.87秒 ← 参考値
- **CPU時間**: 535.93秒（419.11% = 約4コア並列利用） ← 参考値
- **メモリ割り当て**: 57.02GB（実使用は11.74MB、GC回収率99.98%） ← **この数値は実環境でも確認できます**
- `main.searchFile`: **CPU時間の84.37%、メモリ割り当ての90.28%を消費** ← 割合は比較的安定

**CPU vs メモリの関係:**
- 正規表現処理: CPU 125秒（バックトラック）+ メモリ 16.28GB（コンパイル）
- mutex競合: CPU 92秒（ロック待ち）+ メモリ 9.03GB（スライス拡張）
- ファイルI/O: CPU 209秒（読み込み時間）+ メモリ 8.91GB（バッファ）
- GCオーバーヘッド: 57GBの割り当て/回収による追加のCPU消費

**最適化の優先順位:**

**注意**: この優先順位は**pprofのデータのみ**に基づいています。runtime/traceのデータを見ると、優先順位が大きく変わります（下表参照）。

| pprofでの<br>優先度 | 改善項目 | CPU削減<br>（推定） | メモリ削減 | trace.out検証後の<br>実際の優先度 |
|---------------------|----------|---------------------|------------|----------------------------------|
| （なし） | **ワーカープール導入** | - | - | **🔴 緊急** ← 74万ゴルーチン |
| 中 | **mutex競合の削減** | 92秒 | - | **🔴 緊急** ← 実際は7.6時間のブロック |
| 最高 | ストリーミング処理への変更 | ※ | 25.14GB | **🟠 最高** ← GC頻度劇的改善 |
| 最高 | 正規表現の事前コンパイル | 8.82秒 | 16.28GB | **🟡 高** ← コンパイルコスト削減 |
| 高 | 結果スライス容量事前確保 | - | 9.03GB | **🟢 中** ← GC負荷削減 |
| 検討 | 正規表現パターン簡略化 | 最大125秒 | - | **🔵 検討** ← バックトラック削減 |

※ ストリーミング処理: `os.ReadFile`(8.91GB) + `strings.Split`(16.23GB) = 25.14GB削減。
  CPU時間は209秒すべてが削減されるわけではなく、ファイルI/O自体は残る。

**期待される総合効果:**
- **CPU時間**: 直接削減 8.82秒 + GC削減による間接効果
- **メモリ割り当て**: 50GB以上削減（88%改善） → GC頻度の大幅な低減
- **実行時間**: GC削減とmutex競合改善により、大幅な短縮が期待される

**注意**:
- CPU削減の数値は単純合算できない（重複やGC削減効果を含む）
- 最大の効果はメモリ効率改善によるGC負荷の削減

---

## Block Profile と Mutex Profile の使用方法

### 標準CPU Profileの限界

標準のCPU Profileは**CPU実行時間のみ**を測定するため、「待っている時間」は計測されません。

しかし、pprofには**ブロッキング時間を測定する専用のプロファイルタイプ**があります：

#### Block Profile: 同期プリミティブでのブロック時間

```go
package main

import (
    "os"
    "runtime"
    "runtime/pprof"
)

func main() {
    // ブロックプロファイルを有効化
    // 引数: 1 = すべてのブロックイベントを記録
    runtime.SetBlockProfileRate(1)

    // アプリケーションコード
    runApp()

    // プロファイルを保存
    f, _ := os.Create("block.prof")
    defer f.Close()
    pprof.Lookup("block").WriteTo(f, 0)
}
```

```bash
# 解析
go tool pprof block.prof
(pprof) top
# Mutex, Channel, WaitGroupでのブロック時間が表示される
```

**注意**: 本番環境では `runtime.SetBlockProfileRate(100)` など低い値を設定（オーバーヘッド削減）

#### Mutex Profile: ミューテックス競合時間

```go
package main

import (
    "os"
    "runtime"
    "runtime/pprof"
)

func main() {
    // ミューテックスプロファイルを有効化
    // 引数: 1 = 100%サンプリング
    runtime.SetMutexProfileFraction(1)

    // アプリケーションコード
    runApp()

    // プロファイルを保存
    f, _ := os.Create("mutex.prof")
    defer f.Close()
    pprof.Lookup("mutex").WriteTo(f, 0)
}
```

```bash
# 解析
go tool pprof mutex.prof
(pprof) top
# Unlock時に記録された競合時間が表示される
```

**重要**: Mutex Profileは**Unlock時**に記録されます。表示される時間は「他のゴルーチンがこのミューテックスを待っていた累積時間」です。

### HTTP経由でのプロファイル取得

```go
import _ "net/http/pprof"

func main() {
    // Block/Mutex Profileを有効化
    runtime.SetBlockProfileRate(1)
    runtime.SetMutexProfileFraction(1)

    // HTTPサーバ起動（pprofエンドポイント自動追加）
    go http.ListenAndServe(":6060", nil)

    // アプリケーションコード
}
```

```bash
# Block Profileを取得
curl -o block.prof http://localhost:6060/debug/pprof/block

# Mutex Profileを取得
curl -o mutex.prof http://localhost:6060/debug/pprof/mutex

# 解析
go tool pprof -http=:8080 block.prof
go tool pprof -http=:8080 mutex.prof
```

---

## pprofで見えないもの（重要な警告）

以下は**標準のCPU/Heap Profileでは発見が難しい**が、**パフォーマンスに致命的な影響を与える可能性がある**問題です：

### ❌ **ゴルーチンの過剰生成**
- CPU/Heap Profileでは検出できない
- **runtime/traceでは**: 74万個のゴルーチンを検出
- **影響**: ゴルーチン生成/破棄のオーバーヘッド、スケジューリングコスト、メモリフットプリント増大

### ⚠️ **ミューテックスでの待ち時間（標準CPU Profileの場合）**
- **標準CPU Profileの表示**: mutex関連のCPU時間は約92秒
- **実際の影響**: ブロック時間は**7.6時間**（Block/Mutex ProfileまたはTraceで判明）
- **説明**: 標準のCPU ProfileはCPU実行時間のみを測定します。ブロック時間を見るには：
  - **Block Profile** (`runtime.SetBlockProfileRate()`) を有効化、または
  - **Mutex Profile** (`runtime.SetMutexProfileFraction()`) を有効化、または
  - **runtime/trace** を使用
- **結果**: CPU Profileのみでmutexの優先度を判断すると、重大な問題を見逃す危険性

### ❌ **GCの実際の影響**
- pprofではメモリ割り当て量は分かるが、GCの発生頻度や停止時間は不明
- **runtime/traceでは**: 数百ミリ秒に1回の頻繁なGC、Stop The Worldの影響を可視化

### ⚠️ **結論: pprofとtraceの併用が必須**

pprofだけで最適化の優先順位を決めると、**重大な問題を見逃す**可能性があります：

- **誤った優先順位の例**:
  - pprof: mutex競合は「中」優先度（92秒のCPU時間）
  - trace: mutex競合は「緊急」（7.6時間のブロック時間）

- **見逃す問題の例**:
  - 74万個のゴルーチン生成（最優先で解決すべき）
  - GCによるStop The Worldの頻繁な発生

**次のステップ**: [Part 2: runtime/trace](../03_trace) で、pprofでは見えなかった並行処理の問題を発見します。

---

## 気づきの共有（5分）

グループで以下を共有してください：

1. pprofで発見した問題は何でしたか？
2. Flame GraphとTopビューの使い分けは？
3. 予想外だった発見はありますか？

---

## 参考資料

### 公式ドキュメント

- [Profiling Go Programs - Go公式ブログ](https://go.dev/blog/pprof)
- [Diagnostics - Go公式ドキュメント](https://go.dev/doc/diagnostics)
- [runtime/pprof パッケージ](https://pkg.go.dev/runtime/pprof)
- [net/http/pprof パッケージ](https://pkg.go.dev/net/http/pprof)

### プロファイリング手法

- [Profiler labels in Go - rakyll.org](https://rakyll.org/profiler-labels/)
- [Mutex Profiling - rakyll.org](https://rakyll.org/mutexprofile/)
- [DataDog: Go Profiler Notes - Block Profile](https://github.com/DataDog/go-profiler-notes/blob/main/block.md)
- [DataDog: Go Profiler Notes - CPU Profile](https://datadoghq.dev/go-profiler-notes/profiling/cpu-profiler.html)

### GopherCon トーク

- [GopherCon 2019: Dave Cheney - Two Go Programs, Three Different Profiling Techniques](https://www.youtube.com/watch?v=nok0aYiGiYA)
- [GopherCon 2021: Felix Geisendörfer - The Busy Developer's Guide to Go Profiling, Tracing and Observability](https://www.youtube.com/watch?v=7hJz_WOx8JU)

---

## 次のステップ

[Part 2: runtime/traceでの解析](../03_trace) に進み、pprofでは見えなかった並行処理の問題を発見します。

また、より高度なテクニックについては [高度なプロファイリングテクニック](../06_advanced_techniques) を参照してください。
