#!/usr/bin/env python3

import os
import sys
import datetime
import logging
import yaml

MY_FOLDER_PATH = os.path.dirname(os.path.abspath(__file__))
if MY_FOLDER_PATH not in sys.path:
    sys.path.insert(0, MY_FOLDER_PATH)

from bank_helpers_ameria import (
    AMERIABANK_ACCOUNT_TYPE_CARD,
    AMERIABANK_ACCOUNT_TYPE_SETTLEMENT,
    AmeriabankBusinessAccount,
    AmeriabankBusinessUnauthorized,
    download_ameriabank_business_statement_csv,
    download_myameria_history,
    fetch_ameriabank_business_accounts,
    refresh_ameriabank_business_cookie,
)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


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
            MY_FOLDER_PATH, my_ameria["history_path"]
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
    # Download statements for AmeriaBank via HTTP REST API (business.myameria.am).
    if "ameriabank" in config:
        ameriabank = config["ameriabank"]
        cookie = ameriabank.get("cookie", "")
        since_str = ameriabank.get("since-DD-MM-YYYY", "")
        folder_path = ameriabank.get("folder_path", "")
        config_accounts = ameriabank.get("accounts") or []

        if not cookie or not since_str:
            logger.warning("AmeriaBank (business): set cookie and since-DD-MM-YYYY in config.")
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
            # Cookie in a list so we can refresh on 401 and retry
            cookie_ref: list[str] = [cookie]

            def with_401_refresh(fn):
                try:
                    return fn(cookie_ref[0])
                except AmeriabankBusinessUnauthorized:
                    logger.info("AmeriaBank Business 401: refreshing cookie and retrying")
                    cookie_ref[0] = refresh_ameriabank_business_cookie(cookie_ref[0])
                    return fn(cookie_ref[0])

            settlement = with_401_refresh(
                lambda c: fetch_ameriabank_business_accounts(c, AMERIABANK_ACCOUNT_TYPE_SETTLEMENT)
            )
            card = with_401_refresh(
                lambda c: fetch_ameriabank_business_accounts(c, AMERIABANK_ACCOUNT_TYPE_CARD)
            )
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
                return os.path.join(base_dir, f"AccountStatement {acc.number} {sanitize(acc.name)} since {since_safe}.csv")

            def ensure_dir(p: str) -> None:
                d = os.path.dirname(os.path.abspath(p))
                if d:
                    os.makedirs(d, exist_ok=True)

            if not config_accounts:
                for acc in accounts:
                    out_path = out_path_for(acc)
                    ensure_dir(out_path)
                    logger.info("Downloading AmeriaBank statement for %s (%s)...", acc.number, acc.name)
                    with_401_refresh(
                        lambda c, a=acc: download_ameriabank_business_statement_csv(
                            c, a.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                        )
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
                    with_401_refresh(
                        lambda c, a=acc: download_ameriabank_business_statement_csv(
                            c, a.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                        )
                    )


if __name__ == "__main__":
    main()
