package lang

import (
	"embed"
	"io/fs"
	"math/rand"
	"sort"
	"strings"
)

//go:embed data
var dataFS embed.FS

type Snippet struct {
	Topic   string
	Content string
}

type langData struct {
	Name        string
	Words       []string
	EasyWords   []string
	MediumWords []string
	HardWords   []string
	Snippets    []Snippet
}

var languages = map[string]*langData{}

// Names holds code language names, sorted.
var Names []string

// WordNames holds languages with word lists, sorted.
var WordNames []string

// parseLesson extracts a snippet from a lesson file.
// Leading comments are stripped from the typed content.
// Supports // (go/js/dart), # (shell), and -- (lua) comment styles.
// The first "Topic:" comment becomes the display heading.
func parseLesson(content string) []Snippet {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var topic string
	codeStart := 0

	// scan leading comments for metadata, find where code begins
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// check if this is a comment line (any supported prefix)
		isComment := strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "--")

		if !isComment {
			break
		}

		// extract "Topic:" from any comment style
		if topic == "" {
			for _, prefix := range []string{"// Topic: ", "# Topic: ", "-- Topic: "} {
				if strings.HasPrefix(trimmed, prefix) {
					topic = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				}
			}
		}

		codeStart = i + 1
	}

	// skip blank lines between comments and code
	for codeStart < len(lines) && strings.TrimSpace(lines[codeStart]) == "" {
		codeStart++
	}

	codeText := strings.TrimSpace(strings.Join(lines[codeStart:], "\n"))
	if len(codeText) == 0 {
		return nil
	}
	if topic == "" {
		topic = "Code Snippet"
	}
	return []Snippet{{Topic: topic, Content: codeText}}
}

func init() {
	// Discover languages by looking at data/* directories
	entries, err := fs.ReadDir(dataFS, "data")
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		ld := &langData{Name: name}

		// Read easy.txt
		if raw, err := fs.ReadFile(dataFS, "data/"+name+"/easy.txt"); err == nil {
			words := strings.Fields(string(raw))
			ld.EasyWords = append(ld.EasyWords, words...)
			ld.Words = append(ld.Words, words...)
		}

		// Read medium.txt
		if raw, err := fs.ReadFile(dataFS, "data/"+name+"/medium.txt"); err == nil {
			words := strings.Fields(string(raw))
			ld.MediumWords = append(ld.MediumWords, words...)
			ld.Words = append(ld.Words, words...)
		}

		// Read hard.txt
		if raw, err := fs.ReadFile(dataFS, "data/"+name+"/hard.txt"); err == nil {
			words := strings.Fields(string(raw))
			ld.HardWords = append(ld.HardWords, words...)
			ld.Words = append(ld.Words, words...)
		}

		lessons, _ := fs.ReadDir(dataFS, "data/"+name)
		for _, lesson := range lessons {
			if lesson.IsDir() {
				continue
			}
			if strings.HasSuffix(lesson.Name(), ".txt") {
				continue
			}
			path := "data/" + name + "/" + lesson.Name()
			if raw, err := fs.ReadFile(dataFS, path); err == nil {
				snips := parseLesson(string(raw))
				ld.Snippets = append(ld.Snippets, snips...)
			}
		}

		if len(ld.Words) > 0 || len(ld.Snippets) > 0 {
			languages[name] = ld
			if len(ld.Words) > 0 {
				WordNames = append(WordNames, name)
			}
			if len(ld.Snippets) > 0 {
				Names = append(Names, name)
			}
		}
	}
	sort.Strings(Names)
	sort.Strings(WordNames)
}

func HasWords(name string) bool {
	ld, ok := languages[name]
	return ok && len(ld.Words) > 0
}

func HasSnippets(name string) bool {
	ld, ok := languages[name]
	return ok && len(ld.Snippets) > 0
}

func DefaultWordName() string {
	if HasWords("english") {
		return "english"
	}
	if len(WordNames) > 0 {
		return WordNames[0]
	}
	return "english"
}

func DefaultCodeName() string {
	if HasSnippets("go") {
		return "go"
	}
	if len(Names) > 0 {
		return Names[0]
	}
	return "go"
}

// RandomWords picks random words for the word-mode typing test
func RandomWords(name string, difficulty string, count int) []string {
	ld, ok := languages[name]
	if !ok || len(ld.Words) == 0 {
		ld = languages["english"]
	}

	var wordList []string
	switch difficulty {
	case "easy":
		wordList = ld.EasyWords
	case "medium":
		wordList = ld.MediumWords
	case "hard":
		wordList = ld.HardWords
	default:
		wordList = ld.Words
	}

	if len(wordList) == 0 {
		wordList = ld.Words // fallback to general words
	}

	if len(wordList) == 0 {
		return []string{"hello", "world"}
	}

	out := make([]string, count)
	for i := range out {
		out[i] = wordList[rand.Intn(len(wordList))]
	}
	return out
}

// RandomSnippet picks a random code snippet for code-mode typing.
func RandomSnippet(name string, difficulty string) Snippet {
	ld, ok := languages[name]
	if !ok || len(ld.Snippets) == 0 {
		return Snippet{
			Topic:   "Fallback Words",
			Content: strings.Join(RandomWords(name, difficulty, 50), " "),
		}
	}

	return ld.Snippets[rand.Intn(len(ld.Snippets))]
}

// GetSnippets returns all snippets for a given language
func GetSnippets(name string) []Snippet {
	if ld, ok := languages[name]; ok {
		return ld.Snippets
	}
	return nil
}
