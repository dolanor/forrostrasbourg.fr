package main

import (
    "flag"
    "fmt"
    "log"
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
        // English fallback
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
        // English fallback
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

// publishEventMarkdown creates the markdown file and handles git operations.
// It logs every action and performs it only if dryRun is false.
func publishEventMarkdown(templatePath string, parsedDate time.Time, dateStr, lang string, dryRun bool) (string, EventData, error) {
    // Convert date to YYMMDD format
    formattedDate := parsedDate.Format("060102") // "06"=YY, "01"=MM, "02"=DD

    // Determine the base filename from the template
    templateFile := filepath.Base(templatePath)
    baseName := strings.TrimSuffix(templateFile, ".template") // e.g. "pachamamas.md"
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
                return "", data, fmt.Errorf("failed to create output directory: %v", err)
            }
        }

        tmpl, err := template.ParseFiles(templatePath)
        if err != nil {
            return "", data, fmt.Errorf("error parsing template file: %v", err)
        }

        outFile, err := os.Create(outputPath)
        if err != nil {
            return "", data, fmt.Errorf("failed to create output file: %v", err)
        }
        defer outFile.Close()

        if err := tmpl.Execute(outFile, data); err != nil {
            return "", data, fmt.Errorf("error executing template: %v", err)
        }
    }

    // Log git add
    log.Printf("Running 'git add' on %s", outputPath)
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return "", data, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "add", outputPath); err != nil {
            return "", data, fmt.Errorf("git add failed: %v", err)
        }
    }

    // Log git commit
    commitMsg := fmt.Sprintf("Add event for %s based on template %s", dateStr, templateFile)
    log.Printf("Running 'git commit' with message: %q", commitMsg)
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return "", data, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "commit", "-m", commitMsg); err != nil {
            return "", data, fmt.Errorf("git commit failed: %v", err)
        }
    }

    // Log git push
    log.Println("Running 'git push'")
    if !dryRun {
        repoDir, err := os.Getwd()
        if err != nil {
            return "", data, fmt.Errorf("failed to get current working directory: %v", err)
        }

        if err := runGitCommand(repoDir, "push"); err != nil {
            return "", data, fmt.Errorf("git push failed: %v", err)
        }
    }

    return outputPath, data, nil
}

func publishEventOnFacebook(data EventData) error {
    // Placeholder function
    // TODO: Implement Facebook publishing
    log.Println("[Facebook] Publishing event not yet implemented.")
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
    outputPath, data, err := publishEventMarkdown(*templatePath, parsedDate, *dateStr, *lang, *dryRun)
    if err != nil {
        log.Fatalf("Error publishing event: %v", err)
    }

    // Attempt Facebook publishing only if requested and not dry-run
    if *publishFacebook {
        log.Println("Attempting to publish event on Facebook")
        if !*dryRun {
            if err := publishEventOnFacebook(data); err != nil {
                log.Fatalf("Failed to publish event on Facebook: %v", err)
            }
        }
    }

    log.Printf("Event published successfully: %s\n", outputPath)
}
