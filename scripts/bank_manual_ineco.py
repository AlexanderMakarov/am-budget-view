"""Plan manual Inecobank XML exports using account and period metadata."""

import datetime as dt
import re
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from pathlib import Path


class InecoConfigError(ValueError):
    """Invalid manual-download settings."""


@dataclass(frozen=True)
class MissingStatement:
    account: str
    name: str
    account_type: str
    start: dt.date
    end: dt.date
    path: Path


def config_date(value, field: str) -> dt.date:
    try:
        return dt.datetime.strptime(value, "%d-%m-%Y").date()
    except (TypeError, ValueError):
        raise InecoConfigError(f"Inecobank {field} must be a date in DD-MM-YYYY format.") from None


def statement_period(path: Path) -> tuple[str, dt.date, dt.date]:
    """Read declared coverage, including statements with no transactions."""
    root = ET.parse(path).getroot()
    # Accept namespace-qualified exports too.
    for element in root.iter():
        element.tag = element.tag.rsplit("}", 1)[-1]
    if root.tag != "Statement" or root.find("Operations") is None:
        raise ValueError("not an Inecobank Statement with Operations")
    account = (root.findtext("AccountNumber") or "").strip()
    if not account.isascii() or not account.isdigit():
        raise ValueError("missing or invalid AccountNumber")
    period = (root.findtext("Period") or "").strip()
    match = re.fullmatch(r"\[?\s*(\d{2}/\d{2}/\d{4})\s*-\s*(\d{2}/\d{2}/\d{4})\s*\]?", period)
    if not match:
        raise ValueError("missing or unsupported Period; expected [DD/MM/YYYY - DD/MM/YYYY]")
    try:
        start, end = (dt.datetime.strptime(value, "%d/%m/%Y").date() for value in match.groups())
    except ValueError:
        raise ValueError("invalid date in Period") from None
    if start > end:
        raise ValueError("Period ends before it starts")
    return account, start, end


def missing_periods(start: dt.date, end: dt.date, periods):
    """Subtract the union of inclusive covered periods from the requested range."""
    cursor = start
    day = dt.timedelta(days=1)
    for covered_start, covered_end in sorted(periods):
        if covered_end < cursor:
            continue
        if covered_start > end:
            break
        if covered_start > cursor:
            yield cursor, covered_start - day
        if covered_end >= end:
            return
        cursor = covered_end + day
    if cursor <= end:
        yield cursor, end


def plan_downloads(config: dict, base_dir: Path, today: dt.date):
    """Return missing exports, covered accounts, and untrusted-file warnings."""
    if not isinstance(config, dict):
        raise InecoConfigError("inecobank must be a mapping.")
    accounts = config.get("accounts")
    if not isinstance(accounts, list) or not accounts:
        raise InecoConfigError("Set inecobank.accounts to a non-empty list of account mappings.")
    folder = config.get("folder_path", "..")
    pattern = config.get("statement_glob", "*.xml")
    if not isinstance(folder, str) or not isinstance(pattern, str) or not pattern:
        raise InecoConfigError("Inecobank folder_path and statement_glob must be strings.")
    if Path(pattern).is_absolute() or ".." in Path(pattern).parts:
        raise InecoConfigError("Inecobank statement_glob must stay within folder_path.")
    folder = (base_dir / folder).resolve()
    requested = []
    seen = set()
    for item in accounts:
        if not isinstance(item, dict):
            raise InecoConfigError("Each Inecobank account must be a mapping with number.")
        number = item.get("number")
        if not isinstance(number, str) or not number.isascii() or not number.isdigit():
            raise InecoConfigError('Inecobank account number must be a quoted string of digits.')
        if number in seen:
            raise InecoConfigError(f"Duplicate Inecobank account: {number}")
        seen.add(number)
        account_type = item.get("type", "account")
        if account_type not in ("account", "card"):
            raise InecoConfigError(f"Inecobank {number}: type must be account or card.")
        name = item.get("name", "")
        if not isinstance(name, str):
            raise InecoConfigError(f"Inecobank {number}: name must be a string.")
        start = config_date(item.get("since-DD-MM-YYYY", config.get("since-DD-MM-YYYY")), "since-DD-MM-YYYY")
        until = item.get("until-DD-MM-YYYY", config.get("until-DD-MM-YYYY"))
        end = today if until is None else config_date(until, "until-DD-MM-YYYY")
        if start > end:
            raise InecoConfigError(f"Inecobank {number}: start date is after end date.")
        if end > today:
            raise InecoConfigError(f"Inecobank {number}: end date is in the future.")
        requested.append((number, name, account_type, start, end))

    coverage = {}
    warnings = []
    if folder.exists() and not folder.is_dir():
        raise InecoConfigError(f"Inecobank folder_path is not a directory: {folder}")
    try:
        paths = sorted(folder.glob(pattern))
    except (OSError, ValueError) as exc:
        raise InecoConfigError(f"Cannot scan Inecobank statement_glob: {exc}") from None
    for path in paths:
        if not path.is_file():
            continue
        try:
            number, start, end = statement_period(path)
        except (OSError, ET.ParseError, ValueError) as exc:
            warnings.append(f"Not counted: {path.name}: {exc}")
            continue
        coverage.setdefault(number, []).append((start, end))

    missing = []
    complete = []
    for number, name, account_type, start, end in requested:
        gaps = list(missing_periods(start, end, coverage.get(number, [])))
        if not gaps:
            complete.append(number)
        for gap_start, gap_end in gaps:
            filename = f"Statement_{number}_{gap_start.isoformat()}_{gap_end.isoformat()}.xml"
            target = folder / filename
            # An invalid or mislabelled existing export must never be overwritten.
            suffix = 1
            while target.exists():
                target = folder / f"{Path(filename).stem}_{suffix}.xml"
                suffix += 1
            missing.append(MissingStatement(number, name, account_type, gap_start, gap_end, target))
    return missing, complete, warnings


def print_manual_downloads(config: dict, base_dir: Path, today=None) -> int:
    missing, complete, warnings = plan_downloads(config, base_dir, today or dt.date.today())
    print("\nInecobank manual XML download checklist")
    for warning in warnings:
        print(f"  WARNING: {warning}")
    for account in complete:
        print(f"  Covered: {account} — XML periods cover the requested dates.")
    if not missing:
        print("No missing Inecobank statement periods.")
        print("This checks declared XML periods; an export through today only includes activity available at download time.")
        return 0
    print(
        "1. Sign in at https://online.inecobank.am/vcAccount/List.\n"
        "2. Open each account below (use Cards for card accounts) and set the inclusive dates.\n"
        "3. Use the download/export icon and choose XML. Website XLS is not supported.\n"
        "4. Save each export to the suggested path, creating the folder if needed."
    )
    for index, item in enumerate(missing, 1):
        label = f" ({item.name})" if item.name else ""
        print(f"\n  {index}. {item.account_type.capitalize()} {item.account}{label}")
        print(f"     From {item.start:%d/%m/%Y} to {item.end:%d/%m/%Y} "
              f"({(item.end - item.start).days + 1} days, inclusive)")
        print(f"     Save as: {item.path}")
    print(
        "\nRerun: python3 scripts/bank_downloader.py --manual-only\n"
        "Coverage comes from XML AccountNumber and Period, not filenames or transaction dates.\n"
        "Only XML exports are counted; this checks declared coverage, not transaction completeness.\n"
        "An export through today only includes activity available when it was downloaded."
    )
    return len(missing)
