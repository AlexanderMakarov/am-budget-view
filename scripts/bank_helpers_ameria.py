"""
Ameria-related helpers: MyAmeria (retail ob.myameria.am) and AmeriaBank Business (business.myameria.am).
AmeriaBank Business uses HTTP REST API (gateway-businessmyameria.ameriabank.am) only.
Legacy DXScript/WebGUI helpers for online.ameriabank.am are in bank_helpers_dxscript.py.
"""

import csv
import datetime
import json
import logging
import time
from dataclasses import dataclass

import requests

logger = logging.getLogger(__name__)


# --- AmeriaBank Business API (business.myameria.am) ---

AMERIABANK_BUSINESS_API_BASE = "https://gateway-businessmyameria.ameriabank.am/api/v1"
AMERIABANK_BUSINESS_ORIGIN = "https://business.myameria.am"

# Account types for list endpoint
AMERIABANK_ACCOUNT_TYPE_SETTLEMENT = "SettlementAccount"
AMERIABANK_ACCOUNT_TYPE_CARD = "CardAccount"


class AmeriabankBusinessUnauthorized(Exception):
    """Raised when gateway returns 401; caller may refresh cookie and retry."""

    def __init__(self, message: str, response_body: str = ""):
        super().__init__(message)
        self.response_body = response_body


@dataclass
class AmeriabankBusinessAccount:
    """One account from MyAmeria Business API (JSON)."""

    id: int
    number: str
    currency: str
    name: str
    sub_type: str  # SettlementAccount | CardAccount
    balance: float
    account_status: str
    description: str = ""
    closing_date: str | None = None


def _ameriabank_business_headers(cookie: str) -> dict[str, str]:
    """Headers for gateway-businessmyameria.ameriabank.am (Cookie-based auth)."""
    return {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0",
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "en-US",
        "Accept-Encoding": "gzip, deflate, br, zstd",
        "Referer": f"{AMERIABANK_BUSINESS_ORIGIN}/",
        "Origin": AMERIABANK_BUSINESS_ORIGIN,
        "Model": "firefox",
        "DeviceOS": "Linux",
        "DeviceId": "MyAmeriaBusiness Web",
        "DeviceOSVersion": "147.0.0",
        "Platform": "Web",
        "Connection": "keep-alive",
        "Cookie": _latin1_safe(cookie),
        "Sec-Fetch-Dest": "empty",
        "Sec-Fetch-Mode": "cors",
        "Sec-Fetch-Site": "cross-site",
    }


def refresh_ameriabank_business_cookie(cookie: str, *, timeout: int = 60) -> str:
    """
    POST Authentication/Refresh with current cookie; return new cookie from Set-Cookie.
    Cookie must include RefreshToken and TS* (AccessToken can be expired).
    """
    url = f"{AMERIABANK_BUSINESS_API_BASE}/Authentication/Refresh"
    headers = {
        **_ameriabank_business_headers(cookie),
        "Content-Type": "application/json",
        "Sec-Fetch-Storage-Access": "none",
    }
    logger.info("AmeriaBank Business POST %s", url)
    resp = requests.post(url, headers=headers, data="{}", timeout=timeout)
    if resp.status_code != 200:
        body = (resp.text or "")[:500]
        logger.error("AmeriaBank Business Refresh failed %s. Response: %s", resp.status_code, body)
        if resp.status_code == 401:
            raise ValueError(
                "AmeriaBank Business Refresh returned 401 — RefreshToken expired or invalid. "
                "Log in again at https://business.myameria.am and copy a fresh cookie from a request to "
                "gateway-businessmyameria.ameriabank.am. See log above for response body."
            ) from None
        raise ValueError(
            f"AmeriaBank Business Refresh failed {resp.status_code}. See log for response body."
        ) from None
    # Parse Set-Cookie: may be multiple headers or one merged with ", "
    cookies_dict: dict[str, str] = {}

    def parse_set_cookie_value(val: str) -> None:
        # One header value can be "Name=value; path=/; ..." or merged "Name1=val1; ..., Name2=val2; ..."
        for block in val.split(","):
            block = block.strip()
            part = block.split(";")[0].strip()
            if "=" in part:
                name, value = part.split("=", 1)
                name, value = name.strip(), value.strip()
                if name:
                    cookies_dict[name] = value

    set_cookie_vals: list[str]
    if hasattr(resp.raw, "headers") and hasattr(resp.raw.headers, "getlist"):
        raw = resp.raw.headers.getlist("Set-Cookie") or resp.raw.headers.getlist("set-cookie")
        set_cookie_vals = list(raw) if raw else []  # type: ignore[arg-type]
    else:
        set_cookie_vals = []
    if not set_cookie_vals:
        single = resp.headers.get("Set-Cookie") or resp.headers.get("set-cookie") or ""
        if single:
            set_cookie_vals = [single]
    for val in set_cookie_vals:
        parse_set_cookie_value(val)
    if not cookies_dict:
        logger.error("AmeriaBank Business Refresh 200 but no Set-Cookie in response")
        raise ValueError("AmeriaBank Business Refresh returned no Set-Cookie; cannot update cookie.")
    new_cookie = "; ".join(f"{k}={v}" for k, v in cookies_dict.items())
    logger.info("AmeriaBank Business cookie refreshed (keys: %s)", list(cookies_dict.keys()))
    return new_cookie


