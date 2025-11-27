#!/bin/bash

# シンプルな負荷テストスクリプト
# サーバを一度だけ起動して、複数のリクエストを送る

echo "📊 サーバを起動します（CPUプロファイル有効）..."
go run main.go -cpuprofile=cpu.prof &
SERVER_PID=$!

# サーバの起動を待つ
sleep 2

echo "🔄 負荷テストを実行中..."

# 100回のリクエストを送る
for i in {1..100}; do
  echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"pattern":"func.*","paths":[".."],"max_results":50}}}' | go run main.go 2>/dev/null &

  if [ $((i % 10)) -eq 0 ]; then
    echo "  $i/100 リクエスト完了"
    wait
  fi
done

wait

echo "✅ 負荷テスト完了"
echo "⏹️  サーバを停止します..."
kill -SIGINT $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "📈 プロファイルを確認してください："
echo "   go tool pprof -http=:8080 cpu.prof"
