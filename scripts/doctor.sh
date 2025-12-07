#!/bin/bash

set -eu

echo "=== セットアップ確認 ==="
echo ""

has_error=false

# Git のチェック
echo -n "[Git] "
if command -v git &> /dev/null; then
    version=$(git --version | awk '{print $3}')
    echo "✓ インストール済み ($version)"
else
    echo "✗ 未インストール → セクション 1 を参照"
    has_error=true
fi

# Go のチェック
echo -n "[Go] "
if command -v go &> /dev/null; then
    version=$(go version | awk '{print $3}')
    echo "✓ インストール済み ($version)"
    # バージョンチェック (1.25.4 以上)
    if [[ "$version" < "go1.25.4" ]]; then
        echo "  ⚠ Go 1.25.4 以上が必要です → セクション 3 を参照"
        has_error=true
    fi
else
    echo "✗ 未インストール → セクション 3 を参照"
    has_error=true
fi

# Graphviz のチェック
echo -n "[Graphviz] "
if command -v dot &> /dev/null; then
    version=$(dot -V 2>&1 | awk '{print $5}')
    echo "✓ インストール済み ($version)"
else
    echo "✗ 未インストール → セクション 4 を参照"
    has_error=true
fi

echo ""

if [ "$has_error" = true ]; then
    echo "=== 未完了の項目があります ==="
    echo "docs/00_setup.md の該当セクションを参照してセットアップを完了してください。"
    exit 1
else
    echo "=== すべてのセットアップが完了しています 🎉 ==="
    exit 0
fi
