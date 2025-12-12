// heap プロファイリングデモ（問題版）
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

var memprofile = flag.String("memprofile", "", "write memory profile to file")

type Record struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Active  bool    `json:"active"`
	Balance float64 `json:"balance"`
}

func main() {
	flag.Parse()

	// データ処理を実行
	records := processData(100000)
	fmt.Printf("Processed %d records\n", len(records))

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // 最新の統計を取得
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

// 問題 1: スライスの容量を事前確保していない
func processData(n int) []Record {
	var results []Record // 容量が指定されていない

	for i := 0; i < n; i++ {
		record := createRecord(i)
		results = append(results, record) // 何度も再アロケーション
	}

	return results
}

func createRecord(id int) Record {
	return Record{
		ID:      id,
		Name:    fmt.Sprintf("User%d", id),
		Email:   fmt.Sprintf("user%d@example.com", id),
		Active:  id%2 == 0,
		Balance: float64(id) * 100.5,
	}
}

// 問題 2: 毎回新しいバッファを作成
func serializeRecord(record Record) (string, error) {
	// 問題: 毎回新しい bytes.Buffer を作成
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)

	if err := encoder.Encode(record); err != nil {
		return "", err
	}

	// 問題: []byte から string への変換でコピーが発生
	return string(buf.Bytes()), nil
}

// 問題 3: 不要な string から []byte への変換
func deserializeRecord(data string) (*Record, error) {
	// 問題: string から []byte への変換でコピーが発生
	var record Record
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, err
	}
	return &record, nil
}
