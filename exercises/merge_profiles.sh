#!/bin/bash

# 複数のCPUプロファイルをマージするスクリプト

echo "📊 プロファイルファイルを検索中..."
PROFILES=$(ls cpu_*.prof 2>/dev/null | tr '\n' ' ')

if [ -z "$PROFILES" ]; then
    echo "❌ cpu_*.prof ファイルが見つかりませんでした"
    exit 1
fi

PROFILE_COUNT=$(echo $PROFILES | wc -w | tr -d ' ')
echo "✅ ${PROFILE_COUNT}個のプロファイルファイルを発見しました"

echo "🔄 プロファイルをマージ中..."

# go tool pprofでマージ（複数ファイルを指定すると自動的にマージされる）
go tool pprof -proto $PROFILES > cpu_merged.prof

if [ $? -eq 0 ]; then
    echo "✅ マージ完了: cpu_merged.prof"
    echo ""
    echo "📈 プロファイルを表示するには："
    echo "   go tool pprof -http=:8080 cpu_merged.prof"
    echo ""
    echo "🧹 個別のプロファイルファイルを削除するには："
    echo "   rm cpu_*.prof"
else
    echo "❌ マージに失敗しました"
    exit 1
fi