def fetch_ameriabank_business_accounts(
    cookie: str,
    account_type: str,
    *,
    page_index: int = 1,
    page_size: int = 100,
    timeout: int = 60,
) -> list[AmeriabankBusinessAccount]:
    """
    Fetch account list from MyAmeria Business API.
    account_type: AMERIABANK_ACCOUNT_TYPE_SETTLEMENT or AMERIABANK_ACCOUNT_TYPE_CARD.
    """
    url = (
        f"{AMERIABANK_BUSINESS_API_BASE}/Accounts"
        f"?pageIndex={page_index}&pageSize={page_size}"
        f"&accountType={account_type}&accountStatus=Open"
    )
    headers = _ameriabank_business_headers(cookie)
    logger.info("AmeriaBank Business GET Accounts (%s)", account_type)
    resp = requests.get(url, headers=headers, timeout=timeout)
    if resp.status_code == 401:
        body = (resp.text or "")[:500]
        logger.error("AmeriaBank Business 401 Unauthorized. Response: %s", body)
        raise AmeriabankBusinessUnauthorized(
            "AmeriaBank Business API 401 Unauthorized. Refresh cookie and retry.",
            response_body=body,
        ) from None
    resp.raise_for_status()
    data = resp.json()
    accounts = []
    for item in data:
        accounts.append(
            AmeriabankBusinessAccount(
                id=int(item["id"]),
                number=str(item["number"]),
                currency=str(item["currency"]),
                name=str(item.get("name", item.get("description", ""))),
                sub_type=str(item.get("subType", account_type)),
                balance=float(item.get("balance", 0)),
                account_status=str(item.get("accountStatus", "")),
                description=str(item.get("description", "")),
                closing_date=item.get("closingDate"),
            )
        )
    logger.info("AmeriaBank Business: %d %s accounts", len(accounts), account_type)
    return accounts


