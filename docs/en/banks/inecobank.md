# Inecobank

## Supported formats

### [FULL] XML (.xml) — recommended

Download per-account from [Inecobank online](https://online.inecobank.am/vcAccount/List): click an account, choose a date range, then use the download icon in the bottom-right corner.

- Supports all app features and Beancount reports.
- `config.yaml` setting: `inecobankStatementXmlFilesGlob`
- Parsed by `ineco_xml_parser.go`

### [NONE] Excel (.xls) from the website

Downloaded from the same page as XML above — **not supported**. Use XML instead.

### [PARTIAL] Excel (.xlsx) from email

Inecobank sends password-protected `.xlsx` files by email. They lack Receiver/Payer account numbers.

To use them, remove password protection first ([MS Office](https://support.microsoft.com/en-us/office/change-or-remove-workbook-passwords-1c17af87-25e2-4dc6-94f0-19ce21ad0b68), [LibreOffice](https://ask.libreoffice.org/t/remove-file-password-protection/30982)).

- `config.yaml` setting: `inecobankStatementXlsxFilesGlob`
- Parsed by `ineco_excel_parser.go`

## Manual download

1. Log in at https://online.inecobank.am/vcAccount/List
2. Open an account and select a date range
3. Download the XML statement file
4. Place the file next to the app executable (matching `inecobankStatementXmlFilesGlob`)

## In-app download

Not available for Inecobank.

## CLI download

`make bank-downloader` prints a manual download checklist for configured Inecobank accounts before asking for other banks' credentials. It does not automate Inecobank's UI or request an Inecobank cookie.

Add this section to `scripts/bank_dowloader_config.yaml`, replacing the example numbers with your own (keep the quotes):

```yaml
inecobank:
  folder_path: ".."  # Relative to scripts/, or an absolute directory.
  statement_glob: "*.xml"  # Within folder_path; may include subdirectories.
  since-DD-MM-YYYY: "01-04-2024"
  # until-DD-MM-YYYY: "31-12-2024"  # Defaults to today.
  accounts:
    - number: "0000000000000001"
      name: "AMD current account"
      type: "account"
    - number: "0000000000000002"
      name: "AMD card account"
      type: "card"
      since-DD-MM-YYYY: "01-06-2024"  # Optional per-account override.
```

The script reads `AccountNumber` and `Period` from existing XML statements. It combines overlapping or adjacent periods and prints each missing inclusive date range, account number, and a suggested `Statement_<account>_<from>_<to>.xml` filename. Periods in the middle of your history are checked too. Empty but valid statements count as coverage; the last transaction date is not used to infer coverage. Account-level `until-DD-MM-YYYY` can limit downloads for a closed account.

Malformed XML, HTML error pages, and missing or invalid metadata are reported and not counted. Suggested paths avoid overwriting existing files. Excel files are not counted. This checks declared statement coverage, not the completeness of individual transactions; an export through today only contains activity available at download time.

Download the listed XML files manually, put them in `folder_path`, and rerun just the checklist without any bank authentication:

```bash
python3 scripts/bank_downloader.py --manual-only
```

To select a different **app** configuration:

```bash
python3 scripts/bank_downloader.py --manual-only --config tmp-my.yaml
```

The CLI reads `inecobankStatementXmlFilesGlob` from that file (or `config.yaml` in the working directory by default, when present). This glob selects which files to scan; account numbers and coverage dates still come from XML contents. Relative app globs resolve from the working directory, as in the Go app. XLSX settings do not supply XML coverage.

Account lists and requested dates still come from the separate downloader YAML. Select another one with `--download-config path/to/downloads.yaml`; relative downloader paths resolve from its directory. `--config` does not import the app's `bankDownloads` settings into the Python downloader. If no app config is present, the downloader's `folder_path` and `statement_glob` are used as before. When an app config is selected, its XML glob takes precedence; an empty glob produces an explicit error instead of silently scanning unrelated files.

Repeat until there are no missing periods. This command only reads files and prints instructions; it does not download, rename, or overwrite statements. Missing downloads are a checklist, not a command failure. Invalid configuration exits with status 1. Ensure `inecobankStatementXmlFilesGlob` in the application's `config.yaml` matches the saved files.
