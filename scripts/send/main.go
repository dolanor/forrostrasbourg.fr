package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/joho/godotenv"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"go.abhg.dev/goldmark/frontmatter"
)

const messageTempl = `Bonjour à toutes et tous,

Pour cette semaine, on a :
{{ range . }}
- Le {{ .WeekDay }} {{ .StartDay | zeroPrefix }}/{{ .StartMonth | zeroPrefix }} à {{ .StartHour }}, {{ .Title }} : {{ .URL -}}
{{ end }}

Au plaisir de vous y voir
`

func main() {
	cfg, err := loadConfig()
	if err != nil {
		panic(err)
	}

	err = run(cfg.beeperAccessToken, cfg.chatIDs, cfg.forWeek, cfg.send)
	if err != nil {
		panic(err)
	}
}

type config struct {
	beeperAccessToken string
	chatIDs           []string
	send              bool
	forWeek           int
}

func loadConfig() (config, error) {
	cfg := config{}
	err := godotenv.Load()
	if err != nil {
		return cfg, err
	}

	flag.BoolVar(&cfg.send, "send", false, "to actually send the message")
	flag.IntVar(&cfg.forWeek, "for-week", 0, "to change the week this message is for (it's the number of the week of the current year)")

	flag.Parse()

	var ok bool

	cfg.beeperAccessToken, ok = os.LookupEnv("BEEPER_ACCESS_TOKEN")
	if !ok {
		return cfg, errors.New("BEEPER_ACCESS_TOKEN not set in env")
	}

	chatID, ok := os.LookupEnv("FORROSTRASBOURG_CHAT_GROUP_ID")
	if !ok {
		return cfg, errors.New("FORROSTRASBOURG_CHAT_GROUP_ID not set in env")
	}

	if chatID != "" {
		cfg.chatIDs = append(cfg.chatIDs, chatID)
	}

	chatID, ok = os.LookupEnv("SPECIAL_CHAT_GROUP_ID")
	if !ok {
		return cfg, errors.New("SPECIAL_CHAT_GROUP_ID not set in env")
	}

	if chatID != "" {
		cfg.chatIDs = append(cfg.chatIDs, chatID)
	}

	return cfg, nil
}

type event struct {
	Title      string
	StartDay   int
	StartMonth int
	WeekDay    string
	StartHour  string
	URL        *url.URL
}

func run(beeperAccessToken string, chatIDs []string, forWeek int, send bool) error {
	slog.Info("run", "chat_ids", chatIDs)

	currentYear, currentWeek := time.Now().Add(24 * time.Hour).UTC().ISOWeek()
	if forWeek != 0 {
		currentWeek = forWeek
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			&frontmatter.Extender{},
		),
	)

	dirPath := "./content/evenements/"
	eventDir, err := os.OpenRoot(dirPath)
	if err != nil {
		return err
	}
	defer eventDir.Close()

	var events []event

	fs.WalkDir(eventDir.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".md" {
			slog.Debug("ignoring", "path", path, "ext", ext)
			return nil
		}

		f, err := eventDir.FS().Open(path)
		if err != nil {
			return err
		}

		fm, err := getFrontMatter(md, f)
		if err != nil {
			return err
		}

		year, week := fm.StartDate.ISOWeek()
		if year != currentYear || week != currentWeek {
			slog.Debug("igoring event", "year", year, "week", week, "date", fm.StartDate)
			return nil
		}

		pagePath := filepath.Base(path)
		pagePath = strings.TrimSuffix(pagePath, ext)

		u, err := url.Parse("https://forrostrasbourg.fr/evenements/" + pagePath)
		if err != nil {
			slog.Debug("parse url", "path", path, "pagePath", pagePath)
			return nil
		}

		events = append(events, event{
			Title:      fm.Title,
			StartDay:   fm.StartDate.Day(),
			StartMonth: int(fm.StartDate.Month()),
			WeekDay:    frenchWeekDay(fm.StartDate.Weekday()),
			StartHour:  fm.StartDate.Format("15h04"),
			URL:        u,
		})

		return nil
	})

	fmt.Println("EVENTS:\n", events)

	fm := template.FuncMap{
		"zeroPrefix": func(digit any) string {
			zeroPrefixed := fmt.Sprintf("%02d", digit)
			return zeroPrefixed
		},
	}
	t, err := template.New("message").Funcs(fm).Parse(messageTempl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, events)
	if err != nil {
		return err
	}

	message := buf.String()
	fmt.Println("MESSAGE:\n", message)

	if !send {
		slog.Info("not sending")
		return nil
	}

	for _, chatID := range chatIDs {
		err = sendToGroup(beeperAccessToken, chatID, message)
		if err != nil {
			return err
		}
	}
	fmt.Println("MESSAGE SENT")

	return nil
}

type FrontMatter struct {
	Title     string
	StartDate time.Time `yaml:"startDate"`
	EndDate   time.Time `yaml:"endDate"`
}

func getFrontMatter(mdDecoder goldmark.Markdown, r io.Reader) (fm FrontMatter, err error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return fm, err
	}

	ctx := parser.NewContext()
	err = mdDecoder.Convert(b, io.Discard, parser.WithContext(ctx))
	if err != nil {
		return fm, err
	}

	fmd := frontmatter.Get(ctx)
	if fmd == nil {
		return fm, errors.New("no frontmatter found")
	}

	err = fmd.Decode(&fm)
	if err != nil {
		return fm, err
	}

	return fm, nil
}

func frenchWeekDay(day time.Weekday) string {
	frenchDays := map[time.Weekday]string{
		time.Monday:    "lundi",
		time.Tuesday:   "mardi",
		time.Wednesday: "mercredi",
		time.Thursday:  "jeudi",
		time.Friday:    "vendredi",
		time.Saturday:  "samedi",
		time.Sunday:    "dimanche",
	}

	d, ok := frenchDays[day]
	if !ok {
		return "jour"
	}

	return d
}

func sendToGroup(beeperAccessToken string, chatID string, message string) error {
	type Message struct {
		Text string `json:"text"`
	}

	msg := Message{
		Text: message,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(msg)
	if err != nil {
		return err
	}

	chatURL := fmt.Sprintf("http://localhost:23373/v1/chats/%s/messages", chatID)
	req, err := http.NewRequest(http.MethodPost, chatURL, &buf)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", beeperAccessToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		return fmt.Errorf("unexpected status: %v: %s", resp.StatusCode, b)
	}
	return nil
}
