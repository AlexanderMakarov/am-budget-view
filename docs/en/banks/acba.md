# ACBA

## Supported formats

### [PARTIAL] XLS from ACBA Digital

Download from [ACBA Digital](https://acbadigital.am/dashboard). See also `docsdata/ACBA/ACBA get transactions.md` for screenshots.

1. Choose account or card → **Transactions**
2. Set date range, **Armenian** language, **Excel** format
3. Press **Download**

Armenian-language statements contain more information than English. Regular account statements have more data than card statements.

Because only some transactions (regular accounts only) include Receiver/Payer account numbers:

- Beancount reports cannot be built (app shows a terminal warning)
- Account-based categorization won't work for all transactions

- `config.yaml` settings: `acbaRegularAccountXlsFilesGlob`, `acbaCardXlsFilesGlob`
- Parsed by `acba_xls_stmt_regular_account_parser.go` and `acba_xls_stmt_card_parser.go`

**Tip:** do not request statements by email — files are password-protected.

## Manual download

1. Log in at https://acbadigital.am/dashboard
2. Select account/card → Transactions → configure export (Armenian, Excel, date range)
3. Download and place files next to the app

## In-app download

Not available for ACBA.

## CLI download

Not available for ACBA.
