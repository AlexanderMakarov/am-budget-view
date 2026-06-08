# AmeriaBank (Ameria for Business)

## Download in the app (recommended)

Use the [Files](/files) page to download CSV statements without editing `config.yaml` by hand.

### 1. Save non-secret settings

1. Open [Files](/files) → **Download settings**.
2. Under **AmeriaBank CSV statement**, set:
   - **Since date** — earliest transaction date to fetch (`DD-MM-YYYY`, e.g. `01-01-2024`).
   - **Output folder** — optional subfolder inside the app working directory (leave empty to save next to `config.yaml`).
3. Click **Save settings**. These values are stored in `config.yaml`.

### 2. Paste a fresh session cookie

1. Log in at [business.myameria.am](https://business.myameria.am) (OTP from the mobile app: Menu → Settings → OTP → Copy Code).
2. Open browser **DevTools** (F12) → **Network** tab.
3. Reload the page or open any account. Find a request to **gateway-businessmyameria.ameriabank.am**.
4. In **Request headers**, copy the full **Cookie** value. It must include `RefreshToken` (and usually `AccessToken` and `TS*` cookies). A short cookie with only `TS*` keys will fail with HTTP 401.
5. On [Files](/files), click **Download now** on the AmeriaBank row and paste the cookie into the popup.
6. Click **Download now**. New CSV files appear in the file list after a refresh.

**Important:** the cookie is sent only for that download request. It is **not saved** to `config.yaml` or the settings page. Copy a new cookie after logout or when the session expires.

[Full step-by-step with screenshots](/docs/bank/ameria-business) — see **How to copy the Cookie header** below.

## Supported formats

### [FULL] CSV (.csv) — recommended

Download per-account from [Ameria Internet Bank](https://online.ameriabank.am/InternetBank/MainForm.wgx): click an account → Statement, choose a period (use "FromDate" and "To" for custom ranges), enable **"Show equivalent in AMD"** (for exchange rates), then press **Export to CSV** (icon at top-right).

- Supports all app features and Beancount reports.
- `config.yaml` setting: `ameriaCsvFilesGlob`
- Parsed by `ameria_csv_parser.go`
- Also supports CSV files from https://business.myameria.am (new REST API site).

### [NONE] XML (.xml) from the website

Downloaded from the same place as CSV — **not supported** (missing own account number and currency).

### [NONE] XLSX (.xlsx) from email

Sent by AmeriaBank via email — **not supported** (no Receiver/Payer account numbers or exchange rates).

## Manual download from the website

1. Log in at https://online.ameriabank.am/InternetBank/MainForm.wgx (or https://business.myameria.am)
2. Open account → Statement, set date range and **Show equivalent in AMD**
3. Export to CSV
4. Place files next to the app (matching `ameriaCsvFilesGlob`)

## How to copy the Cookie header

1. Open [business.myameria.am](https://business.myameria.am) and sign in.
2. Press **F12** → **Network**.
3. Trigger any API call (reload, open Accounts, open a statement).
4. Click a request whose URL contains `gateway-businessmyameria.ameriabank.am`.
5. Scroll to **Request headers** → **Cookie** → copy the entire value (often 2000+ characters).
6. Paste into the **Download now** popup on the Files page.

If download fails with **401**, the cookie is incomplete or expired — log in again and copy a fresh Cookie that includes `RefreshToken`.

## CLI download (`make bank-downloader`) — advanced

Semi-automatic download via [bank_downloader.py](/scripts/bank_downloader.py) for users who prefer a terminal workflow:

1. Log in at https://business.myameria.am (OTP from mobile app).
2. Copy the **Cookie** header from DevTools (same steps as above).
3. In `scripts/bank_dowloader_config.yaml` set `ameriabank.cookie` and `ameriabank.since-DD-MM-YYYY`. Optionally set `folder_path` and `accounts`.
4. Run `make bank-downloader`.

Session cookies expire — re-copy after re-login. Helpers: [scripts/bank_helpers_ameria.py](/scripts/bank_helpers_ameria.py).

Credentials in the CLI config file are stored locally — do not share or commit them.
