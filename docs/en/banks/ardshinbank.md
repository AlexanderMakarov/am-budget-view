# Ardshinbank

## Supported formats

### [FULL] XLSX from email — recommended

Ardshinbank sends `.xlsx` files by email monthly or yearly. The file contains three sheets (English, Russian, Armenian); the app uses the **Armenian** sheet for more details.

- Receiver/sender account numbers appear to be inner Ardshinbank numbers only, which limits tracking transfers from other banks.
- Supports all app features and Beancount reports.
- `config.yaml` setting: `ardshinbankXlsxFilesGlob`
- Parsed by `ardshin_xlsx_parser.go`

### [NONE] XLSX from website

Files from https://ardshinbank.am/ — **not supported** (same as email files or with less data).

## Manual download

1. Save the XLSX statement from your Ardshinbank email
2. Place it next to the app executable (matching `ardshinbankXlsxFilesGlob`)

## In-app download

Not available for Ardshinbank.

## CLI download

Not available for Ardshinbank.
