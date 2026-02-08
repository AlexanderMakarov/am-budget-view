#!/usr/bin/env python3

import os
import sys
import time
import requests
import datetime
import logging
import csv
import yaml
import re

MY_FOLDER_PATH = os.path.dirname(os.path.abspath(__file__))
if MY_FOLDER_PATH not in sys.path:
    sys.path.insert(0, MY_FOLDER_PATH)

from bank_helpers import (
    AMERIABANK_ACCOUNT_TYPE_CARD,
    AMERIABANK_ACCOUNT_TYPE_SETTLEMENT,
    AmeriabankBusinessAccount,
    download_ameriabank_business_statement_csv,
    fetch_ameriabank_business_accounts,
)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


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
        error_msg = response.text
        logger.error(
            "MyAmeria server %s error on %s: %s",
            response.status_code,
            url,
            error_msg,
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


def get_inecobank_session_info(cookie: str) -> tuple[str | None, str | None]:
    """
    Get current DXScript and DXCss values from Inecobank session.
    
    Args:
        cookie: Cookie value from browser session
        
    Returns:
        Tuple of (dx_script, dx_css) values, or (None, None) if not found
    """
    headers = {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Accept-Language": "en,ru-RU;q=0.8,ru;q=0.5,en-US;q=0.3",
        "Accept-Encoding": "gzip, deflate, br, zstd",
        "Connection": "keep-alive",
        "Cookie": cookie,
        "Upgrade-Insecure-Requests": "1",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "none",
        "Sec-Fetch-User": "?1"
    }
    
    # Try to get the main page to extract DXScript and DXCss
    url = "https://online.inecobank.am/AccountStatement/Statement/50253381001"
    response = requests.get(url, headers=headers, timeout=30)
    
    if not response.ok:
        logger.error(f"Failed to get session info: {response.status_code}")
        return None, None
    
    # Look for DXScript and DXCss in the response
    content = response.text
    dx_script_match = re.search(r'DXScript["\']?\s*:\s*["\']([^"\']+)["\']', content)
    dx_css_match = re.search(r'DXCss["\']?\s*:\s*["\']([^"\']+)["\']', content)
    
    dx_script = dx_script_match.group(1) if dx_script_match else None
    dx_css = dx_css_match.group(1) if dx_css_match else None
    
    logger.info(f"Extracted DXScript: {dx_script[:50] if dx_script else 'None'}...")
    logger.info(f"Extracted DXCss: {dx_css[:50] if dx_css else 'None'}...")
    
    return dx_script, dx_css


def download_inecobank_statement(
    account_number: str,
    from_date_str: str,
    to_date_str: str,
    path: str,
    cookie: str,
    account_type: str = "account",
) -> None:
    """
    Download bank statement from Inecobank.

    Args:
        account_number: Bank account number (16 digits)
        from_date_str: Start date for statement in DD/MM/YYYY format.
        to_date_str: End date for statement in DD/MM/YYYY format.
        path: Path to save the statement file.
        cookie: Cookie value from browser session
        account_type: "account" or "card" to determine the endpoint
    """
    # Convert account number to internal ID (remove first 2 digits)
    internal_account_id = account_number[2:]
    # Determine base URL and endpoints based on account type
    if account_type == "card":
        base_url = "https://online.inecobank.am/CardStatement"
        callback_panel_url = f"{base_url}/_CardStatementCallbackPanel"
        operation_list_url = "https://online.inecobank.am/vcOperation/CardOperationList"
        export_url = f"{base_url}/Export"
        referer_url = f"{base_url}/Statement/{internal_account_id}"
    else:
        base_url = "https://online.inecobank.am/AccountStatement"
        callback_panel_url = f"{base_url}/_AccountStatementCallbackPanel"
        operation_list_url = "https://online.inecobank.am/vcOperation/OperationList"
        export_url = f"{base_url}/Export"
        referer_url = f"{base_url}/Statement/{internal_account_id}"

    # Get current DXScript and DXCss values from the session
    logger.info("Getting current session DXScript and DXCss values...")
    dx_script, dx_css = get_inecobank_session_info(cookie)
    
    if not dx_script or not dx_css:
        logger.warning("Could not extract DXScript/DXCss, using fallback values")
        # Fallback to static values if extraction fails
        dx_script = "1_171,1_94,1_164,1_114,1_121,1_98,1_125,1_113,14_33,1_91,1_156,1_154,1_106,14_1,1_105,14_0,14_22,1_120,1_93,14_2,1_104,1_138,14_13,14_5,1_116,1_152,1_101,14_7,1_103,1_102,14_8,1_169,1_170,1_124,14_9,1_163,1_162,1_147,14_32,1_157,1_166,1_139,1_97,1_141,1_142,14_15,1_155,1_143,1_144,14_16,14_17,1_126,14_11,1_146,1_149,14_20,1_160,1_158,1_153,1_161,14_25,1_165,14_28,14_31,1_100,5_5,5_4,4_11,4_10,4_6,4_7,4_9,14_14,4_12,4_13,4_14,1_110,1_112,1_137,14_12,1_117,1_107,14_3,1_108,1_109,1_122,1_145,1_119,14_18,14_19,1_118,14_29,1_123,1_136"
        dx_css = "1_12,0_5140,0_5136,1_10,0_5005,1_5,0_5007,0_5009,0_5011,0_5114,0_5110,0_5138,0_5012,4_2,0_5014,5_1,0_5092,/Content/Site.css??v=1.4.0"

    # Common headers for all requests
    headers = {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0",
        "Accept": "text/html, */*; q=0.01",
        "Accept-Language": "en,ru-RU;q=0.8,ru;q=0.5,en-US;q=0.3",
        "Accept-Encoding": "gzip, deflate, br, zstd",
        "Content-Type": "application/x-www-form-urlencoded",
        "X-Requested-With": "XMLHttpRequest",
        "Origin": "https://online.inecobank.am",
        "Connection": "keep-alive",
        "Cookie": cookie,
        "Sec-Fetch-Dest": "empty",
        "Sec-Fetch-Mode": "cors",
        "Sec-Fetch-Site": "same-origin",
        "TE": "trailers"
    }

    headers.update({
        "DXScript": dx_script,
        "DXCss": dx_css,
        "Referer": referer_url
    })

    # Step 1: Callback panel request
    logger.info(f"Making callback panel request for Inecobank account {account_number}")
    callback_data = {
        "DXCallbackName": f"cbp{account_type.capitalize()}Statement",
        "DXCallbackArgument": "c0:",
        "INECO_LIST_FILTER_CALLBACK": f"{from_date_str};{to_date_str};{internal_account_id}; "
    }
    response = requests.post(callback_panel_url, headers=headers, data=callback_data, timeout=30)
    if not response.ok:
        logger.error(f"Callback panel request failed: {response.status_code} - {response.text}")
        raise Exception(f"Callback panel request failed: {response.status_code}")

    # Step 2: Operation list request
    logger.info(f"Making operation list request for Inecobank account {account_number}")
    # URL encode the filter parameters
    from_date_encoded = from_date_str.replace("/", "%2F")
    to_date_encoded = to_date_str.replace("/", "%2F")
    operation_list_url_with_params = (
        f"{operation_list_url}?INECO_LIST_CALLBACK=1"
        f"&INECO_LIST_VIEW_ID=1"
        f"&INECO_LIST_BASE_FILTER=%28%22Date%22%20between%20TO_DATE%28%27{from_date_encoded}%2000%3A00%3A00%27%2C%27dd%2Fmm%2Fyyyy%20hh24%3Ami%3Ass%27%29%20and%20TO_DATE%28%27{to_date_encoded}%2023%3A59%3A59%27%2C%27dd%2Fmm%2Fyyyy%20hh24%3Ami%3Ass%27%29%29%20and%20%28%22account_id%22%3D{internal_account_id}%29"
        f"&ACCOUNT_STATEMENT_TOTAL_OUT=0"
        f"&ACCOUNT_STATEMENT_TOTAL_IN=0"
        f"&ACCOUNT_STATEMENT_CURRENCY=AMD"
    )
    operation_data = {
        "DXCallbackName": f"gv{account_type.capitalize()}Statement",
        "DXCallbackArgument": "c0:KV|0;[];GB|0;0|CUSTOMCALLBACK0|;",
        f"gv{account_type.capitalize()}Statement$DXSelInput": "",
        f"gv{account_type.capitalize()}Statement$DXKVInput": "[]",
        f"gv{account_type.capitalize()}Statement$CallbackState": "BwQHAQIFU3RhdGUHRgcFBwACAQcBAgEHAgIBBwMCAQcEAgEHAAcABwAHAAIABQAAAIAJAgJJRAcACQIAAgEDBwQCAAcAAgEHDgcAAgEHAAcABwACEEZpbHRlckV4cHJlc3Npb24HAgACClNob3dGb290ZXIKAgECCFBhZ2VTaXplAwcU",
        "DXMVCEditorsValues": "{}",
        "INECO_LIST_FILTER_CALLBACK": f"{from_date_str};{to_date_str};{internal_account_id}; "
    }
    response = requests.post(operation_list_url_with_params, headers=headers, data=operation_data, timeout=30)
    if not response.ok:
        logger.error(f"Operation list request failed: {response.status_code} - {response.text}")
        raise Exception(f"Operation list request failed: {response.status_code}")
    # Add a small delay to make requests more human-like
    time.sleep(3)

    # Step 3: Download XML file
    logger.info(f"Downloading XML statement for Inecobank account {account_number}")
    # Update headers for the export request
    export_headers = headers.copy()
    export_headers.update({
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "Upgrade-Insecure-Requests": "1",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-User": "?1",
        "Priority": "u=0, i"
    })
    # Prepare export data - match the exact format from the curl request
    export_filter = f"{from_date_str};{to_date_str};{internal_account_id};2"
    # URL encode the parameters as they appear in the curl request
    from_date_encoded = from_date_str.replace("/", "%2F")
    to_date_encoded = to_date_str.replace("/", "%2F")
    export_filter_encoded = f"{from_date_encoded};{to_date_encoded};{internal_account_id};2"
    export_data = {
        "export_filter": export_filter_encoded,
        "export_sorting": "",
        "DXScript": dx_script,
        "DXCss": dx_css,
        "DXMVCEditorsValues": f'{{"export_filter":"{export_filter_encoded}","export_sorting":null}}',
        "btnExportXml": "btnExportXml"
    }
    # Make the export request
    response = requests.post(export_url, headers=export_headers, data=export_data, stream=True, timeout=30)
    if not response.ok:
        logger.error(f"Export request failed: {response.status_code} - {response.text}")
        raise Exception(f"Export request failed: {response.status_code}")

    # Save the XML file
    with open(path, 'wb') as f:
        for chunk in response.iter_content(chunk_size=8192):
            if chunk:
                f.write(chunk)
    # Check if the downloaded file contains the rejection message
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
        if "Request Rejected" in content:
            logger.error(f"Downloaded file contains rejection message: {content[:200]}...")
            raise Exception("Bank rejected the request - file contains rejection message")
    logger.info(f"Successfully downloaded Inecobank account {account_number} statement to {path}")


def main():
    # Parse config from YAML file.
    config_path = os.path.join(MY_FOLDER_PATH, "bank_dowloader_config.yaml")
    with open(config_path, 'r') as f:
        config = yaml.safe_load(f)
    to_date = datetime.datetime.now()
    if "my_ameria" in config:
        # Download statements for all accounts in MyAmeria from "History" page.
        # Note that it saves them directly as generic CSV files to don't
        # add "myAmeriaMyAccounts" to config.yaml.
        my_ameria = config["my_ameria"]
        my_ameria_history_path = os.path.join(
            MY_FOLDER_PATH, my_ameria["folder_path"]
        )
        logger.info("Downloading MyAmeria all accounts history into %s...", my_ameria_history_path)
        download_myameria_history(
            path=my_ameria_history_path,
            auth_token=my_ameria["auth_token"],
            from_date_str=my_ameria["since-DD-MM-YYYY"],
            to_date_str=to_date.strftime("%d-%m-%Y"),
            client_id=my_ameria["client_id"],
        )
    # FYI: code below downloads per-account/card Excel files
    # but they contain too few info, data from "History" page is richer.
    # my_ameria_accounts = my_ameria["accounts"]
    # for account in my_ameria_accounts:
    #     logger.info("Downloading statement for %s...", account["name"])
    #     statement_path = os.path.join(MY_FOLDER_PATH, account["path"])
    #     download_myameria_statement(
    #         type=account["type"],
    #         account_number=account["account_number"],
    #         inner_account_number=account["inner_account_number"],
    #         client_id=my_ameria["client_id"],
    #         auth_token=my_ameria["auth_token"],
    #         from_date_str=account["since-DD-MM-YYYY"],
    #         to_date_str=to_date.strftime("%d-%m-%Y"),
    #         path=os.path.abspath(statement_path),
    #     )
    # Download statements for all accounts in Inecobank
    # if "inecobank" in config:
    #     inecobank = config["inecobank"]
    #     inecobank_accounts = inecobank["accounts"]
    #     for account in inecobank_accounts:
    #         logger.info("Downloading Inecobank statement for %s...", account["name"])
    #         statement_path = os.path.join(MY_FOLDER_PATH, account["path"])
    #         download_inecobank_statement(
    #             account_number=account["account_number"],
    #             from_date_str=account["since-DD/MM/YYYY"],
    #             to_date_str=to_date.strftime("%d/%m/%Y"),
    #             path=os.path.abspath(statement_path),
    #             cookie=inecobank["cookie"],
    #             account_type=account.get("type"),
    #         )
    # Download statements for AmeriaBank Business (business.myameria.am HTTP API).
    if "ameriabank" in config:
        ameriabank = config["ameriabank"]
        cookie = ameriabank.get("cookie", "")
        since_str = ameriabank.get("since-DD-MM-YYYY", "")
        folder_path = ameriabank.get("folder_path", "")
        config_accounts = ameriabank.get("accounts") or []
        if not cookie or not since_str:
            logger.warning("AmeriaBank: set cookie and since-DD-MM-YYYY in config (business.myameria.am).")
        else:
            # DD-MM-YYYY -> YYYY-MM-DD for API
            parts = since_str.replace("/", "-").strip().split("-")
            if len(parts) == 3:
                start_yyyy_mm_dd = f"{parts[2]}-{parts[1]}-{parts[0]}"
            else:
                start_yyyy_mm_dd = since_str
            end_yyyy_mm_dd = to_date.strftime("%Y-%m-%d")
            base_dir = (os.path.join(MY_FOLDER_PATH, folder_path) if folder_path and not os.path.isabs(folder_path)
                        else (folder_path if folder_path else MY_FOLDER_PATH))
            settlement = fetch_ameriabank_business_accounts(cookie, AMERIABANK_ACCOUNT_TYPE_SETTLEMENT)
            card = fetch_ameriabank_business_accounts(cookie, AMERIABANK_ACCOUNT_TYPE_CARD)
            accounts: list[AmeriabankBusinessAccount] = settlement + card
            logger.info(
                "AmeriaBank Business accounts (%d): %s",
                len(accounts),
                ", ".join(f"{a.number} ({a.name}, {a.sub_type})" for a in accounts),
            )
            since_safe = since_str.replace("/", "-")

            def sanitize(s: str) -> str:
                return (s or "").replace("/", "_").replace("\\", "_").strip() or "account"

            def out_path_for(acc: AmeriabankBusinessAccount) -> str:
                return os.path.join(base_dir, f"{acc.number}_{sanitize(acc.name)}_since_{since_safe}.csv")

            def ensure_dir(p: str) -> None:
                d = os.path.dirname(os.path.abspath(p))
                if d:
                    os.makedirs(d, exist_ok=True)

            if not config_accounts:
                for acc in accounts:
                    out_path = out_path_for(acc)
                    ensure_dir(out_path)
                    logger.info("Downloading AmeriaBank statement for %s (%s)...", acc.number, acc.name)
                    download_ameriabank_business_statement_csv(
                        cookie, acc.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                    )
            else:
                for cfg in config_accounts:
                    number = (cfg.get("number") or cfg.get("account_number") or "").strip()
                    name = (cfg.get("name") or "").strip()
                    path_cfg = (cfg.get("path") or "").strip()
                    acc = next(
                        (a for a in accounts if a.number == number or sanitize(a.name) == name),
                        None,
                    )
                    if acc is None:
                        logger.warning("AmeriaBank: account not found (number=%s, name=%s); skip.", number or "?", name or "?")
                        continue
                    out_path = (
                        os.path.normpath(os.path.join(MY_FOLDER_PATH, path_cfg))
                        if path_cfg and not os.path.isabs(path_cfg)
                        else (path_cfg if path_cfg else out_path_for(acc))
                    )
                    ensure_dir(out_path)
                    logger.info("Downloading AmeriaBank statement for %s to %s...", acc.number, out_path)
                    download_ameriabank_business_statement_csv(
                        cookie, acc.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                    )


if __name__ == "__main__":
    main()
