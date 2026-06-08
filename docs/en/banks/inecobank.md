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

Not available for Inecobank.
