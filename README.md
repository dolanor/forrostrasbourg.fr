# Publish Event Script

This Go script allows you to publish a new event entry from a template.
It automates the creation of a Markdown file in the `content/evenements` directory, then stages, commits, and pushes the changes to your Git repository.

## Features

- Takes an existing template file as input.
- Inserts a provided date into the template.
- Generates a new Markdown file named `[YYMMDD]-[basename].md` in `content/evenements`.
- Automatically runs `git add`, `git commit`, and `git push` to publish changes.

## Prerequisites

- Go installed (1.18+ recommended).
- A configured Git environment. Ensure you have write access to the repository and the correct SSH keys or tokens.
- A template file in `content/evenements/templates/` (or your chosen directory), for example: `okivu.md.template`.

## Usage

1. **Basic Command**

   ```bash
   go run scripts/publish-event.go \
       -template content/evenements/templates/okivu.md.template \
       -date "2024-11-27"