def download_ameriabank_business_statement_csv(
    cookie: str,
    account_id: int,
    start_date_yyyy_mm_dd: str,
    end_date_yyyy_mm_dd: str,
    path: str,
    *,
    timeout: int = 60,
) -> None:
    """
    Download statement CSV for one account from MyAmeria Business API.
    start_date_yyyy_mm_dd / end_date_yyyy_mm_dd: YYYY-MM-DD (API format).
    Uses withAmd=true to get Debit(AMD)/Credit(AMD) columns but it doesn't work.
    """
    url = (
        f"{AMERIABANK_BUSINESS_API_BASE}/Accounts/{account_id}/Statements/Export"
        f"?exportFormat=Csv"
        f"&startDate={start_date_yyyy_mm_dd}&endDate={end_date_yyyy_mm_dd}"
        "&withAmd=true"
    )
    headers = _ameriabank_business_headers(cookie)
    headers["Accept"] = "text/csv, application/csv, application/json, */*"
    logger.info("AmeriaBank Business GET Export %s", url)
    resp = requests.get(url, headers=headers, timeout=timeout)
    if resp.status_code == 401:
        body = (resp.text or "")[:500]
        logger.error("AmeriaBank Business 401 Unauthorized on Export. Response: %s", body)
        raise AmeriabankBusinessUnauthorized(
            "AmeriaBank Business API 401 Unauthorized on Export.",
            response_body=body,
        ) from None
    resp.raise_for_status()
    raw = resp.content
    # Response may be CSV or JSON-wrapped; try decode as UTF-8
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        text = raw.decode("utf-8", errors="replace")
    # If server returns JSON with CSV inside (e.g. {"data": "..."}), unwrap
    if text.strip().startswith("{"):
        try:
            obj = json.loads(text)
            if isinstance(obj, dict) and "data" in obj:
                text = obj["data"]
            elif isinstance(obj, str):
                text = obj
        except Exception:
            pass
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    logger.info("Downloaded AmeriaBank Business statement to %s", path)


def _latin1_safe(s: str) -> str:
    """Ensure string is valid for HTTP header value (latin-1). Drops chars that can't be encoded."""
    if not s:
        return s
    return s.encode("latin-1", errors="ignore").decode("latin-1")


# --- MyAmeria retail (ob.myameria.am) ---


def download_myameria_statement(
    type: str,
    account_number: str,
    inner_account_number: str,
    client_id: str,
    auth_token: str,
    from_date_str: str,
    to_date_str: str,
    path: str,
) -> None:
    """
    Download bank statement from MyAmeria bank.

    Args:
        type: "card" or "account"
        account_number: Bank account number
        inner_account_number: Inner account number
        client_id: Client ID
        auth_token: Authorization token
        from_date_str: Start date for statement in MM-MM-YYYY format.
        to_date_str: End date for statement in MM-MM-YYYY format.
        path: Path to save the statement file.
    """
    now = datetime.datetime.now()
    # Convert DD-MM-YYYY to DD/MM/YYYY and encode
    url = (
        f"https://ob.myameria.am/api/statement/{type}/{inner_account_number}"
        f"?withEquivalentCurrency=true"
        f"&withDailyMovement=false"
        f"&withOverdraft=false"
        f"&dateFrom={from_date_str.replace('-', '%2F')}"
        f"&dateTo={to_date_str.replace('-', '%2F')}"
        f"&accountNumber={account_number}"
        f"&fileType=xls"
    )
    headers = {
        "Content-Type": "application/json",
        "Authorization": auth_token,
        "Client-Time": now.strftime("%H:%M:%S"),
        "Client-Id": client_id,
        "Locale": "en",
        "Timezone-Offset": str(-int(time.timezone / 60))
    }
    # Make the request with streaming enabled to handle chunked transfer
    response = requests.get(url, headers=headers, stream=True, timeout=30)
    if not response.ok:
        error_msg = response.text
        logger.error(
            "MyAmeria server %s error on %s: %s",
            response.status_code,
            url,
            error_msg,
        )
    # Accumulate all chunks in memory.
    chunks = []
    for chunk in response.iter_content(chunk_size=None):
        if chunk:  # filter out keep-alive chunks
            chunks.append(chunk)
    # Write all accumulated data to file.
    with open(path, 'wb') as f:
        f.write(b''.join(chunks))
    logger.info(f"Successfully downloaded statement to {path}")


