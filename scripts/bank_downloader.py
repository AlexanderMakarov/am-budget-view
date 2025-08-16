#!/usr/bin/env python3

import os
import time
import requests
import datetime
import logging
import csv
import yaml
import json


MY_FOLDER_PATH = os.path.dirname(os.path.abspath(__file__))

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
            case "deposit" | "deposit:cash":
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


def download_ameriabank_statement(
    type: str,
    account_number: str,
    cookie: str,
    from_date_str: str,
    to_date_str: str,
    path: str,
) -> None:
    """
    Download bank statement from Ameria bank Business.

    Args:
        type: "card" or "account"
        account_number: Bank account number
        cookie: Cookie value
        from_date_str: Start date for statement in MM-MM-YYYY format.
        to_date_str: End date for statement in MM-MM-YYYY format.
        path: Path to save the statement file.
    """
    url = (
        "https://online.ameriabank.am/InternetBank/Route/"
        "2.1005212.80911/moz/en-US/AmeriaBank/983038.49148.414/0/"
        "AmeriaBank/Component.MainForm.0.551.ExportCsv.wgx"  # 551 here is changing.
        "?requestid=638849339623154138"  # Changes and looks like encodes account number.
        "&format=csv"
        "&encoding=utf-16"
    )
    headers = {
        "Content-Type": "application/json",
        "Cookie": cookie,
        "Accept": "text/csv",  # In browser 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8'
        "Referer": "https://online.ameriabank.am/InternetBank/MainForm.wgx",
        "Sec-Fetch-Dest": "document",
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


def main():
    # Parse config from YAML file.
    config_path = os.path.join(MY_FOLDER_PATH, "bank_dowloader_config.yaml")
    with open(config_path, 'r') as f:
        config = yaml.safe_load(f)
    to_date = datetime.datetime.now()
    # Download statements for all accounts in MyAmeria from "History" page.
    # Note that it saves them directly as generic CSV files to don't
    # add "myAmeriaMyAccounts" to config.yaml.
    my_ameria = config["my_ameria"]
    my_ameria_history_path = os.path.join(
        MY_FOLDER_PATH, my_ameria["history_path"]
    )
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
    # TODO: Download statements for all accounts in Ameria Business.


if __name__ == "__main__":
    main()
