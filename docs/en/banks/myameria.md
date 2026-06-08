# MyAmeria (Ameria for Individuals)

## Download in the app (recommended)

Use the [Files](/files) page to download a Generic CSV via the bank API without editing `config.yaml` by hand.

### 1. Save non-secret settings

1. Open [Files](/files) → **Download settings**.
2. Under **MyAmeria generic CSV**, set:
   - **Client ID** — `Client-Id` request header from browser DevTools (see below).
   - **Since date** — earliest transaction date to fetch (`DD-MM-YYYY`, e.g. `01-01-2024`).
3. Click **Save settings**. These values are stored in `config.yaml`.

Also configure `myAmeriaMyAccounts` in `config.yaml` (account number → currency). The API export does not include currency per account.

### 2. Paste a fresh auth token

1. Log in at [account.myameria.am](https://account.myameria.am).
2. Open browser **DevTools** (F12) → **Network** tab.
3. Reload the page or open transaction history. Find a request to **ob.myameria.am** (or similar MyAmeria API host).
4. In **Request headers**, copy the full **Authorization** value (starts with `Bearer `). The token expires in about **15 minutes**.
5. On [Files](/files), click **Download now** on the MyAmeria row and paste the token into the popup.
6. Click **Download now**. A new CSV file appears in the file list after a refresh.

**Important:** the auth token is sent only for that download request. It is **not saved** to `config.yaml` or the settings page. Paste a new token each time it expires.

The in-app download produces a **Generic CSV** file (not a MyAmeria History Excel export).

## Supported formats

### [FULL] History Excel (.xls) — recommended (2025+)

Download from https://myameria.am/history: press **Filter** (right), set dates, then **Excel** in the Actions section.

- One file covers all accounts and cards.
- `config.yaml` setting: `myAmeriaHistoryXlsFilesGlob`
- Requires `myAmeriaMyAccounts` map (account number → currency) — the file does not include this data.
- Supports app features and Beancount reports except exchange rates (not in this file).
- Parsed by `ameria_history_parser.go`

### [OUTDATED] Account Statements Excel (.xls)

Downloaded from pages like https://myameria.am/cards-and-accounts/account-statement/****** before 2025. Did not work for cards, only accounts. Kept for backward compatibility with older files.

- `config.yaml` setting: `myAmeriaAccountStatementXlsxFilesGlob`
- Parsed by `ameria_stmt_parser.go`
- Since 2025, use History Excel instead.

### [NONE] Account/Card Statements CSV (2025+)

Downloaded from account/card statement pages in 2025+ — **not supported** (missing Receiver/Payer account numbers and native currency amounts).

## Manual download from the website

1. Log in at https://myameria.am/history
2. Filter by date range and export to Excel
3. Configure `myAmeriaMyAccounts` in `config.yaml` with your account numbers and currencies
4. Place the file next to the app (matching `myAmeriaHistoryXlsFilesGlob`)

## How to copy Client-Id and Authorization

1. Open [account.myameria.am](https://account.myameria.am) and sign in.
2. Press **F12** → **Network**.
3. Reload or open **History** so API requests appear.
4. Click a request to **ob.myameria.am** (or another `*.myameria.am` API URL).
5. Under **Request headers**:
   - **Client-Id** — copy into **Download settings** on the Files page (saved in config).
   - **Authorization** — copy the full value (`Bearer eyJ…`) into the **Download now** popup (not saved).

![How to copy Authorization header from browser DevTools](/static/docs/devtools-auth-header.png)

## CLI download (`make bank-downloader`) — advanced

Semi-automatic download via [bank_downloader.py](/scripts/bank_downloader.py):

1. Copy [scripts/bank_dowloader_config.yaml.template](/scripts/bank_dowloader_config.yaml.template) to `scripts/bank_dowloader_config.yaml`.
2. Log in at https://account.myameria.am, open DevTools → Network.
3. Copy **Client-Id** → `client_id`, **Authorization** → `auth_token` in the YAML file.
4. Set `since-DD-MM-YYYY` and optionally `history_path`.
5. Run `make bank-downloader`.

**Notes:**

1. The script uses the bank API and produces a **Generic CSV** file, not a MyAmeria History Excel file.
2. Update `auth_token` each time it expires (~15 minutes).

Credentials in the CLI config file are stored locally — do not share or commit them.
