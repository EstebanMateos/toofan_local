package main

import (
	"embed"
	"math/rand"
	"strings"
)

//go:embed data/english/*.txt
var dataFS embed.FS

var wordList []string

func init() {
	files := []string{"easy.txt", "medium.txt", "hard.txt"}
	for _, f := range files {
		content, err := dataFS.ReadFile("data/english/" + f)
		if err != nil {
			continue
		}
		words := strings.Fields(string(content))
		for _, w := range words {
			w = strings.TrimSpace(w)
			if w != "" {
				wordList = append(wordList, w)
			}
		}
	}

	if len(wordList) == 0 {
		wordList = []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog"}
	}
}

func generateText(count int) string {
	res := make([]string, count)
	for i := range res {
		res[i] = wordList[rand.Intn(len(wordList))]
	}
	return strings.Join(res, " ")
}
