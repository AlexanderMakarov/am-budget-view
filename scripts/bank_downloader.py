#!/usr/bin/env python3
"""Alternative CLI for semi-automatic bank statement downloads.

Primary path: in-app Files page (configure ``bankDownloads`` in config.yaml).

Alternative CLI: this script with scripts/bank_dowloader_config.yaml
(copy from bank_dowloader_config.yaml.template). Run via ``make bank-downloader``.
"""

import os
import argparse
import sys
import datetime
import getpass
import logging
import shlex
from pathlib import Path
import yaml

MY_FOLDER_PATH = os.path.dirname(os.path.abspath(__file__))
if MY_FOLDER_PATH not in sys.path:
    sys.path.insert(0, MY_FOLDER_PATH)

from bank_helpers_ameria import (
    AMERIABANK_ACCOUNT_TYPE_CARD,
    AMERIABANK_ACCOUNT_TYPE_SETTLEMENT,
    AmeriabankBusinessAccount,
    AmeriabankBusinessUnauthorized,
    MyAmeriaUnauthorized,
    download_ameriabank_business_statement_csv,
    download_myameria_history,
    fetch_ameriabank_business_accounts,
    refresh_ameriabank_business_cookie,
)
from bank_manual_ineco import InecoConfigError, print_manual_downloads

CONFIG_FILENAME = "bank_dowloader_config.yaml"

logger = logging.getLogger(__name__)


def config_path() -> str:
    return os.path.join(MY_FOLDER_PATH, CONFIG_FILENAME)


def prompt_credentials(config: dict) -> dict:
    """Collect per-run secrets while retaining the user's download settings."""
    if not sys.stdin.isatty():
        return config
    config = {key: dict(value) if isinstance(value, dict) else value
              for key, value in config.items()}
    try:
        if "my_ameria" in config:
            print(
                "MyAmeria: sign in at https://account.myameria.am.\n"
                "Open DevTools > Network, open History, and select a successful\n"
                "request to ob.myameria.am. Copy its Authorization request header."
            )
            config["my_ameria"]["auth_token"] = getpass.getpass(
                "MyAmeria Authorization (hidden; with or without Bearer): "
            )
            if not config["my_ameria"].get("client_id"):
                config["my_ameria"]["client_id"] = input(
                    "Client-Id from the same request: "
                ).strip()
        if "ameriabank" in config:
            print(
                "AmeriaBank Business: sign in at https://business.myameria.am.\n"
                "Open DevTools > Network and copy the Cookie request header\n"
                "from a request to gateway-businessmyameria.ameriabank.am."
            )
            config["ameriabank"]["cookie"] = getpass.getpass(
                "AmeriaBank Business Cookie (hidden): "
            )
    except (EOFError, KeyboardInterrupt):
        raise MyAmeriaUnauthorized("Bank authentication cancelled.") from None
    return config


def run_download(config: dict, base_dir=None) -> None:
    """Download transactions using a parsed bank_dowloader_config.yaml dict."""
    base_dir = os.fspath(base_dir) if base_dir is not None else MY_FOLDER_PATH
    to_date = datetime.datetime.now()
    if "my_ameria" in config:
        my_ameria = config["my_ameria"]
        my_ameria_history_path = os.path.join(
            base_dir, my_ameria["history_path"]
        )
        logger.info(
            "Downloading MyAmeria all accounts history into %s...",
            my_ameria_history_path,
        )
        def download_history(token: str, client_id: str) -> None:
            download_myameria_history(
                path=my_ameria_history_path,
                auth_token=token,
                from_date_str=my_ameria["since-DD-MM-YYYY"],
                to_date_str=to_date.strftime("%d-%m-%Y"),
                client_id=client_id,
            )

        try:
            download_history(my_ameria.get("auth_token", ""), my_ameria["client_id"])
        except MyAmeriaUnauthorized as exc:
            if not sys.stdin.isatty():
                raise MyAmeriaUnauthorized(
                    f"{exc} Update my_ameria.auth_token in scripts/{CONFIG_FILENAME} "
                    "or rerun make bank-downloader in a terminal to paste a fresh token."
                ) from None
            logger.warning("%s", exc)
            try:
                token = getpass.getpass("Fresh MyAmeria Authorization (hidden): ")
                client_id = input(
                    "Client-Id from the same session (Enter to keep configured value): "
                ).strip() or my_ameria["client_id"]
            except (EOFError, KeyboardInterrupt):
                raise MyAmeriaUnauthorized("MyAmeria authentication cancelled.") from None
            # Retry once, using credentials only in memory.
            download_history(token, client_id)
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
            output_dir = (
                os.path.join(base_dir, folder_path)
                if folder_path and not os.path.isabs(folder_path)
                else (folder_path if folder_path else base_dir)
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
                    output_dir,
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
                        os.path.normpath(os.path.join(base_dir, path_cfg))
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
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--config", metavar="PATH",
        help="App YAML whose inecobankStatementXmlFilesGlob selects existing statements "
             "(default: config.yaml in the working directory, if present).",
    )
    parser.add_argument(
        "--download-config", metavar="PATH",
        help="Downloader YAML with accounts/date settings (default: scripts/bank_dowloader_config.yaml). "
             "Relative statement/output paths resolve from that file's directory.",
    )
    parser.add_argument(
        "--manual-only", action="store_true",
        help="Check existing Inecobank XML statements and print missing downloads; no bank API calls.",
    )
    args = parser.parse_args()
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - %(message)s",
    )
    path = Path(args.download_config or config_path()).expanduser().resolve()
    if not os.path.isfile(path):
        print(
            f"Config not found: {path}\n"
            "Copy scripts/bank_dowloader_config.yaml.template to "
            f"scripts/{CONFIG_FILENAME}, fill in credentials, then run: make bank-downloader\n"
            "Or use the in-app Files page (bankDownloads in config.yaml).",
            file=sys.stderr,
        )
        sys.exit(1)
    try:
        with open(path, "r", encoding="utf-8") as f:
            config = yaml.safe_load(f) or {}
        if not isinstance(config, dict):
            raise InecoConfigError(f"Config must be a YAML mapping: {path}")
        app_path = Path(args.config or "config.yaml").expanduser().resolve()
        statement_glob = None
        if args.config or app_path.is_file():
            with app_path.open(encoding="utf-8") as f:
                app_config = yaml.safe_load(f) or {}
            if not isinstance(app_config, dict):
                raise InecoConfigError(f"App config must be a YAML mapping: {app_path}")
            if "inecobank" in config:
                statement_glob = app_config.get("inecobankStatementXmlFilesGlob")
                if not isinstance(statement_glob, str) or not statement_glob.strip():
                    raise InecoConfigError(
                        f"Set inecobankStatementXmlFilesGlob in {app_path} to scan XML statements. "
                        "The manual checklist does not count XLSX files."
                    )
        if "inecobank" in config:
            rerun = ["python3", "scripts/bank_downloader.py", "--manual-only"]
            if args.config:
                rerun.extend(["--config", args.config])
            if args.download_config:
                rerun.extend(["--download-config", args.download_config])
            print_manual_downloads(
                config["inecobank"], path.parent, statement_glob=statement_glob,
                rerun_command=shlex.join(rerun),
            )
        elif args.manual_only:
            raise InecoConfigError(f"Add an inecobank section to {path}.")
        if args.manual_only:
            return
        run_download(prompt_credentials(config), path.parent)
    except (MyAmeriaUnauthorized, InecoConfigError, OSError, yaml.YAMLError) as exc:
        logger.error("%s", exc)
        sys.exit(1)


if __name__ == "__main__":
    main()