def convert_myameria_history_entries(
    entries: list[dict]
) -> dict[tuple[str, str], list[dict]]:
    """Parses MyAmeria history JSON for my accounts and their currencies.

    Args:
        entries: List of entries from MyAmeria history HTTP response.

    Returns:
        Dictionary where key is tuple of account number and currency,
        and value is list of transactions for this account.
    """
    result = {}
    my_accounts: set[tuple[str, str]] = set()
    logger.info("Parsing %d MyAmeria History entries...", len(entries))
    # Process entries in reverse order to find account ownership patterns
    for entry in reversed(entries):
        transaction_type = entry["transactionType"]
        accounting_type = entry["accountingType"]
        currency = entry["amount"]["currency"]
        debit_account = entry["debitAccountNumber"]
        credit_account = entry["creditAccountNumber"]
        # Identify my accounts based on transaction types
        match transaction_type:
            case "transfer:between-own-accounts" | "transfer:local":
                # Transfer or exchange of currencies between my accounts.
                if accounting_type == "DEBIT":
                    my_accounts.add((debit_account, currency))
                else:
                    my_accounts.add((credit_account, currency))
            case "exchange":
                # Exchange of currencies between my accounts.
                if accounting_type == "DEBIT":
                    my_accounts.add((debit_account, currency))
                else:
                    my_accounts.add((credit_account, currency))
            case ("card" | "transfer:to-card" | "transfer:international" |
                  "charge:commission:transfer" | "charge:commission" |
                  "charge:international" | "cash-out"):
                # Expense or refund from/to my account.
                if accounting_type == "DEBIT":
                    my_accounts.add((debit_account, currency))
                else:
                    my_accounts.add((credit_account, currency))
            case "deposit" | "deposit:cash" | "deposit:replenishment":
                # Income to my account via ATM or bank branch.
                my_accounts.add((credit_account, currency))
            case _:
                raise ValueError(
                    f"Unknown transaction type: {transaction_type}"
                )
    # Log discovered accounts.
    logger.info(
        "Discovered %d my accounts:\n  %s",
        len(my_accounts),
        "\n  ".join(
            f"{account} ({currency})"
            for account, currency in sorted(my_accounts)
        )
    )
    # Extract just account numbers from my_accounts
    my_account_numbers = {account for account, _ in my_accounts}
    # Now group transactions by account number only (not by currency).
    account_transactions = {}
    for entry in entries:
        debit_account = entry["debitAccountNumber"]
        credit_account = entry["creditAccountNumber"]
        accounting_type = entry["accountingType"]
        # Check if this transaction involves any of my accounts.
        transaction_assigned = False
        # Check if it's a debit from my account.
        if accounting_type == "DEBIT" and debit_account in my_account_numbers:
            account_transactions.setdefault(debit_account, []).append(entry)
            transaction_assigned = True
        # Check if it's a credit to my account.
        if accounting_type == "CREDIT" and credit_account in my_account_numbers:
            account_transactions.setdefault(credit_account, []).append(entry)
            transaction_assigned = True
        # Fail if transaction doesn't belong to any of my accounts.
        if not transaction_assigned:
            raise ValueError(
                f"Transaction {entry['id']} doesn't belong "
                f"to any of my accounts"
            )
    # Use currency from my_accounts set for each account.
    for account, transactions in account_transactions.items():
        # Find the currency for this account from my_accounts.
        account_currencies = {x for acc, x in my_accounts if acc == account}
        if not account_currencies or len(account_currencies) != 1:
            raise ValueError(f"Could not find currency for account {account}")
        account_currency = account_currencies.pop()
        result[(account, account_currency)] = transactions
    # Log statistics.
    account_stats = [
        (account, currency, len(transactions))
        for (account, currency), transactions in result.items()
    ]
    account_stats.sort()
    logger.info(
        "Transaction statistics:\n  %s",
        "\n  ".join(
            f"{account} ({currency}) - {n} transactions"
            for account, currency, n in account_stats
        )
    )
    # Check transactions are not duplicated between my accounts.
    total_transactions = sum(len(x) for x in result.values())
    if total_transactions != len(entries):
        raise ValueError(
            "Transactions are duplicated between my accounts: "
            + f"{total_transactions} != {len(entries)}"
        )
    return result


