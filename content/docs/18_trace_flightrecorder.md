---
title: "Part 2-3: Flight Recorder"
weight: 180
---

## Flight Recorderとは

Go 1.22以降で利用可能な**Flight Recorder**は、トレースを常時バックグラウンドで記録し、問題発生時にスナップショットを取得できる機能です。

飛行機のフライトレコーダー（ブラックボックス）のように、常に記録し続けるため、**問題が発生した後でもトレースデータを取得できます**。

参考: [Go Blog - More powerful Go execution traces (2024)](https://go.dev/blog/execution-traces-2024)

---

## 従来の方法との比較

### 従来の方法（手動でトレース開始/終了）

```go
func main() {
    // 問題が発生する前にトレースを開始しておく必要がある
    f, _ := os.Create("trace.out")
    trace.Start(f)
    defer trace.Stop()

    doWork()  // ← 問題発生前にStart()していないと記録されない
}
```

**問題点**:
- 問題発生前にトレースを開始しておく必要がある
- 「あの時トレースを取っておけば...」という事態が発生

### Flight Recorder（常時記録）

```go
func main() {
    // Flight Recorder開始（常時バックグラウンドで記録）
    fr := trace.NewFlightRecorder()
    fr.Start()

    start := time.Now()
    doWork()

    // 遅い処理を検出したら、その場でスナップショットを取得！
    if time.Since(start) > threshold {
        var b bytes.Buffer
        fr.WriteTo(&b)  // ← 過去のトレースデータが取得できる！
        os.WriteFile("trace.out", b.Bytes(), 0o644)
    }
}
```

**利点**:
- 問題発生後にトレースを取得できる
- 常時記録されているため、再現困難な問題も捉えられる
- バッファは循環するため、メモリ使用量は一定

---

## 演習: Flight Recorderの実践

### 演習の目的

ランダムに遅延が発生するプログラムを題材に、Flight Recorderで遅延発生時のトレースを自動取得します。

演習ディレクトリ: `exercises/trace/flightrecorder/`

### プログラムの動作

- 5つのgoroutineを起動
- 各goroutineがランダムな時間（0-500ms）待機
- **300ms以上**かかる処理があれば、自動的にトレースを保存

---

## 演習手順

### ステップ1: Flight Recorderの実行

```bash
cd exercises/trace/flightrecorder/

# Flight Recorder付きで実行
go run main.go
```

出力例：
```
Starting Flight Recorder...
Goroutine 1: waiting 234ms
Goroutine 2: waiting 456ms
Goroutine 3: waiting 123ms
Goroutine 4: waiting 378ms
Goroutine 5: waiting 89ms

⚠️  Slow operation detected: 456ms
📝 Trace saved to: flightrecorder.out

⚠️  Slow operation detected: 378ms
📝 Trace saved to: flightrecorder.out
```

遅延が発生すると、自動的に`flightrecorder.out`が保存されます。

### ステップ2: トレースの分析

```bash
go tool trace flightrecorder.out
```

**View trace**で確認：
- 遅延が発生したgoroutineを特定
- 何が原因で遅れたかを調査

---

## Flight Recorder APIの使い方

### 基本的な使用方法

```go
import (
    "bytes"
    "os"
    "runtime/trace"
)

func main() {
    // Flight Recorderの作成と開始
    fr := trace.NewFlightRecorder()
    fr.Start()
    defer fr.Stop()

    // 処理を実行
    doWork()

    // 問題検出時にスナップショット取得
    if problemDetected() {
        var buf bytes.Buffer
        _, err := fr.WriteTo(&buf)
        if err != nil {
            panic(err)
        }

        // ファイルに保存
        os.WriteFile("trace.out", buf.Bytes(), 0o644)
    }
}
```

### HTTPエンドポイントでの利用

```go
import (
    "net/http"
    "runtime/trace"
)

var flightRecorder *trace.FlightRecorder

func main() {
    // Flight Recorder開始
    flightRecorder = trace.NewFlightRecorder()
    flightRecorder.Start()

    // HTTPエンドポイント
    http.HandleFunc("/debug/trace/snapshot", snapshotHandler)
    http.ListenAndServe(":6060", nil)
}

func snapshotHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Header().Set("Content-Disposition", "attachment; filename=trace.out")

    _, err := flightRecorder.WriteTo(w)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}
```

使用例：
```bash
# スナップショット取得
curl http://localhost:6060/debug/trace/snapshot > trace.out

# 分析
go tool trace trace.out
```

---

## 実践的な使用パターン

### パターン1: レイテンシ閾値での自動保存

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        if duration > 500*time.Millisecond {
            saveTraceSnapshot(fmt.Sprintf("slow_request_%d.out", start.Unix()))
        }
    }()

    // リクエスト処理
    process(r)
}
```

### パターン2: エラー発生時の自動保存

```go
func criticalOperation() error {
    err := doWork()
    if err != nil {
        // エラー発生時にトレースを保存
        saveTraceSnapshot(fmt.Sprintf("error_%d.out", time.Now().Unix()))
        return err
    }
    return nil
}
```

### パターン3: シグナルハンドラでの保存

```go
func main() {
    fr := trace.NewFlightRecorder()
    fr.Start()
    defer fr.Stop()

    // SIGUSR1でトレーススナップショットを保存
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGUSR1)

    go func() {
        for range sigCh {
            saveTraceSnapshot("signal_trace.out")
        }
    }()

    // アプリケーション実行
    runApp()
}
```

使用例：
```bash
# プロセスID確認
ps aux | grep myapp

