"""
Bank-specific helpers for automation (statements, transactions, etc.).
Reusable across bank_downloader.py and future scripts.
"""

import logging
import re
import time
import urllib.parse
from dataclasses import dataclass

import requests

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# AmeriaBank (online.ameriabank.am) – legal entities / business
# ---------------------------------------------------------------------------

AMERIABANK_ORIGIN = "https://online.ameriabank.am"
AMERIABANK_MAIN_REFERER = "https://online.ameriabank.am/InternetBank/MainForm.wgx"


@dataclass
class AmeriabankAccount:
    """One account from AmeriaBank Accounts list (from XML)."""

    account_number: str
    currency: str
    name: str
    account_type: str
    available_balance: str
    note: str = ""


def _latin1_safe(s: str) -> str:
    """Ensure string is valid for HTTP header value (latin-1). Drops chars that can't be encoded."""
    if not s:
        return s
    return s.encode("latin-1", errors="ignore").decode("latin-1")


def ameriabank_session_headers(
    cookie: str,
    *,
    referer: str | None = None,
    accept: str = "application/xml, text/xml, */*; q=0.01",
) -> dict[str, str]:
    """Build request headers for AmeriaBank session (Cookie-based auth)."""
    return {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0",
        "Accept": accept,
        "Accept-Language": "en,ru-RU;q=0.9,ru;q=0.8,en-US;q=0.7",
        "Accept-Encoding": "gzip, deflate, br, zstd",
        "Connection": "keep-alive",
        "Cookie": _latin1_safe(cookie),
        "Referer": referer or AMERIABANK_MAIN_REFERER,
        "Sec-Fetch-Dest": "empty",
        "Sec-Fetch-Mode": "cors",
        "Sec-Fetch-Site": "same-origin",
    }


def parse_ameriabank_accounts_from_xml(xml_text: str) -> list[AmeriabankAccount]:
    """
    Parse account list from Content.MainForm.wgx XML: WC:LV with R rows, each R has SI COL="n" c="value".
    Columns: c0=Account No., c1=Currency, c2=Name, c3=Account Type, c5=Available Balance, c7=Note.
    """
    accounts = []
    for row_match in re.finditer(r'<(?:\w*:)?R\s+Id="\d+"[^>]*>(.*?)</(?:\w*:)?R>', xml_text, re.DOTALL | re.IGNORECASE):
        inner = row_match.group(1)
        cols = {}
        for m in re.finditer(r'<(?:\w*:)?SI\s+COL="(\d+)"\s+c="([^"]*)"', inner, re.IGNORECASE):
            cols[int(m.group(1))] = m.group(2)
        if 0 not in cols:
            continue
        accounts.append(
            AmeriabankAccount(
                account_number=cols.get(0, "").strip(),
                currency=cols.get(1, "").strip(),
                name=cols.get(2, "").strip(),
                account_type=cols.get(3, "").strip(),
                available_balance=cols.get(5, "").strip(),
                note=cols.get(7, "").strip(),
            )
        )
    return accounts


def download_ameriabank_statement_csv(
    export_url: str,
    cookie: str,
    path: str,
    *,
    timeout: int = 60,
) -> None:
    """
    Download CSV from AmeriaBank ExportCsv URL (GET with session cookie).
    Response may be utf-16; we decode and save as utf-8 for consistency.
    """
    # Export URL might be with :443 in host; normalize to same-origin
    url = export_url.replace(":443", "")
    headers = ameriabank_session_headers(
        cookie,
        referer=AMERIABANK_MAIN_REFERER,
        accept="text/csv, application/csv, */*; q=0.01",
    )
    url_short = url[:90] + "..." if len(url) > 90 else url
    logger.info("AmeriaBank GET export %s", url_short)
    resp = requests.get(url, headers=headers, stream=True, timeout=timeout)
    logger.info("AmeriaBank GET export -> %s len=%s", resp.status_code, len(resp.content))
    resp.raise_for_status()
    raw = resp.content
    # Bank may return utf-16; try to decode
    try:
        text = raw.decode("utf-16")
    except UnicodeDecodeError:
        text = raw.decode("utf-8", errors="replace")
    with open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    logger.info("Downloaded AmeriaBank statement to %s", path)


