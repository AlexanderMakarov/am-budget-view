# Generic CSV

## Supported formats

### [FULL] Generic CSV

CSV files with transactions from any source.

- `config.yaml` setting: `genericCsvFilesGlob`
- Parsed by `generic_csv_parser.go`
- Supports all app features and Beancount reports
- Own account number and currency are deduced from the fields below

**Required headers** (first row):

| Header | Description |
|--------|-------------|
| `Date` | Transaction date in `YYYY-MM-DD` format |
| `FromAccount` | Sender account number |
| `ToAccount` | Receiver account number |
| `IsExpense` | `true` if expense (`FromAccount` is yours), `false` if income (`ToAccount` is yours) |
| `Amount` | Amount in account currency, dot + 2 decimals (e.g. `1500.30`) |
| `Details` | Transaction description (main categorization source) |
| `AccountCurrency` | 3-letter ISO currency code |
| `OriginCurrency` | *(optional)* Original currency before conversion |
| `OriginCurrencyAmount` | *(optional)* Amount in origin currency |

The MyAmeria CLI downloader (`make bank-downloader`) also produces Generic CSV files.

## Manual download

Create or export a CSV matching the headers above and place it next to the app (matching `genericCsvFilesGlob`).

## In-app download

Not applicable — Generic CSV is a custom/manual format.

## CLI download

MyAmeria semi-automatic download via `make bank-downloader` outputs Generic CSV (see [MyAmeria documentation](myameria.md)).
