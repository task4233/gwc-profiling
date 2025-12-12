// トレース基本デモ（問題版）
// ゴルーチンの挙動と GC を可視化
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/trace"
	"sync"
	"time"
)

var tracefile = flag.String("trace", "", "write trace to file")

func main() {
	flag.Parse()

	if *tracefile != "" {
		f, err := os.Create(*tracefile)
		if err != nil {
			log.Fatal("could not create trace file: ", err)
		}
		defer f.Close()
		if err := trace.Start(f); err != nil {
			log.Fatal("could not start trace: ", err)
		}
		defer trace.Stop()
	}

	fmt.Println("Starting trace demo...")

	// 問題 1: ゴルーチンが過剰に生成される
	// → トレースで大量のゴルーチン生成が見える
	problemExcessiveGoroutines()

	// 問題 2: チャネルで長時間ブロック
	// → トレースでブロッキング時間が見える
	problemChannelBlocking()

	// 問題 3: 頻繁な GC
	// → トレースで GC イベントが頻発
	problemFrequentGC()

	fmt.Println("Trace demo completed")
}

// 問題 1: 過剰なゴルーチン生成
func problemExcessiveGoroutines() {
	fmt.Println("Problem 1: Excessive goroutine creation")

	// 1000 個のゴルーチンを作成（多すぎる）
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 単純な計算
			sum := 0
			for j := 0; j < 1000; j++ {
				sum += j
			}
			_ = sum
		}(i)
	}
	wg.Wait()
}

// 問題 2: チャネルでのブロッキング
func problemChannelBlocking() {
	fmt.Println("Problem 2: Channel blocking")

	// バッファなしチャネル（ブロックする）
	ch := make(chan int)
	done := make(chan bool)

	// 送信側
	go func() {
		for i := 0; i < 100; i++ {
			ch <- i // 受信側が遅いのでブロック
			time.Sleep(10 * time.Millisecond) // わざと遅くする
		}
		close(ch)
	}()

	// 受信側（遅い）
	go func() {
		for v := range ch {
			_ = v
			time.Sleep(5 * time.Millisecond) // 送信側より遅い
		}
		done <- true
	}()

	<-done
}

// 問題 3: 頻繁な GC
func problemFrequentGC() {
	fmt.Println("Problem 3: Frequent GC")

	// 大量の短命なオブジェクトを生成（GC が頻発）
	for i := 0; i < 1000; i++ {
		// 毎回新しいスライスを作成
		data := make([]byte, 1024*1024) // 1MB
		_ = data

		// わざと GC を誘発
		if i%100 == 0 {
			runtime.GC()
		}
	}
}
