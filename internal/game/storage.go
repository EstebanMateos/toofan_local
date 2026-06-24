package game

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var dataDir string

type RaceRecord struct {
	ID         string      `json:"id"`
	At         string      `json:"at"`
	Duration   int         `json:"duration"`
	Mode       string      `json:"mode"`
	Language   string      `json:"language"`
	Difficulty string      `json:"difficulty"`
	Text       string      `json:"text"`
	Stats      Stats       `json:"stats"`
	Points     []RacePoint `json:"points"`
}

type ConfigRecord struct {
	Duration   int                `json:"duration"`
	Mode       string             `json:"mode"`
	Lang       string             `json:"lang"`
	Difficulty string             `json:"difficulty"`
	Theme      string             `json:"theme"`
	Username   string             `json:"username,omitempty"`
	ServerURL  string             `json:"serverURL,omitempty"`
	PB         map[string]float64 `json:"pb"`
}

type ResultRecord struct {
	At   string  `json:"at"`
	WPM  float64 `json:"wpm"`
	Acc  float64 `json:"acc"`
	Dur  int     `json:"dur"`
	Mode string  `json:"mode"`
	Raw  float64 `json:"raw"`
	Err  int     `json:"err"`
}

func init() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dataDir = filepath.Join(configDir, "toofan")
	migrate()
}

func SaveResult(s Stats, duration int, mode string, language string) {
	os.MkdirAll(dataDir, 0755)
	f, err := os.OpenFile(filepath.Join(dataDir, "results.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	label := mode
	if mode == "code" {
		label = "code:" + language
	}

	rec := ResultRecord{
		At:   time.Now().Format("2006-01-02 15:04"),
		WPM:  s.WPM,
		Acc:  s.Accuracy,
		Dur:  duration,
		Mode: label,
		Raw:  s.Raw,
		Err:  s.Mistakes,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(b))
}

func GetPB(duration int, mode string) float64 {
	cfg, ok := loadConfigRecord()
	if !ok || cfg.PB == nil {
		return 0
	}
	return cfg.PB[fmt.Sprintf("%s-%d", mode, duration)]
}

func SavePB(duration int, mode string, wpm float64) {
	cfg, _ := loadConfigRecord()
	if cfg.PB == nil {
		cfg.PB = make(map[string]float64)
	}
	cfg.PB[fmt.Sprintf("%s-%d", mode, duration)] = wpm
	saveConfigRecord(cfg)
}

func SaveRace(r RaceRecord) {
	os.MkdirAll(dataDir, 0755)
	path := filepath.Join(dataDir, "races.jsonl")

	races := LoadRaces()
	races = append(races, r)
	if len(races) > 10 {
		races = races[len(races)-10:]
	}

	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	for _, rec := range races {
		b, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		fmt.Fprintln(f, string(b))
	}
}

func LoadRaces() []RaceRecord {
	path := filepath.Join(dataDir, "races.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []RaceRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r RaceRecord
		if err := json.Unmarshal([]byte(scanner.Text()), &r); err != nil {
			continue
		}
		if r.Text == "" || len(r.Points) == 0 {
			continue
		}
		out = append(out, r)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func LoadConfig() (duration int, mode string, language string, difficulty string, themeName string, username string, serverURL string) {
	duration, mode, language, difficulty, themeName = 30, "words", "go", "easy", "tokyonight"
	cfg, ok := loadConfigRecord()
	if !ok {
		return
	}
	if cfg.Duration > 0 {
		duration = cfg.Duration
	}
	if cfg.Mode != "" {
		mode = cfg.Mode
	}
	if cfg.Lang != "" {
		language = cfg.Lang
	}
	if cfg.Difficulty != "" {
		difficulty = cfg.Difficulty
	}
	if cfg.Theme != "" {
		themeName = cfg.Theme
	}
	username = cfg.Username
	serverURL = cfg.ServerURL
	return
}

func SaveConfig(duration int, mode string, language string, difficulty string, themeName string, username string, serverURL string) {
	cfg, _ := loadConfigRecord()
	cfg.Duration = duration
	cfg.Mode = mode
	cfg.Lang = language
	cfg.Difficulty = difficulty
	cfg.Theme = themeName
	cfg.Username = username
	cfg.ServerURL = serverURL
	saveConfigRecord(cfg)
}

// SplitBundle parses a bundled backup file (sections marked with "### filename")
// and returns a map of filename -> content.
func SplitBundle(content string) map[string]string {
	sections := make(map[string]string)
	var currentName string
	var buf strings.Builder

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "### ") {
			if currentName != "" {
				sections[currentName] = strings.TrimRight(buf.String(), "\n")
			}
			currentName = strings.TrimPrefix(line, "### ")
			buf.Reset()
		} else if currentName != "" {
			buf.WriteString(line + "\n")
		}
	}
	if currentName != "" {
		sections[currentName] = strings.TrimRight(buf.String(), "\n")
	}
	return sections
}

func SaveBackup() (string, error) {
	backupDir := filepath.Join(dataDir, "backups")
	os.MkdirAll(backupDir, 0755)

	stamp := time.Now().Format("2006-01-02_15-04")
	dest := filepath.Join(backupDir, fmt.Sprintf("toofan_backup_%s.bak", stamp))

	var bundle strings.Builder
	for _, name := range []string{"config.json", "results.jsonl", "races.jsonl"} {
		data, err := os.ReadFile(filepath.Join(dataDir, name))
		if err != nil {
			continue
		}
		bundle.WriteString("### " + name + "\n")
		bundle.Write(data)
		bundle.WriteString("\n")
	}
	if err := os.WriteFile(dest, []byte(bundle.String()), 0644); err != nil {
		return "", err
	}
	return dest, nil
}

func loadConfigRecord() (ConfigRecord, bool) {
	cfg := ConfigRecord{
		Duration:   30,
		Mode:       "words",
		Lang:       "go",
		Difficulty: "easy",
		Theme:      "tokyonight",
		PB:         make(map[string]float64),
	}
	data, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		return cfg, false
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, false
	}
	if cfg.PB == nil {
		cfg.PB = make(map[string]float64)
	}
	return cfg, true
}

func saveConfigRecord(cfg ConfigRecord) {
	os.MkdirAll(dataDir, 0755)
	if cfg.PB == nil {
		cfg.PB = make(map[string]float64)
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dataDir, "config.json"), b, 0644)
}