def download_myameria_history(
    path: str,
    auth_token: str,
    from_date_str: str,
    to_date_str: str,
    client_id: str,
) -> None:
    now = datetime.datetime.now()
    logger.info(f"Downloading MyAmeria history from {from_date_str} to {to_date_str}")
    url = (
        f"https://ob.myameria.am/api/events/past"
        f"?locale=en"
        f"&toAmount=10000000000"
        f"&fromDate={from_date_str.replace('-', '%2F')}"
        f"&toDate={to_date_str.replace('-', '%2F')}"
        f"&sort=date"
        f"&size=10000"  # Ask all.
        f"&page=1"
    )
    headers = {
        "Content-Type": "application/json",
        "Authorization": auth_token,
        "Client-Time": now.strftime("%H:%M:%S"),
        "Client-Id": client_id,
        "Locale": "en",
        "Timezone-Offset": str(-int(time.timezone / 60))
    }
    response = requests.get(url, headers=headers, stream=True, timeout=30)
    if not response.ok:
        error_msg = (response.text or "")[:500]
        logger.error(
            "MyAmeria server %s error on %s: %s",
            response.status_code,
            url,
            error_msg,
        )
        raise ValueError(
            f"MyAmeria API returned HTTP {response.status_code}. "
            f"Response: {error_msg or '(empty)'}"
        )
    # Parse JSON response and group transactions by account.
    data = response.json()['data']
    # FIY: debug
    # json.dump(data, open('scripts/my_ameria_history.json', 'w'), indent=2)
    # data = json.load(open('scripts/my_ameria_history.json', 'r'))
    accounts_with_transactions = convert_myameria_history_entries(
        data["entries"]
    )
    # Write CSV with proper headers compatible with generic_csv_parser.go
    with open(path, 'w', newline='', encoding='utf-8') as f:
        writer = csv.writer(f)
        writer.writerow([
            "Date",
            "FromAccount",
            "ToAccount",
            "IsExpense",
            "Amount",
            "Details",
            "AccountCurrency",
            "OriginCurrency",
            "OriginCurrencyAmount"
        ])
        # Process each entry from accounts_with_transactions
        for (_, native_currency), transactions in (
            accounts_with_transactions.items()
        ):
            for i, entry in enumerate(transactions):
                # Parse operation date from ISO format to YYYY-MM-DD
                operation_date = datetime.datetime.fromisoformat(
                    entry["operationDate"].replace('Z', '+00:00')
                ).strftime("%Y-%m-%d")
                # Determine if it's an expense based on accounting type
                is_expense = entry["accountingType"] == "DEBIT"
                # Set FromAccount and ToAccount based on expense/income
                debit_account = entry["debitAccountNumber"]
                credit_account = entry["creditAccountNumber"]
                if is_expense:
                    # Money going out of my account
                    from_account = debit_account
                    to_account = credit_account
                else:
                    # Money coming into my account
                    from_account = debit_account
                    to_account = credit_account
                # Get transaction amount and currency
                transaction_currency = entry["amount"]["currency"]
                transaction_amount = entry["amount"]["amount"]
                if transaction_currency == native_currency:
                    # Transaction is in account's native currency - use main fields
                    account_amount = transaction_amount
                    account_currency = native_currency
                    origin_currency = ""
                    origin_amount = ""
                else:
                    # Transaction is in different currency - use origin fields
                    account_amount = 0.0
                    account_currency = native_currency
                    origin_currency = transaction_currency
                    origin_amount = f"{transaction_amount:.2f}"
                if account_amount <= 0:
                    raise ValueError(f"{i} line: wrong amount '{account_amount}' for {entry}")
                writer.writerow([
                    operation_date,
                    from_account,
                    to_account,
                    str(is_expense).lower(),  # Convert boolean to lowercase
                    f"{account_amount:.2f}",  # Format amount with 2 decimal places
                    entry["details"],
                    account_currency,
                    origin_currency,
                    origin_amount
                ])
    logger.info(f"Successfully downloaded history to {path}")
