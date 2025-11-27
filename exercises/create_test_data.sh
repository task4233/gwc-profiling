#!/bin/bash

# テストデータ作成スクリプト
# 負荷テスト用に大量のGoファイルを生成します

echo "📝 テストデータ作成中..."

mkdir -p testdata/large

# Goファイルを30個作成（trace用に軽量化）
for i in {1..30}; do
  cat > testdata/large/file_${i}.go <<EOF
package testdata

import (
	"fmt"
	"strings"
	"regexp"
	"encoding/json"
)

// これは自動生成されたテストファイル ${i} です

type TestStruct${i} struct {
	ID   int
	Name string
	Data []byte
}

func NewTestStruct${i}(id int, name string) *TestStruct${i} {
	return &TestStruct${i}{
		ID:   id,
		Name: name,
		Data: make([]byte, 0),
	}
}

func (t *TestStruct${i}) ProcessData() error {
	// データ処理のシミュレーション
	pattern := "test.*pattern"
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("regexp compile error: %w", err)
	}

	data := strings.Repeat("test data ", 100)
	matches := re.FindAllString(data, -1)

	result, err := json.Marshal(matches)
	if err != nil {
		return err
	}

	t.Data = result
	return nil
}

func (t *TestStruct${i}) GetID() int {
	return t.ID
}

func (t *TestStruct${i}) GetName() string {
	return t.Name
}

func (t *TestStruct${i}) SetName(name string) {
	t.Name = name
}

type Interface${i} interface {
	ProcessData() error
	GetID() int
	GetName() string
	SetName(string)
}

func Helper${i}Function(input string) string {
	return strings.ToUpper(input)
}

func AnotherHelper${i}(data []byte) (string, error) {
	var result string
	err := json.Unmarshal(data, &result)
	return result, err
}

// ダミー関数を追加してファイルサイズを増やす
EOF

  # ダミー関数を追加（20個→5個に削減）
  for j in {1..5}; do
    cat >> testdata/large/file_${i}.go <<EOF

func DummyFunc${i}_${j}(param int) int {
	result := param * ${j}
	if result > 100 {
		return result / 2
	}
	return result
}
EOF
  done

done

echo "✅ テストデータ作成完了"
echo "   作成されたファイル: 30個"
echo "   場所: testdata/large/"

# ファイル数とサイズを表示
FILE_COUNT=$(ls testdata/large/*.go | wc -l)
TOTAL_SIZE=$(du -sh testdata/large/ | cut -f1)
echo "   合計サイズ: ${TOTAL_SIZE}"