func migrate() {
	_ = os.MkdirAll(dataDir, 0755)
	migrateConfig()
	migrateResults()
	migrateRaces()
	migrateBackups()
}

func migrateConfig() {
	oldConfig := filepath.Join(dataDir, "config.txt")
	oldPB := filepath.Join(dataDir, "pb.txt")
	newConfig := filepath.Join(dataDir, "config.json")

	if !fileExists(oldConfig) && !fileExists(oldPB) {
		return
	}

	if !fileExists(newConfig) {
		cfg := ConfigRecord{
			Duration:   30,
			Mode:       "words",
			Lang:       "go",
			Difficulty: "easy",
			Theme:      "tokyonight",
			PB:         make(map[string]float64),
		}

		if f, err := os.Open(oldConfig); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "=", 2)
				if len(parts) != 2 {
					continue
				}
				switch parts[0] {
				case "duration":
					cfg.Duration, _ = strconv.Atoi(parts[1])
				case "mode":
					cfg.Mode = parts[1]
				case "lang":
					cfg.Lang = parts[1]
				case "difficulty":
					cfg.Difficulty = parts[1]
				case "theme":
					cfg.Theme = parts[1]
				case "username":
					cfg.Username = parts[1]
				case "server":
					cfg.ServerURL = parts[1]
				}
			}
			_ = f.Close()
		}

		if f, err := os.Open(oldPB); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "=", 2)
				if len(parts) != 2 {
					continue
				}
				val, err := strconv.ParseFloat(parts[1], 64)
				if err != nil {
					continue
				}
				cfg.PB[parts[0]] = val
			}
			_ = f.Close()
		}

		saveConfigRecord(cfg)
		if !fileExists(newConfig) {
			return
		}
		fmt.Printf("migrated config to %s\n", newConfig)
	}

	_ = os.Remove(oldConfig)
	_ = os.Remove(oldPB)
}

