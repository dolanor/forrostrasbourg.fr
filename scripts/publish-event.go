package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "gopkg.in/yaml.v3"
    "io"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "text/template"
    "time"
)

type EventData struct {
    Date                string
    LongDate            string
    LongDateCapitalized string
}

type FrontMatterData struct {
    Title string `yaml:"title"`
    Place string `yaml:"place"`
    City  string `yaml:"city"`
}

func runGitCommand(dir string, args ...string) error {
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to run git command '%v': %v\nOutput: %s", args, err, string(output))
    }
    return nil
}

func getWeekdayName(d time.Time, lang string) string {
    var weekdays []string
    switch lang {
    case "fr":
        weekdays = []string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}
    default:
        weekdays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
    }

    return weekdays[int(d.Weekday())]
}

func getMonthName(d time.Time, lang string) string {
    var months []string
    switch lang {
    case "fr":
        months = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
    default:
        months = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}
    }

    return months[int(d.Month())-1]
}

func capitalizeFirstLetter(s string) string {
    if s == "" {
        return s
    }
    return strings.ToUpper(string(s[0])) + s[1:]
}

// extractFrontMatter parses the front matter from the generated markdown file
// and returns title, place, and city.
func extractFrontMatter(filePath string) (FrontMatterData, error) {
    var fmData FrontMatterData

    f, err := os.Open(filePath)
    if err != nil {
        return fmData, fmt.Errorf("failed to open file for front matter parsing: %v", err)
    }
    defer f.Close()

    var frontMatterLines []string
    inFrontMatter := false
    var content bytes.Buffer
    buf := make([]byte, 4096)
    for {
        n, err := f.Read(buf)
        if n > 0 {
            content.Write(buf[:n])
        }
        if err == io.EOF {
            break
        } else if err != nil {
            return fmData, err
        }
    }

    lines := strings.Split(content.String(), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "---" {
            if !inFrontMatter {
                // starting front matter
                inFrontMatter = true
                continue
            } else {
                // ending front matter
                break
            }
        }
        if inFrontMatter {
            frontMatterLines = append(frontMatterLines, line)
        }
    }

    fmContent := strings.Join(frontMatterLines, "\n")
    if fmContent == "" {
        return fmData, fmt.Errorf("no front matter found in %s", filePath)
    }

    if err := yaml.Unmarshal([]byte(fmContent), &fmData); err != nil {
        return fmData, fmt.Errorf("failed to parse front matter: %v", err)
    }

    return fmData, nil
}

// publishEventMarkdown creates the markdown file and handles git operations.
// It logs every action and performs it only if dryRun is false.
// It returns the outputPath, EventData, and FrontMatterData.
func publishEventMarkdown(templatePath string, parsedDate time.Time, dateStr, lang string, dryRun bool) (string, EventData, FrontMatterData, error) {
    // Convert date to YYMMDD format
    formattedDate := parsedDate.Format("060102")

    // Determine the base filename from the template
    templateFile := filepath.Base(templatePath)
    baseName := strings.TrimSuffix(templateFile, ".template")
    baseName = strings.TrimSuffix(baseName, ".md")

    // Construct the output filename: e.g. "241129-pachamamas.md"
    outputFilename := fmt.Sprintf("%s-%s.md", formattedDate, baseName)
    outputDir := "content/evenements"
    outputPath := filepath.Join(outputDir, outputFilename)

    // Prepare EventData
    weekdayLower := getWeekdayName(parsedDate, lang)
    monthLower := getMonthName(parsedDate, lang)
    day := parsedDate.Day()
    longDate := fmt.Sprintf("%s %d %s", weekdayLower, day, monthLower)
    longDateCapitalized := fmt.Sprintf("%s %d %s", capitalizeFirstLetter(weekdayLower), day, monthLower)

    data := EventData{
        Date:                dateStr,
        LongDate:            longDate,
        LongDateCapitalized: longDateCapitalized,
    }

    // Log file creation
    log.Printf("Creating event markdown file at: %s", outputPath)
    if !dryRun {
        if _, err := os.Stat(outputDir); os.IsNotExist(err) {
            if err := os.MkdirAll(outputDir, 0755); err != nil {
                return "", data, FrontMatterData{}, fmt.Errorf("failed to create output directory: %v", err)
            }
        }

        tmpl, err := template.ParseFiles(templatePath)
        if err != nil {
            return "", data, FrontMatterData{}, fmt.Errorf("error parsing template file: %v", err)
        }

        outFile, err := os.Create(outputPath)
        if err != nil {
            return "", data, FrontMatterData{}, fmt.Errorf("failed to create output file: %v", err)
        }
        defer outFile.Close()

        if err := tmpl.Execute(outFile, data); err != nil {
            return "", data, FrontMatterData{}, fmt.Errorf("error executing template: %v", err)
        }
    }

    // Extract front matter (title, place, city)
    var fmData FrontMatterData
    if !dryRun {
        fm, err := extractFrontMatter(outputPath)
        if err != nil {
            return outputPath, data, fmData, fmt.Errorf("failed to extract front matter: %v", err)
        }
        fmData = fm
    }

    // Log git add
    log.Printf("Running 'git add' on %s", outputPath)
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return outputPath, data, fmData, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "add", outputPath); err != nil {
            return outputPath, data, fmData, fmt.Errorf("git add failed: %v", err)
        }
    }

    // Log git commit
    commitMsg := fmt.Sprintf("Add event for %s based on template %s", dateStr, templateFile)
    log.Printf("Running 'git commit' with message: %q", commitMsg)
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return outputPath, data, fmData, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "commit", "-m", commitMsg); err != nil {
            return outputPath, data, fmData, fmt.Errorf("git commit failed: %v", err)
        }
    }

    // Log git push
    log.Println("Running 'git push'")
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return outputPath, data, fmData, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "push"); err != nil {
            return outputPath, data, fmData, fmt.Errorf("git push failed: %v", err)
        }
    }

    return outputPath, data, fmData, nil
}