def extract_export_csv_url_from_ameriabank_xml(xml_text: str) -> str | None:
    """
    Parse server response XML after we simulate clicking "Export to CSV". The server returns
    the one-time download URL (with requestid) in the response.
    """
    # Format: <MIs><MI ME="sb476" ARG0="https://...ExportCsv.wgx?requestid=...&amp;format=csv&amp;encoding=utf-16" .../>
    match = re.search(
        r'<MI\s[^>]*\sARG0="(https?://[^"]*ExportCsv\.wgx[^"]*)"',
        xml_text,
        re.IGNORECASE,
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # ARG0="https://...ExportCsv.wgx?requestid=...&format=csv&encoding=utf-16"
    match = re.search(
        r'ARG0="(https?://[^"]*ExportCsv\.wgx[^"]*)"',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # Alternate: ARG0 with requestid and format=csv (different endpoint name)
    match = re.search(
        r'ARG0="(https?://[^"]*requestid=[^"]*format=csv[^"]*)"',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # Any URL containing ExportCsv or requestid+format=csv
    match = re.search(
        r'(https?://[^"<>\s]+ExportCsv[^"<>\s]*(?:\?[^"<>\s]*)?)',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    match = re.search(
        r'(https?://[^"<>\s]+requestid=[^"<>\s]+format=csv[^"<>\s]*)',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # MI or similar: ... "https://...requestid=...&format=csv..."
    match = re.search(
        r'"(https?://[^"]*requestid=\d+[^"]*format=csv[^"]*)"',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # ARG0 with escaped ampersands
    match = re.search(
        r'ARG0="(https?://[^"]+)"',
        xml_text
    )
    if match:
        u = match.group(1).replace("&amp;", "&")
        if "requestid=" in u and "csv" in u.lower():
            return u
    # Any substring that looks like the export URL (requestid= digits then format=csv)
    match = re.search(
        r'(https?://[^"<>\s]+requestid=\d+[^"<>\s]*format=csv[^"<>\s]*)',
        xml_text
    )
    if match:
        return match.group(1).replace("&amp;", "&")
    # Single-quoted or unquoted URL
    match = re.search(
        r'["\']?(https?://online\.ameriabank[^"<>\s]+requestid=\d+[^"<>\s]*)["\']?',
        xml_text
    )
    if match:
        u = match.group(1).replace("&amp;", "&")
        if "csv" in u.lower() or "ExportCsv" in u:
            return u
    # URL may be in attribute with escaped &amp;: find https before requestid=, end at closing quote
    idx = xml_text.find("requestid=")
    if idx >= 0:
        start = xml_text.rfind("https://", 0, idx)
        if start < 0:
            start = xml_text.rfind("http://", 0, idx)
        if start >= 0:
            end = xml_text.find('"', start)
            if end < 0:
                end = xml_text.find("'", start)
            if end > start:
                u = xml_text[start:end].replace("&amp;", "&")
                if "requestid=" in u and ("format=csv" in u or "ExportCsv" in u or "csv" in u.lower()):
                    return u
        # Try: any ARG0="..."; take the value that contains requestid
        for arg in re.finditer(r'ARG0="([^"]+)"', xml_text):
            u = arg.group(1).replace("&amp;", "&")
            if "requestid=" in u and ("csv" in u.lower() or "ExportCsv" in u):
                return u
    # ExportCsv (or ExportCsv.wgx) might appear in any attribute; find it and extract full URL
    ex = xml_text.find("ExportCsv.wgx")
    if ex < 0:
        ex = xml_text.find("ExportCsv")
    if ex >= 0:
        start = xml_text.rfind("https://", 0, ex)
        if start < 0:
            start = xml_text.rfind("http://", 0, ex)
        if start >= 0:
            end = xml_text.find('"', start)
            if end < 0:
                end = xml_text.find("'", start)
            if end < 0:
                end = xml_text.find("<", start)
            if end > start:
                u = xml_text[start:end].replace("&amp;", "&")
                if "requestid=" in u:
                    return u
    return None


def _short_preview(text: str, max_len: int = 380) -> str:
    """One-line preview for logs: length and start of content."""
    if not text:
        return "empty"
    t = text.strip()
    if len(t) <= max_len:
        return t.replace("\n", " ")[:max_len]
    return t[:max_len].replace("\n", " ") + "..."


def _ameriabank_lr_from_xml(xml_text: str) -> str | None:
    """Extract LR (session token) from server XML. Server sends it in every response; we send it back in each POST. Not from user."""
    if not xml_text or not xml_text.strip():
        return None
    # Standard: <WG:R ... LR="...">
    m = re.search(r'<WG:R\s[^>]*\sLR="([^"]+)"', xml_text)
    if m:
        return m.group(1)
    # LR before other attrs
    m = re.search(r'<WG:R\s+LR="([^"]+)"', xml_text)
    if m:
        return m.group(1)
    # With namespace (e.g. wg:R or WG:R in different casing)
    m = re.search(r'<[^:>]*:R\s[^>]*\sLR="([^"]+)"', xml_text)
    if m:
        return m.group(1)
    # Any LR="..." in first 3k chars (session token often near start)
    head = xml_text[:3000]
    m = re.search(r'\bLR="([^"]+)"', head)
    if m:
        return m.group(1)
    return None


def _ameriabank_find_id_by_text(xml_text: str, text: str) -> str | None:
    """Find component Id by TX or TT attribute (case-insensitive contains)."""
    # Match Id="123" where same tag has TX="..." or TT="..." containing text
    pattern = rf'<[^>]+\s(?:TX|TT)="[^"]*{re.escape(text)}[^"]*"[^>]*\sId="(\d+)"'
    m = re.search(pattern, xml_text, re.IGNORECASE)
    if m:
        return m.group(1)
    pattern2 = rf'<[^>]+\sId="(\d+)"[^>]+\s(?:TX|TT)="[^"]*{re.escape(text)}[^"]*"'
    m = re.search(pattern2, xml_text, re.IGNORECASE)
    return m.group(1) if m else None


def _ameriabank_parse_lv_and_row_ids(xml_text: str) -> tuple[str | None, list[str]]:
    """Parse WC:LV Id and child R Ids (for account list). Returns (lv_id, [r_id, ...])."""
    lv_id = None
    lv_match = re.search(r'<WC:LV\s[^>]*\sId="(\d+)"', xml_text)
    if lv_match:
        lv_id = lv_match.group(1)
    if not lv_id:
        lv_match = re.search(r'<\w*:?LV\s[^>]*Id="(\d+)"', xml_text)
        if lv_match:
            lv_id = lv_match.group(1)
    row_ids = re.findall(r'<R\s+Id="(\d+)"', xml_text)
    if not row_ids:
        row_ids = re.findall(r'<\w*:?R\s+Id="(\d+)"', xml_text)
    if not row_ids:
        row_ids = re.findall(r'<R\s[^>]*\sId="(\d+)"', xml_text)
    return lv_id, row_ids


class AmeriabankWebGuiSession:
    """
    Minimal WebGUI client to simulate user flow on online.ameriabank.am.
    User provides only cookie + content_url (copy from Network tab after login).

    Protocol (no user-provided requestid or LR):
    - LR: Session token that the server includes in every XML response. We parse it from
      the initial GET and send it back in each POST (clicks, etc.) so the server ties
      the action to this session. User does not provide LR.
    - requestid: When we simulate clicking "Export to CSV", the server responds with
      XML containing the one-time download URL (with requestid). We parse that URL from
      the response and GET it to download the CSV. We do not ask the user for requestid
      and we do not guess it.
    """

    def __init__(self, cookie: str, content_url: str, timeout: int = 60):
        self.cookie = cookie
        # Accept path-only URL (e.g. /InternetBank/Route/...); prepend origin
        url = (content_url or "").strip()
        if url and not url.startswith("http://") and not url.startswith("https://"):
            url = AMERIABANK_ORIGIN.rstrip("/") + ("/" if not url.startswith("/") else "") + url
        self.content_url = url
        self.timeout = timeout
        # Derive route base (strip Content.MainForm.wgx and query)
        self.route_base = self.content_url.split("Content.MainForm.wgx")[0].split("content.MainForm.wgx")[0]
        self._lr: str | None = None
        self._xml: str = ""

    def _headers(self) -> dict[str, str]:
        return {
            **ameriabank_session_headers(self.cookie),
            "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
            "X-Requested-With": "XMLHttpRequest",
            "Origin": AMERIABANK_ORIGIN,
        }

    def _post_events(self, lr: str | None, events: str) -> str:
        """POST event payload; returns response text."""
        if not lr:
            raise ValueError("Session LR not set (call get_initial_content first)")
        payload = {"<ES LR": f'"{lr}">{events}</ES>'}
        data = urllib.parse.urlencode(payload)
        url_short = self.content_url[:80] + "..." if len(self.content_url) > 80 else self.content_url
        logger.info("AmeriaBank POST %s (body ~%s bytes)", url_short, len(data))
        resp = requests.post(
            self.content_url,
            headers=self._headers(),
            data=data,
            timeout=self.timeout,
        )
        resp.raise_for_status()
        logger.info("AmeriaBank POST -> %s len=%s preview=%s", resp.status_code, len(resp.text), _short_preview(resp.text))
        return resp.text

    def get_initial_content(self) -> str:
        """GET initial content (main form). Returns XML; sets _lr and _xml."""
        url_short = self.content_url[:80] + "..." if len(self.content_url) > 80 else self.content_url
        logger.info("AmeriaBank GET %s", url_short)
        resp = requests.get(
            self.content_url,
            headers=ameriabank_session_headers(self.cookie),
            timeout=self.timeout,
        )
        logger.info("AmeriaBank GET -> %s len=%s content-type=%s", resp.status_code, len(resp.text), resp.headers.get("Content-Type", ""))
        resp.raise_for_status()
        self._xml = resp.text
        if "Dear customer" in self._xml or "we were unable to complete" in self._xml:
            logger.warning("AmeriaBank GET response: server error page. preview=%s", _short_preview(self._xml))
            raise ValueError(
                "Server returned an error page ('we were unable to complete your request'). "
                "Try again later or re-login and copy a fresh cookie and content_url (avoid copying truncated or modified values)."
            )
        redirect = re.search(r'RedirectToUrl="([^"]+)"', self._xml)
        if redirect:
            logger.warning("AmeriaBank GET response: redirect to %s (session expired or logged out)", redirect.group(1))
            raise ValueError(
                "Session expired or logged out: server returned redirect to %s. "
                "Re-login at https://online.ameriabank.am and copy a fresh cookie and content_url from the Network tab."
                % redirect.group(1)
            )
        self._lr = _ameriabank_lr_from_xml(self._xml)
        if self._lr:
            logger.info("AmeriaBank GET response: LR parsed (len=%s)", len(self._lr))
        else:
            logger.warning("AmeriaBank GET response: LR not found. preview=%s", _short_preview(self._xml))
            raise ValueError("Could not parse LR from initial content (wrong content_url or unexpected response; check logs for response preview)")
        return self._xml

    def click_by_tx_or_tt(self, text: str) -> str:
        """Find component by TX/TT containing text, send Click, return new XML."""
        tid = _ameriabank_find_id_by_text(self._xml, text)
        if not tid:
            raise ValueError(f"Could not find component with text containing: {text}")
        logger.info("AmeriaBank click '%s' (id=%s)", text[:40], tid)
        events = f'<E CPX="0" CPY="0" SR="{tid}" TP="Click" BTN="L" X="0" Y="0"/>'
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        return self._xml

    def navigate_to_accounts(self) -> list[AmeriabankAccount]:
        """Click Accounts tab and parse account list. Sets _lv_id and _row_ids for later use."""
        logger.info("AmeriaBank navigating to Accounts tab")
        self.click_by_tx_or_tt("Accounts")
        time.sleep(0.8)
        accounts = parse_ameriabank_accounts_from_xml(self._xml)
        if not accounts:
            time.sleep(0.5)
            accounts = parse_ameriabank_accounts_from_xml(self._xml)
        if not accounts:
            m = re.search(r'<(?:\w*:)?(?:LV|GV|R)\s', self._xml, re.IGNORECASE)
            idx = m.start() if m else -1
            snippet = _short_preview(self._xml[max(0, idx - 50) : idx + 500] if idx >= 0 else self._xml[:500], 450)
            logger.warning("AmeriaBank accounts: no list parsed. snippet=%s", snippet)
            raise ValueError("Could not parse account list from Accounts tab")
        self._lv_id, self._row_ids = _ameriabank_parse_lv_and_row_ids(self._xml)
        if not self._lv_id or len(self._row_ids) < len(accounts):
            logger.warning("AmeriaBank accounts: parsed %d accounts, lv_id=%s, row_ids=%d", len(accounts), self._lv_id, len(self._row_ids))
            raise ValueError("Could not parse account list IDs from XML (lv_id=%s, row_ids=%d, accounts=%d)" % (self._lv_id, len(self._row_ids), len(accounts)))
        account_numbers = [a.account_number for a in accounts]
        logger.info("Found %d accounts on Accounts tab: %s", len(accounts), ", ".join(account_numbers))
        return accounts

    def open_statement_for_account(self, lv_id: str, row_id: str) -> str:
        """Select row and click Statement; returns dialog XML."""
        st_id = _ameriabank_find_id_by_text(self._xml, "Statement")
        if not st_id:
            st_id = _ameriabank_find_id_by_text(self._xml, "View account statement")
        if not st_id:
            raise ValueError("Could not find Statement button")
        events = (
            f'<E CPX="0" CPY="0" SR="{lv_id}" TP="GotFocus"/>'
            f'<E CPX="0" CPY="0" SR="{lv_id}" TP="SelectionChange" Indexes="0"/>'
            f'<E CPX="0" CPY="0" SR="{row_id}" TP="Click" BTN="L" X="0" Y="0"/>'
            f'<E CPX="0" CPY="0" SR="{st_id}" TP="Click" BTN="L" X="0" Y="0"/>'
        )
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        return self._xml

    def set_statement_period_and_ok(
        self,
        period_ix: int = 5,
        *,
        from_dd_mm_yyyy: str | None = None,
        to_dd_mm_yyyy: str | None = None,
    ) -> str:
        """Set period or custom date range and click OK in Statement dialog."""
        if from_dd_mm_yyyy and to_dd_mm_yyyy:
            if not self._set_statement_dates(from_dd_mm_yyyy, to_dd_mm_yyyy):
                logger.warning("Custom date controls not found; using period_ix=%s", period_ix)
                self._set_statement_period_ix(period_ix)
        else:
            self._set_statement_period_ix(period_ix)
        ok_id = _ameriabank_find_id_by_text(self._xml, "OK")
        if not ok_id:
            ok_id = _ameriabank_find_id_by_text(self._xml, "Ok")
        if not ok_id:
            ok_id = _ameriabank_find_id_by_text(self._xml, "Apply")
        if not ok_id:
            m = re.search(r'<WC:B[^>]*\sId="(\d+)"[^>]*TX="OK"', self._xml)
            ok_id = m.group(1) if m else None
        if not ok_id:
            m = re.search(r'TX="OK"[^>]*\sId="(\d+)"', self._xml)
            ok_id = m.group(1) if m else None
        if not ok_id:
            m = re.search(r'TT="OK"[^>]*\sId="(\d+)"', self._xml)
            ok_id = m.group(1) if m else None
        if not ok_id:
            m = re.search(r'<[^>]*\s(?:TX|TT)="(?:OK|Ok|Apply)"[^>]*\sId="(\d+)"', self._xml)
            ok_id = m.group(1) if m else None
        if not ok_id:
            ok_id = _ameriabank_find_id_by_text(self._xml, "Show")
        if not ok_id:
            ok_id = _ameriabank_find_id_by_text(self._xml, "Display")
        if not ok_id:
            m = re.search(r'<WC:B[^>]*\sId="(\d+)"[^>]*TX="([^"]{1,15})"', self._xml)
            if m:
                ok_id = m.group(1)
                logger.info("AmeriaBank using button Id=%s TX=%s for Statement dialog", ok_id, m.group(2))
        if not ok_id:
            raise ValueError("Could not find OK/Apply button in Statement dialog")
        events = f'<E CPX="0" CPY="0" SR="{ok_id}" TP="Click" BTN="L" X="0" Y="0"/>'
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        self._send_statement_loaded_if_present()
        return self._xml

    def _send_statement_loaded_if_present(self) -> None:
        """After OK on Statement dialog, browser sends OnLoaded for the statement view. Send it so server returns updated LR used for Export CSV."""
        idx = self._xml.find("Print.wgx")
        if idx < 0:
            idx = self._xml.find("TouchScrollHtmlBox")
        if idx < 0:
            return
        before = self._xml[: idx + 1]
        uc_ids = re.findall(r"<WC:UC\s[^>]*\sId=\"(\d+)\"", before, re.IGNORECASE)
        if not uc_ids:
            return
        loaded_id = uc_ids[0]
        events = f'<E CPX="0" CPY="0" SR="{loaded_id}" TP="OnLoaded"/>'
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr

    def _set_statement_period_ix(self, period_ix: int) -> None:
        """Select period by index in Statement dialog (e.g. 5 = Last 31 Days)."""
        period_id = _ameriabank_find_id_by_text(self._xml, "Period")
        if not period_id:
            period_id = re.search(r'<WC:CB[^>]*\sId="(\d+)"', self._xml)
            period_id = period_id.group(1) if period_id else None
        if period_id:
            events = f'<E CPX="0" CPY="0" SR="{period_id}" TP="SelectionChange" Indexes="{period_ix}"/>'
            self._xml = self._post_events(self._lr, events)
            self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr

    def _set_statement_dates(self, from_dd_mm_yyyy: str, to_dd_mm_yyyy: str) -> bool:
        """Try to set From/To date in Statement dialog. Returns True if controls found and set."""
        from_id = _ameriabank_find_id_by_text(self._xml, "From") or _ameriabank_find_id_by_text(self._xml, "FromDate")
        to_id = _ameriabank_find_id_by_text(self._xml, "To ")
        if not to_id:
            to_id = _ameriabank_find_id_by_text(self._xml, "To")
        if not from_id or not to_id:
            return False
        value_from = from_dd_mm_yyyy.replace("-", ".")
        value_to = to_dd_mm_yyyy.replace("-", ".")
        events = (
            f'<E CPX="0" CPY="0" SR="{from_id}" TP="ValueChange" Val="{value_from}"/>'
            f'<E CPX="0" CPY="0" SR="{to_id}" TP="ValueChange" Val="{value_to}"/>'
        )
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        return True

    def click_export_csv(self) -> str | None:
        """Click Export to CSV; returns the export URL from response."""
        cid = _ameriabank_find_id_by_text(self._xml, "Export to CSV")
        if not cid:
            cid = _ameriabank_find_id_by_text(self._xml, "ExportCsv")
        if not cid:
            cid = re.search(r'TT="Export to CSV"[^>]*\sId="(\d+)"', self._xml)
            cid = cid.group(1) if cid else None
        if not cid:
            raise ValueError("Could not find Export to CSV button")
        events = f'<E CPX="0" CPY="0" SR="{cid}" TP="Click" BTN="L" X="0" Y="0"/>'
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        url = extract_export_csv_url_from_ameriabank_xml(self._xml)
        if not url:
            has_req = "requestid=" in self._xml
            has_export = "ExportCsv" in self._xml or "export" in self._xml.lower()
            logger.warning("AmeriaBank Export CSV response: no export URL (requestid=%s ExportCsv=%s). preview=%s", has_req, has_export, _short_preview(self._xml, 500))
        return url

    def download_statement_for_account(
        self,
        account_index: int,
        since_dd_mm_yyyy: str,
        to_dd_mm_yyyy: str,
        output_path: str,
        *,
        period_ix: int = 5,
    ) -> None:
        """
        Open Statement for the account at account_index, set date range (since–to),
        export CSV and download to output_path. Caller must have called get_initial_content()
        and navigate_to_accounts() first. After this call, session is back on Accounts tab.
        """
        import os
        if account_index >= len(getattr(self, "_row_ids", [])):
            raise ValueError(f"Account index {account_index} out of range")
        lv_id = self._lv_id
        row_ids = self._row_ids
        st_id = _ameriabank_find_id_by_text(self._xml, "Statement") or _ameriabank_find_id_by_text(self._xml, "View account statement")
        if not st_id:
            raise ValueError("Statement button not found")
        events = (
            f'<E CPX="0" CPY="0" SR="{lv_id}" TP="GotFocus"/>'
            f'<E CPX="0" CPY="0" SR="{lv_id}" TP="SelectionChange" Indexes="{account_index}"/>'
            f'<E CPX="0" CPY="0" SR="{row_ids[account_index]}" TP="Click" BTN="L" X="0" Y="0"/>'
            f'<E CPX="0" CPY="0" SR="{st_id}" TP="Click" BTN="L" X="0" Y="0"/>'
        )
        self._xml = self._post_events(self._lr, events)
        self._lr = _ameriabank_lr_from_xml(self._xml) or self._lr
        time.sleep(0.3)
        self.set_statement_period_and_ok(
            period_ix,
            from_dd_mm_yyyy=since_dd_mm_yyyy,
            to_dd_mm_yyyy=to_dd_mm_yyyy,
        )
        time.sleep(0.5)
        logger.info("AmeriaBank clicking Export CSV for account index %s", account_index)
        export_url = self.click_export_csv()
        if not export_url:
            raise ValueError("No export URL returned for this account")
        out_dir = os.path.dirname(os.path.abspath(output_path))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        download_ameriabank_statement_csv(export_url, self.cookie, output_path)
        time.sleep(0.3)
        self.click_by_tx_or_tt("Accounts")
        time.sleep(0.3)
        self._lv_id, self._row_ids = _ameriabank_parse_lv_and_row_ids(self._xml)


