"""Synthetic statement coverage tests; no bank connections or user configuration."""

import contextlib
import datetime as dt
import io
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import bank_downloader as cli
from bank_manual_ineco import InecoConfigError, plan_downloads, print_manual_downloads


class ManualInecoTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.folder = Path(self.temporary.name)
        self.today = dt.date(2026, 9, 16)
        self.config = {
            "folder_path": ".", "since-DD-MM-YYYY": "01-09-2026",
            "accounts": [{"number": "0001", "name": "AMD"}, {"number": "0002", "type": "card"}],
        }

    def export(self, filename, account, start, end):
        (self.folder / filename).write_text(
            f'<Statement><AccountNumber>{account}</AccountNumber>'
            f'<Period>[{start} - {end}]</Period><Operations/></Statement>', encoding="utf-8")

    def plan(self):
        return plan_downloads(self.config, self.folder, self.today)

    def test_missing_accounts_and_default_today(self):
        missing, complete, warnings = self.plan()
        self.assertEqual(len(missing), 2)
        self.assertEqual(missing[0].start, dt.date(2026, 9, 1))
        self.assertEqual(missing[0].end, self.today)
        self.assertEqual(missing[1].account_type, "card")
        self.assertEqual((complete, warnings), ([], []))

    def test_merge_periods_and_find_internal_gap_by_content(self):
        self.export("renamed.xml", "0001", "01/09/2026", "03/09/2026")
        self.export("overlap.xml", "0001", "02/09/2026", "05/09/2026")
        self.export("adjacent.xml", "0001", "06/09/2026", "07/09/2026")
        self.export("late.xml", "0001", "10/09/2026", "12/09/2026")
        self.export("wrong-account.xml", "9999", "01/09/2026", "16/09/2026")
        self.export("empty-card.xml", "0002", "01/08/2026", "20/09/2026")
        missing, complete, warnings = self.plan()
        self.assertEqual([(item.start.day, item.end.day) for item in missing], [(8, 9), (13, 16)])
        self.assertTrue(all(item.account == "0001" for item in missing))
        self.assertEqual(complete, ["0002"])
        self.assertFalse(warnings)

    def test_invalid_and_rejected_files_are_not_coverage_or_overwritten(self):
        target = self.folder / "Statement_0001_2026-09-01_2026-09-16.xml"
        target.write_text("<html>Request Rejected</html>")
        (self.folder / "broken.xml").write_text("<Statement>")
        self.export("bad-period.xml", "0001", "31/02/2026", "16/09/2026")
        missing, complete, warnings = self.plan()
        self.assertEqual(len(warnings), 3)
        self.assertFalse(complete)
        self.assertEqual(missing[0].path.name, "Statement_0001_2026-09-01_2026-09-16_1.xml")
        self.assertEqual(target.read_text(), "<html>Request Rejected</html>")

    def test_overrides_and_periods_outside_requested_range(self):
        self.config["accounts"][0].update({"since-DD-MM-YYYY": "04-09-2026", "until-DD-MM-YYYY": "10-09-2026"})
        self.export("old.xml", "0001", "01/08/2026", "31/08/2026")
        self.export("later.xml", "0001", "11/09/2026", "16/09/2026")
        missing, _, _ = self.plan()
        self.assertEqual((missing[0].start.day, missing[0].end.day), (4, 10))

    def test_namespace_and_no_transactions(self):
        (self.folder / "namespace.xml").write_text(
            '<Statement xmlns="urn:example"><AccountNumber>0001</AccountNumber>'
            '<Period>[01/09/2026 - 16/09/2026]</Period><Operations/></Statement>')
        missing, complete, _ = self.plan()
        self.assertEqual(complete, ["0001"])
        self.assertEqual([item.account for item in missing], ["0002"])

    def test_live_export_lowercase_root_and_separately_bracketed_dates(self):
        # Synthetic metadata with the same structure as a real bank export.
        # The misleading filename must never override the declared period.
        (self.folder / "Statement_0001_since_20200101.xml").write_text(
            '<?xml version="1.0" encoding="utf-8"?>'
            '<statement><AccountNumber>0001</AccountNumber>'
            '<Period>[01/09/2026] - [10/09/2026]</Period><Operations/></statement>')
        missing, _, warnings = self.plan()
        gaps = [(item.start.day, item.end.day) for item in missing if item.account == "0001"]
        self.assertEqual(gaps, [(11, 16)])
        self.assertFalse(warnings)

    def test_html_rejection_has_actionable_warning(self):
        (self.folder / "failed-download.xml").write_text(
            '<html><head><title>Request Rejected</title></head><body><br></body></html>')
        missing, complete, warnings = self.plan()
        self.assertEqual(len(missing), 2)
        self.assertFalse(complete)
        self.assertEqual(len(warnings), 1)
        self.assertIn("HTML page (Request Rejected), not an XML statement", warnings[0])

    def test_rerun_after_saving_exports_has_no_remaining_downloads(self):
        missing, _, _ = self.plan()
        for item in missing:
            self.export(item.path.name, item.account, item.start.strftime("%d/%m/%Y"),
                        item.end.strftime("%d/%m/%Y"))
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            count = print_manual_downloads(self.config, self.folder, self.today)
        self.assertEqual(count, 0)
        self.assertIn("No missing Inecobank statement periods", output.getvalue())

    def test_new_folder_reports_missing_without_creating_it(self):
        self.config["folder_path"] = "new-folder"
        missing, _, _ = self.plan()
        self.assertEqual(len(missing), 2)
        self.assertFalse((self.folder / "new-folder").exists())

    def test_leading_gap_and_single_day_gap(self):
        self.export("middle.xml", "0001", "03/09/2026", "15/09/2026")
        missing, _, _ = self.plan()
        gaps = [(item.start.day, item.end.day) for item in missing if item.account == "0001"]
        self.assertEqual(gaps, [(1, 2), (16, 16)])

    def test_invalid_config(self):
        for change in [{"number": 1}, {"number": "../file"}, {"type": "unknown"},
                       {"since-DD-MM-YYYY": "bad"}, {"until-DD-MM-YYYY": "01-01-2027"},
                       {"until-DD-MM-YYYY": "01-08-2026"}]:
            with self.subTest(change=change):
                self.config["accounts"] = [{"number": "0001", **change}]
                with self.assertRaises(InecoConfigError):
                    self.plan()

    def test_instructions_include_account_dates_and_duration(self):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            count = print_manual_downloads(self.config, self.folder, self.today)
        self.assertEqual(count, 2)
        for text in ["0001 (AMD)", "Card 0002", "01/09/2026", "16/09/2026", "16 days", "--manual-only"]:
            self.assertIn(text, output.getvalue())

    def test_manual_only_never_prompts_or_calls_downloads(self):
        fixture = self.folder / "fixture.yaml"
        fixture.write_text('inecobank:\n  folder_path: "."\n  since-DD-MM-YYYY: "01-09-2026"\n  accounts:\n    - number: "0001"\n')
        with patch.object(cli, "config_path", return_value=str(fixture)), \
                patch.object(cli, "MY_FOLDER_PATH", str(self.folder)), \
                patch.object(cli.sys, "argv", ["bank_downloader.py", "--manual-only"]), \
                patch.object(cli, "prompt_credentials") as prompt, \
                patch.object(cli, "run_download") as download, \
                contextlib.redirect_stdout(io.StringIO()):
            cli.main()
        prompt.assert_not_called()
        download.assert_not_called()


if __name__ == "__main__":
    unittest.main()