# トレース取得
kill -USR1 <PID>
```

---

## Flight Recorderのベストプラクティス

### 1. 本番環境での使用

```go
// 本番環境でも常時有効化
func main() {
    if os.Getenv("ENABLE_FLIGHT_RECORDER") == "true" {
        fr := trace.NewFlightRecorder()
        fr.Start()
        defer fr.Stop()

        // エンドポイント提供
        http.HandleFunc("/debug/trace/snapshot", snapshotHandler)
    }

    // アプリケーション実行
}
```

### 2. 自動保存戦略

```go
// 遅いリクエストのトップN件だけ保存
type SlowRequestTracker struct {
    traces []TraceData
    mu     sync.Mutex
}

func (s *SlowRequestTracker) Add(duration time.Duration, trace []byte) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if len(s.traces) < 10 || duration > s.traces[len(s.traces)-1].Duration {
        // トップ10にランクイン
        s.traces = append(s.traces, TraceData{duration, trace})
        sort.Slice(s.traces, func(i, j int) bool {
            return s.traces[i].Duration > s.traces[j].Duration
        })
        if len(s.traces) > 10 {
            s.traces = s.traces[:10]
        }
    }
}
```

### 3. メモリ使用量の管理

Flight Recorderはバッファを循環させますが、保存頻度が高いとディスクを圧迫します：

```go
// 保存頻度を制限
var (
    lastSave time.Time
    saveMu   sync.Mutex
)

func saveTraceSnapshot(filename string) {
    saveMu.Lock()
    defer saveMu.Unlock()

    // 1分に1回まで
    if time.Since(lastSave) < 1*time.Minute {
        return
    }

    // 保存処理
    var buf bytes.Buffer
    flightRecorder.WriteTo(&buf)
    os.WriteFile(filename, buf.Bytes(), 0o644)

    lastSave = time.Now()
}
```

---

## トラブルシューティング

### トレースが空

**原因**: スナップショット取得前にStop()が呼ばれた

**解決**: Stop()を遅延させるか、deferで管理

### ファイルサイズが大きい

**原因**: バッファサイズが大きい

**解決**: バッファサイズは固定（ランタイムが管理）だが、保存頻度を制限

### パフォーマンス影響

**原因**: Flight Recorderのオーバーヘッド

**解決**: 本番環境では環境変数で有効/無効を切り替え

---

## まとめ

Flight Recorderを使うことで：

1. **問題発生後のトレース取得**: 再現困難な問題を捉える
2. **常時監視**: 本番環境で継続的にトレース
3. **自動保存**: 閾値やエラーで自動的にスナップショット

次は[ProfilingとTraceの比較]({{< relref "19_comparison.md" >}})でそれぞれの使い分けを学びます。
