package main

import (
    "bytes"
    "encoding/json"
    "errors"
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

func runGitCommand(dir string, args ...string) (string, error) {
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()
    if err != nil {
        return string(output), fmt.Errorf("failed to run git command '%v': %v\nOutput: %s", args, err, string(output))
    }
    return string(output), nil
}

func runGitCheckChanges(dir, filePath string) (bool, error) {
    // run: git diff --cached --exit-code filePath
    // exit code 0 means no changes, exit code 1 means changes present
    cmd := exec.Command("git", "diff", "--cached", "--exit-code", filePath)
    cmd.Dir = dir
    output, err := cmd.CombinedOutput()

    if err != nil {
        // Check if it's just exit code 1 (changes present) or a real error
        if exitErr, ok := err.(*exec.ExitError); ok {
            // Exit status 1 indicates differences
            if exitErr.ExitCode() == 1 {
                // Changes present
                return true, nil
            }
        }
        return false, fmt.Errorf("error running git diff: %v\nOutput: %s", err, string(output))
    }

    // If we get here, err == nil, exit code 0 → no changes
    return false, nil
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
// Returns outputPath, EventData, FrontMatterData, and a boolean indicating if event was already published.
func publishEventMarkdown(templatePath string, parsedDate time.Time, dateStr, lang string, dryRun bool) (string, EventData, FrontMatterData, bool, error) {
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
                return "", data, FrontMatterData{}, false, fmt.Errorf("failed to create output directory: %v", err)
            }
        }

        tmpl, err := template.ParseFiles(templatePath)
        if err != nil {
            return "", data, FrontMatterData{}, false, fmt.Errorf("error parsing template file: %v", err)
        }

        outFile, err := os.Create(outputPath)
        if err != nil {
            return "", data, FrontMatterData{}, false, fmt.Errorf("failed to create output file: %v", err)
        }
        defer outFile.Close()

        if err := tmpl.Execute(outFile, data); err != nil {
            return "", data, FrontMatterData{}, false, fmt.Errorf("error executing template: %v", err)
        }
    }

    fmData := FrontMatterData{}
    if !dryRun {
        fm, err := extractFrontMatter(outputPath)
        if err != nil {
            return outputPath, data, fmData, false, fmt.Errorf("failed to extract front matter: %v", err)
        }
        fmData = fm
    }

    // Log git add
    log.Printf("Running 'git add' on %s", outputPath)
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return outputPath, data, fmData, false, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if _, err := runGitCommand(repoDir, "add", outputPath); err != nil {
            return outputPath, data, fmData, false, fmt.Errorf("git add failed: %v", err)
        }

        // Now check if there are any changes via git diff
        hasChanges, err := runGitCheckChanges(repoDir, outputPath)
        if err != nil {
            return outputPath, data, fmData, false, err
        }
        if !hasChanges {
            // No changes to commit
            log.Println("No changes detected. The event appears to be already published.")
            return outputPath, data, fmData, true, nil
        }

        // If we reach here, changes are present, proceed to commit
        commitMsg := fmt.Sprintf("Add event for %s based on template %s", dateStr, templateFile)
        log.Printf("Running 'git commit' with message: %q", commitMsg)
        if _, err := runGitCommand(repoDir, "commit", "-m", commitMsg); err != nil {
            return outputPath, data, fmData, false, fmt.Errorf("git commit failed: %v", err)
        }

        // Log git push
        log.Println("Running 'git push'")
        if _, err := runGitCommand(repoDir, "push"); err != nil {
            return outputPath, data, fmData, false, fmt.Errorf("git push failed: %v", err)
        }
    }

    return outputPath, data, fmData, false, nil
}

// waitForEventPage checks the given URL periodically until it gets a 200 response or hits a timeout.
func waitForEventPage(eventURL string, timeout, interval time.Duration) error {
    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        resp, err := http.Get(eventURL)
        if err == nil {
            resp.Body.Close()
            if resp.StatusCode == http.StatusOK {
                // Page is live
                return nil
            }
        }

        // Page not live yet, wait before retrying
        time.Sleep(interval)
    }

    return errors.New("timed out waiting for the event page to become available")
}

func publishEventOnFacebook(data EventData, fmData FrontMatterData, outputPath string, pageAccessToken string) error {
    pageID := "351984064669408" // Replace with your actual Facebook Page ID

    // Derive the event URL from the output filename.
    baseFilename := filepath.Base(outputPath)
    eventSlug := strings.TrimSuffix(baseFilename, ".md")
    eventURL := fmt.Sprintf("https://forrostrasbourg.fr/evenements/%s/", eventSlug)

    // Create a simple French message describing the event
    message := fmt.Sprintf(
        `le %s: %s
        %s, %s .

Plus d'informations :
%s`,
        fmData.Title,
        fmData.Place,
        fmData.City,
        data.LongDateCapitalized,
        eventURL,
    )

    // Before publishing to Facebook, wait for the event page to become available
    log.Printf("Waiting for event page to become available: %s", eventURL)
    if err := waitForEventPage(eventURL, 5*time.Minute, 10*time.Second); err != nil {
        return fmt.Errorf("event page did not become available in time: %v", err)
    }

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

    // Check FACEBOOK_PAGE_ACCESS_TOKEN once if publishing to Facebook
    var pageAccessToken string
    if *publishFacebook {
        pageAccessToken = os.Getenv("FACEBOOK_PAGE_ACCESS_TOKEN")
        if pageAccessToken == "" {
            log.Fatal("FACEBOOK_PAGE_ACCESS_TOKEN not set in environment")
        }
    }

    parsedDate, err := time.Parse("2006-01-02", *dateStr)
    if err != nil {
        log.Fatalf("Invalid date format: %v", err)
    }

    // Publish the markdown (file creation and git)
    outputPath, data, fmData, _, err := publishEventMarkdown(*templatePath, parsedDate, *dateStr, *lang, *dryRun)
    if err != nil {
        log.Fatalf("Error publishing event: %v", err)
    }

    // Event is successfully published (git)
    log.Printf("Event published successfully: %s\n", outputPath)

    // Attempt Facebook publishing only if requested and not dry-run
    if *publishFacebook && !*dryRun {
        log.Println("Attempting to publish event on Facebook")
        if err := publishEventOnFacebook(data, fmData, outputPath, pageAccessToken); err != nil {
            log.Fatalf("Failed to publish event on Facebook: %v", err)
        }
    }
}