func publishEventOnFacebook(data EventData, fmData FrontMatterData, outputPath string) error {
    pageID := "351984064669408" // Replace with your actual Facebook Page ID
    pageAccessToken := os.Getenv("FACEBOOK_PAGE_ACCESS_TOKEN")
    if pageAccessToken == "" {
        return fmt.Errorf("FACEBOOK_PAGE_ACCESS_TOKEN not set in environment")
    }

    // Derive the event URL from the output filename.
    baseFilename := filepath.Base(outputPath) // e.g. "241127-pachamamas.md"
    eventSlug := strings.TrimSuffix(baseFilename, ".md")
    eventURL := fmt.Sprintf("https://forrostrasbourg.fr/evenements/%s/", eventSlug)

    // Create a simple French message describing the event
    message := fmt.Sprintf(
`%s
Se tiendra à %s, %s le %s.

Plus d'informations :
%s`,
        fmData.Title,
        fmData.Place,
        fmData.City,
        data.LongDateCapitalized,
        eventURL,
    )

    url := fmt.Sprintf("https://graph.facebook.com/%s/feed", pageID)
    requestBody := map[string]string{
        "message":      message,
        "access_token": pageAccessToken,
    }

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
        return fmt.Errorf("error marshaling request body: %v", err)
    }

    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error posting to Facebook: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        var fbErr map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&fbErr); err == nil {
            return fmt.Errorf("facebook API returned status %d: %v", resp.StatusCode, fbErr)
        }
        return fmt.Errorf("facebook API returned status %d", resp.StatusCode)
    }

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return fmt.Errorf("error decoding response body: %v", err)
    }

    log.Printf("Post published successfully on Facebook! Response: %+v\n", result)
    return nil
}

func main() {
    dateStr := flag.String("date", "", "Event date in YYYY-MM-DD format")
    templatePath := flag.String("template", "", "Path to the template markdown file (e.g. pachamamas.md.template)")
    lang := flag.String("lang", "fr", "Language code for date formatting (e.g. 'fr' or 'en')")
    dryRun := flag.Bool("dry-run", false, "If true, only echo the actions without carrying them out")
    publishFacebook := flag.Bool("publish-facebook", false, "If true, attempt to publish the event on Facebook")
    flag.Parse()

    if *dateStr == "" {
        log.Fatal("You must provide a -date parameter.")
    }
    if *templatePath == "" {
        log.Fatal("You must provide a -template parameter.")
    }

    parsedDate, err := time.Parse("2006-01-02", *dateStr)
    if err != nil {
        log.Fatalf("Invalid date format: %v", err)
    }

    // Publish the markdown (file creation and git)
    outputPath, data, fmData, err := publishEventMarkdown(*templatePath, parsedDate, *dateStr, *lang, *dryRun)
    if err != nil {
        log.Fatalf("Error publishing event: %v", err)
    }

    log.Printf("Event published successfully: %s\n", outputPath)

    // Attempt Facebook publishing only if requested and not dry-run
    if *publishFacebook {
        log.Println("Attempting to publish event on Facebook")
        if !*dryRun {
            if err := publishEventOnFacebook(data, fmData, outputPath); err != nil {
                log.Fatalf("Failed to publish event on Facebook: %v", err)
            }
        }
    }
}
