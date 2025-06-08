#!/usr/bin/env python3

import os
import time
import requests
import datetime
import logging
import yaml


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
    # Format dates as MM/DD/YYYY for MyAmeria API
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


def main():
    # Parse config from YAML file.
    config_path = os.path.join(MY_FOLDER_PATH, "bank_dowloader_config.yaml")
    with open(config_path, 'r') as f:
        config = yaml.safe_load(f)
    to_date = datetime.datetime.now()
    # Download statements for all accounts in MyAmeria.
    my_ameria = config["my_ameria"]
    my_ameria_accounts = my_ameria["accounts"]
    for account in my_ameria_accounts:
        logger.info("Downloading statement for %s...", account["name"])
        statement_path = os.path.join(MY_FOLDER_PATH, account["path"])
        download_myameria_statement(
            type=account["type"],
            account_number=account["account_number"],
            inner_account_number=account["inner_account_number"],
            client_id=my_ameria["client_id"],
            auth_token=my_ameria["auth_token"],
            from_date_str=account["since-DD-MM-YYYY"],
            to_date_str=to_date.strftime("%d-%m-%Y"),
            path=os.path.abspath(statement_path),
        )
    # TODO: Download statements for all accounts in Ameria Business.


if __name__ == "__main__":
    main()
