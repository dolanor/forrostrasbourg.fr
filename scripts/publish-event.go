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

func main() {
    date := flag.String("date", "", "Event date in YYYY-MM-DD format")
    templatePath := flag.String("template", "", "Path to the template markdown file (e.g. pachamamas.md.template)")
    lang := flag.String("lang", "fr", "Language code for date formatting (e.g. 'fr' or 'en')")
    flag.Parse()

    if *date == "" {
        log.Fatal("You must provide a -date parameter.")
    }
    if *templatePath == "" {
        log.Fatal("You must provide a -template parameter.")
    }

    parsedDate, err := time.Parse("2006-01-02", *date)
    if err != nil {
        log.Fatalf("Invalid date format: %v", err)
    }

    // Convert date to YYMMDD format
    formattedDate := parsedDate.Format("060102") // "06"=YY, "01"=MM, "02"=DD

    // Determine the base filename from the template
    templateFile := filepath.Base(*templatePath)
    baseName := strings.TrimSuffix(templateFile, ".template") // e.g. "pachamamas.md"
    baseName = strings.TrimSuffix(baseName, ".md")

    // Construct the output filename: e.g. "241129-pachamamas.md"
    outputFilename := fmt.Sprintf("%s-%s.md", formattedDate, baseName)

    // Prepare directories
    outputDir := "content/evenements"
    if _, err := os.Stat(outputDir); os.IsNotExist(err) {
        if err := os.MkdirAll(outputDir, 0755); err != nil {
            log.Fatalf("Failed to create output directory: %v", err)
        }
    }
    outputPath := filepath.Join(outputDir, outputFilename)

    // Parse the template
    tmpl, err := template.ParseFiles(*templatePath)
    if err != nil {
        log.Fatalf("Error parsing template file: %v", err)
    }

    // Get localized weekday and month
    weekdayLower := getWeekdayName(parsedDate, *lang)
    monthLower := getMonthName(parsedDate, *lang)
    day := parsedDate.Day()

    longDate := fmt.Sprintf("%s %d %s", weekdayLower, day, monthLower)
    longDateCapitalized := fmt.Sprintf("%s %d %s", capitalizeFirstLetter(weekdayLower), day, monthLower)

    data := EventData{
        Date:                *date,
        LongDate:            longDate,
        LongDateCapitalized: longDateCapitalized,
    }

    outFile, err := os.Create(outputPath)
    if err != nil {
        log.Fatalf("Failed to create output file: %v", err)
    }
    defer outFile.Close()

    if err := tmpl.Execute(outFile, data); err != nil {
        log.Fatalf("Error executing template: %v", err)
    }

    // Git operations
    repoDir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Failed to get current working directory: %v", err)
    }

    if err := runGitCommand(repoDir, "add", outputPath); err != nil {
        log.Fatalf("git add failed: %v", err)
    }

    commitMsg := fmt.Sprintf("Add event for %s based on template %s", *date, templateFile)
    if err := runGitCommand(repoDir, "commit", "-m", commitMsg); err != nil {
        log.Fatalf("git commit failed: %v", err)
    }

    if err := runGitCommand(repoDir, "push"); err != nil {
        log.Fatalf("git push failed: %v", err)
    }

    log.Printf("Event published successfully: %s\n", outputPath)
}
