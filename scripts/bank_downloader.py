#!/usr/bin/env python3
"""Alternative CLI for semi-automatic bank statement downloads.

Primary path: in-app Files page (configure ``bankDownloads`` in config.yaml).

Alternative CLI: this script with scripts/bank_dowloader_config.yaml
(copy from bank_dowloader_config.yaml.template). Run via ``make bank-downloader``.
"""

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

CONFIG_FILENAME = "bank_dowloader_config.yaml"

logger = logging.getLogger(__name__)


def config_path() -> str:
    return os.path.join(MY_FOLDER_PATH, CONFIG_FILENAME)


def run_download(config: dict) -> None:
    """Download transactions using a parsed bank_dowloader_config.yaml dict."""
    to_date = datetime.datetime.now()
    if "my_ameria" in config:
        my_ameria = config["my_ameria"]
        my_ameria_history_path = os.path.join(
            MY_FOLDER_PATH, my_ameria["history_path"]
        )
        logger.info(
            "Downloading MyAmeria all accounts history into %s...",
            my_ameria_history_path,
        )
        download_myameria_history(
            path=my_ameria_history_path,
            auth_token=my_ameria["auth_token"],
            from_date_str=my_ameria["since-DD-MM-YYYY"],
            to_date_str=to_date.strftime("%d-%m-%Y"),
            client_id=my_ameria["client_id"],
        )
    if "ameriabank" in config:
        ameriabank = config["ameriabank"]
        cookie = ameriabank.get("cookie", "")
        since_str = ameriabank.get("since-DD-MM-YYYY", "")
        folder_path = ameriabank.get("folder_path", "")
        config_accounts = ameriabank.get("accounts") or []

        if not cookie or not since_str:
            logger.warning(
                "AmeriaBank (business): set cookie and since-DD-MM-YYYY in config."
            )
        else:
            parts = since_str.replace("/", "-").strip().split("-")
            if len(parts) == 3:
                start_yyyy_mm_dd = f"{parts[2]}-{parts[1]}-{parts[0]}"
            else:
                start_yyyy_mm_dd = since_str
            end_yyyy_mm_dd = to_date.strftime("%Y-%m-%d")
            base_dir = (
                os.path.join(MY_FOLDER_PATH, folder_path)
                if folder_path and not os.path.isabs(folder_path)
                else (folder_path if folder_path else MY_FOLDER_PATH)
            )
            cookie_ref: list[str] = [cookie]

            def with_401_refresh(fn):
                try:
                    return fn(cookie_ref[0])
                except AmeriabankBusinessUnauthorized:
                    logger.info(
                        "AmeriaBank Business 401: refreshing cookie and retrying"
                    )
                    cookie_ref[0] = refresh_ameriabank_business_cookie(cookie_ref[0])
                    return fn(cookie_ref[0])

            settlement = with_401_refresh(
                lambda c: fetch_ameriabank_business_accounts(
                    c, AMERIABANK_ACCOUNT_TYPE_SETTLEMENT
                )
            )
            card = with_401_refresh(
                lambda c: fetch_ameriabank_business_accounts(
                    c, AMERIABANK_ACCOUNT_TYPE_CARD
                )
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
                return os.path.join(
                    base_dir,
                    f"AccountStatement {acc.number} {sanitize(acc.name)} since {since_safe}.csv",
                )

            def ensure_dir(p: str) -> None:
                d = os.path.dirname(os.path.abspath(p))
                if d:
                    os.makedirs(d, exist_ok=True)

            if not config_accounts:
                for acc in accounts:
                    out_path = out_path_for(acc)
                    ensure_dir(out_path)
                    logger.info(
                        "Downloading AmeriaBank statement for %s (%s)...",
                        acc.number,
                        acc.name,
                    )
                    with_401_refresh(
                        lambda c, a=acc: download_ameriabank_business_statement_csv(
                            c, a.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                        )
                    )
            else:
                for cfg in config_accounts:
                    number = (
                        cfg.get("number") or cfg.get("account_number") or ""
                    ).strip()
                    name = (cfg.get("name") or "").strip()
                    path_cfg = (cfg.get("path") or "").strip()
                    acc = next(
                        (
                            a
                            for a in accounts
                            if a.number == number or sanitize(a.name) == name
                        ),
                        None,
                    )
                    if acc is None:
                        logger.warning(
                            "AmeriaBank: account not found (number=%s, name=%s); skip.",
                            number or "?",
                            name or "?",
                        )
                        continue
                    out_path = (
                        os.path.normpath(os.path.join(MY_FOLDER_PATH, path_cfg))
                        if path_cfg and not os.path.isabs(path_cfg)
                        else (path_cfg if path_cfg else out_path_for(acc))
                    )
                    ensure_dir(out_path)
                    logger.info(
                        "Downloading AmeriaBank statement for %s to %s...",
                        acc.number,
                        out_path,
                    )
                    with_401_refresh(
                        lambda c, a=acc: download_ameriabank_business_statement_csv(
                            c, a.id, start_yyyy_mm_dd, end_yyyy_mm_dd, out_path
                        )
                    )


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - %(message)s",
    )
    path = config_path()
    if not os.path.isfile(path):
        print(
            f"Config not found: {path}\n"
            "Copy scripts/bank_dowloader_config.yaml.template to "
            f"scripts/{CONFIG_FILENAME}, fill in credentials, then run: make bank-downloader\n"
            "Or use the in-app Files page (bankDownloads in config.yaml).",
            file=sys.stderr,
        )
        sys.exit(1)
    with open(path, "r", encoding="utf-8") as f:
        config = yaml.safe_load(f) or {}
    run_download(config)


if __name__ == "__main__":
    main()
