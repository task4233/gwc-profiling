package main

import (
	"testing"
)

func TestSearch(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		paths      []string
		maxResults int
		wantCount  int
	}{
		{
			name:       "basic function search",
			pattern:    "func.*",
			paths:      []string{"testdata"},
			maxResults: 100,
			wantCount:  10, // testdata/sample.go に10個以上の関数がある
		},
		{
			name:       "limited results",
			pattern:    "func.*",
			paths:      []string{"testdata"},
			maxResults: 3,
			wantCount:  3,
		},
		{
			name:       "type search",
			pattern:    "type.*struct",
			paths:      []string{"testdata"},
			maxResults: 100,
			wantCount:  1, // User struct
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := search(tt.pattern, tt.paths, tt.maxResults)

			if len(results) > tt.maxResults {
				t.Errorf("search() returned %d results, max was %d", len(results), tt.maxResults)
			}

			if tt.name == "basic function search" && len(results) < tt.wantCount {
				t.Errorf("search() returned %d results, want at least %d", len(results), tt.wantCount)
			} else if tt.name != "basic function search" && len(results) != tt.wantCount {
				t.Errorf("search() returned %d results, want %d", len(results), tt.wantCount)
			}

			// 結果の基本的な検証
			for _, result := range results {
				if result.File == "" {
					t.Error("search() result has empty File")
				}
				if result.Line <= 0 {
					t.Error("search() result has invalid Line number")
				}
				if result.Content == "" {
					t.Error("search() result has empty Content")
				}
			}
		})
	}
}

func TestSearchFile(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    int // 最小マッチ数
	}{
		{
			name:    "find functions",
			pattern: "func",
			want:    10,
		},
		{
			name:    "find imports",
			pattern: "import",
			want:    1,
		},
		{
			name:    "find return statements",
			pattern: "return",
			want:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// グローバル変数をリセット
			resultsMu.Lock()
			allResults = []Match{}
			resultsMu.Unlock()

			searchFile("testdata/sample.go", tt.pattern)

			resultsMu.Lock()
			count := len(allResults)
			resultsMu.Unlock()

			if count < tt.want {
				t.Errorf("searchFile() found %d matches, want at least %d", count, tt.want)
			}
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	pattern := "func.*"
	paths := []string{"testdata"}
	maxResults := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		search(pattern, paths, maxResults)
	}
}

func BenchmarkSearchFile(b *testing.B) {
	pattern := "func.*"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// グローバル変数をリセット
		resultsMu.Lock()
		allResults = []Match{}
		resultsMu.Unlock()

		searchFile("testdata/sample.go", pattern)
	}
}
