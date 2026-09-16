# Inecobank

## Supported formats

### [FULL] XML (.xml) — recommended

Download one statement per account from [Inecobank online](https://online.inecobank.am/vcAccount/List): open an account, choose a date range, then use the download icon in the bottom-right corner.

- Supports all app features and Beancount reports.
- `config.yaml` setting: `inecobankStatementXmlFilesGlob`
- Parsed by `ineco_xml_parser.go`

### [NONE] Excel (.xls) from the website

Downloaded from the same page as XML above — **not supported**. Use XML instead.

### [PARTIAL] Excel (.xlsx) from email

Inecobank sends password-protected `.xlsx` files by email. They lack Receiver/Payer account numbers. Remove password protection first ([MS Office](https://support.microsoft.com/en-us/office/change-or-remove-workbook-passwords-1c17af87-25e2-4dc6-94f0-19ce21ad0b68), [LibreOffice](https://ask.libreoffice.org/t/remove-file-password-protection/30982)).

- `config.yaml` setting: `inecobankStatementXlsxFilesGlob`
- Parsed by `ineco_excel_parser.go`

## Manual download

1. Log in at https://online.inecobank.am/vcAccount/List
2. Open an account and select a date range
3. Download the XML statement file
4. Save it where it matches `inecobankStatementXmlFilesGlob`

## In-app download

Direct download is not available for Inecobank. The CLI below checks existing XML coverage and tells you exactly which exports remain.

## Manual checklist through the CLI

Configure the XML glob and requested accounts in the same app configuration used by the Go application:

```yaml
inecobankStatementXmlFilesGlob: "Statement *.xml"

bankDownloads:
  inecobank:
    sinceDate: "01-04-2024"
    # untilDate: "31-12-2024"  # Optional; defaults to today.
    accounts:
      - number: "0000000000000001"  # Quote account numbers.
        name: "AMD current account"
        type: account
      - number: "0000000000000002"
        name: "AMD card account"
        type: card
        sinceDate: "01-06-2024"  # Optional per-account override.
        # untilDate: "31-12-2024"
```

Run the checklist with the default `config.yaml`:

```bash
python3 scripts/bank_downloader.py --manual-only
```

Select another app configuration such as `tmp-my.yaml` with:

```bash
python3 scripts/bank_downloader.py --manual-only --config tmp-my.yaml
```

For each account, the script reads `AccountNumber` and `Period` from matching XML statements, combines overlapping or adjacent periods, and prints every missing inclusive range. It checks gaps in the middle of the history too. Empty valid statements count as coverage; filenames and transaction dates do not determine coverage.

Malformed XML, HTML error pages, and files with missing metadata are reported and not counted. Excel files are not counted. Suggested paths avoid overwriting existing files and are chosen to match `inecobankStatementXmlFilesGlob` where possible.

Download the listed XML files manually and rerun the command until it reports no missing periods. The command only reads files and prints instructions; it does not download, rename, or overwrite statements. An export through today contains only activity available when it was downloaded.

Existing setups may keep the legacy `inecobank` section in `scripts/bank_dowloader_config.yaml`; the app configuration above takes precedence. `--download-config path/to/downloads.yaml` is still available for that compatibility format and for MyAmeria/AmeriaBank CLI settings.