func migrateResults() {
	oldResults := filepath.Join(dataDir, "results.txt")
	newResults := filepath.Join(dataDir, "results.jsonl")

	if !fileExists(oldResults) {
		return
	}

	if !fileExists(newResults) {
		in, err := os.Open(oldResults)
		if err != nil {
			return
		}

		out, err := os.Create(newResults)
		if err != nil {
			in.Close()
			return
		}

		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			rec, ok := parseLegacyResultLine(scanner.Text())
			if !ok {
				continue
			}
			b, err := json.Marshal(rec)
			if err != nil {
				continue
			}
			fmt.Fprintln(out, string(b))
		}
		in.Close()
		out.Close()

		if !fileExists(newResults) {
			return
		}
		fmt.Printf("migrated results to %s\n", newResults)
	}

	_ = os.Remove(oldResults)
}

func migrateRaces() {
	oldRaces := filepath.Join(dataDir, "races.txt")
	newRaces := filepath.Join(dataDir, "races.jsonl")

	if !fileExists(oldRaces) {
		return
	}

	if !fileExists(newRaces) {
		data, err := os.ReadFile(oldRaces)
		if err != nil {
			return
		}
		if err := os.WriteFile(newRaces, data, 0644); err != nil {
			return
		}
		fmt.Printf("migrated races to %s\n", newRaces)
	}

	_ = os.Remove(oldRaces)
}

func migrateBackups() {
	backupDir := filepath.Join(dataDir, "backups")

	txt, _ := filepath.Glob(filepath.Join(backupDir, "toofan_backup_*.txt"))
	for _, src := range txt {
		dest := strings.TrimSuffix(src, ".txt") + ".bak"
		if !fileExists(dest) {
			_ = os.Rename(src, dest)
		} else {
			_ = os.Remove(src)
		}
	}

	jsonl, _ := filepath.Glob(filepath.Join(backupDir, "toofan_backup_*.jsonl"))
	for _, src := range jsonl {
		raw, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		dest := strings.TrimSuffix(src, ".jsonl") + ".bak"
		if fileExists(dest) {
			_ = os.Remove(src)
			continue
		}

		content := string(raw)
		if strings.HasPrefix(strings.TrimSpace(content), "{") {
			var bundle strings.Builder
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var entry struct {
					File string `json:"file"`
					Data string `json:"data"`
				}
				if json.Unmarshal([]byte(line), &entry) != nil {
					continue
				}
				bundle.WriteString("### " + entry.File + "\n")
				bundle.WriteString(entry.Data)
				if !strings.HasSuffix(entry.Data, "\n") {
					bundle.WriteString("\n")
				}
			}
			_ = os.WriteFile(dest, []byte(bundle.String()), 0644)
		} else {
			_ = os.WriteFile(dest, raw, 0644)
		}

		if fileExists(dest) {
			_ = os.Remove(src)
		}
	}
}

func parseLegacyResultLine(line string) (ResultRecord, bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 5 {
		return ResultRecord{}, false
	}

	rec := ResultRecord{
		At:   strings.TrimSpace(parts[0]),
		Mode: strings.TrimSpace(parts[4]),
	}
	var err error

	wpmStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "wpm"))
	rec.WPM, err = strconv.ParseFloat(wpmStr, 64)
	if err != nil {
		return ResultRecord{}, false
	}

	accStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[2]), "%"))
	rec.Acc, err = strconv.ParseFloat(accStr, 64)
	if err != nil {
		return ResultRecord{}, false
	}

	durStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[3]), "s"))
	rec.Dur, err = strconv.Atoi(durStr)
	if err != nil {
		return ResultRecord{}, false
	}

	if len(parts) >= 6 {
		rawStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[5]), "raw"))
		rec.Raw, _ = strconv.ParseFloat(rawStr, 64)
	}
	if len(parts) >= 7 {
		errStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[6]), "err"))
		rec.Err, _ = strconv.Atoi(errStr)
	}
	return rec, true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func RestoreBackup(src string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	for name, data := range SplitBundle(string(raw)) {
		_ = os.WriteFile(filepath.Join(dataDir, name), []byte(data), 0644)
	}
	return nil
}

func ListBackups() ([]string, string) {
	backupDir := filepath.Join(dataDir, "backups")
	files, _ := filepath.Glob(filepath.Join(backupDir, "toofan_backup_*.bak"))
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	return files, backupDir
}
