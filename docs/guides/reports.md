# PDF Reports

Octrafic can generate professional PDF reports from your test results.

## How it works

After running tests, simply ask the AI to generate a report:

```
generate a report from these tests
```

Octrafic will:
1. Collect all test results from the current session
2. Generate a structured Markdown report
3. Convert it to a styled PDF using WeasyPrint
4. Save it to `~/Documents/octrafic/`

## Requirements

PDF generation requires [WeasyPrint](https://weasyprint.org/). Follow the official installation guide for your platform:

[WeasyPrint Installation Guide](https://doc.courtbouillon.org/weasyprint/stable/first_steps.html)

If WeasyPrint is not installed, Octrafic will let you know when you try to generate a report.

## Output location

Reports are saved to:

```
~/Documents/octrafic/octrafic-report-2025-01-15_143025.pdf
```

The file name includes a timestamp so reports never overwrite each other.

## What's in the report

- Title and date
- Summary — total tests, passed, failed
- Results table — method, endpoint, status code, duration
- AI analysis and recommendations

## Example prompts

```
generate a report
```

```
create a PDF report for these test results
```

```
generate a report and name it users-api-tests.pdf
```

## Styling

Reports use the Octrafic brand theme — sky blue headers, styled tables, and dark code blocks. Each report includes page numbers and a footer with the generation timestamp.

## Related

- [Quick Start](../getting-started/quick-start.md) — Getting started with Octrafic
- [Authentication](./authentication.md) — Configuring auth for API tests
