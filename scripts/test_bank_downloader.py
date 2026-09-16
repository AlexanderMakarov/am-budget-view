"""Offline regression tests; never read local bank configuration or call banks."""

import copy
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import bank_downloader as cli
import bank_helpers_ameria as ameria


class AuthenticationTests(unittest.TestCase):
    def config(self):
        return {"my_ameria": {
            "auth_token": "stale", "client_id": "client",
            "history_path": "unused.csv", "since-DD-MM-YYYY": "01-04-2024",
        }}

    def test_guided_prompts_do_not_mutate_config(self):
        config = self.config()
        config["ameriabank"] = {"cookie": "stale-cookie"}
        original = copy.deepcopy(config)
        with patch.object(cli.sys.stdin, "isatty", return_value=True), \
                patch.object(cli.getpass, "getpass", side_effect=["fresh", "cookie"]), \
                patch("builtins.print"):
            prompted = cli.prompt_credentials(config)
        self.assertEqual(config, original)
        self.assertEqual(prompted["my_ameria"]["auth_token"], "fresh")
        self.assertEqual(prompted["ameriabank"]["cookie"], "cookie")

    def test_noninteractive_does_not_prompt(self):
        config = self.config()
        with patch.object(cli.sys.stdin, "isatty", return_value=False), \
                patch.object(cli.getpass, "getpass") as prompt:
            self.assertIs(cli.prompt_credentials(config), config)
            with patch.object(cli, "download_myameria_history",
                              side_effect=ameria.MyAmeriaUnauthorized("401")):
                with self.assertRaisesRegex(ameria.MyAmeriaUnauthorized, "terminal"):
                    cli.run_download(config)
            prompt.assert_not_called()

    def test_rejected_token_retries_once_with_new_session(self):
        config = self.config()
        with patch.object(cli.sys.stdin, "isatty", return_value=True), \
                patch.object(cli.getpass, "getpass", return_value="fresh"), \
                patch("builtins.input", return_value="new-client"), \
                patch.object(cli, "download_myameria_history", side_effect=[
                    ameria.MyAmeriaUnauthorized("401"), None]) as download:
            cli.run_download(config)
        self.assertEqual(download.call_count, 2)
        self.assertEqual(download.call_args.kwargs["auth_token"], "fresh")
        self.assertEqual(download.call_args.kwargs["client_id"], "new-client")
        self.assertEqual(config["my_ameria"]["auth_token"], "stale")

    def test_second_rejection_stops(self):
        with patch.object(cli.sys.stdin, "isatty", return_value=True), \
                patch.object(cli.getpass, "getpass", return_value="fresh"), \
                patch("builtins.input", return_value=""), \
                patch.object(cli, "download_myameria_history",
                              side_effect=ameria.MyAmeriaUnauthorized("401")) as download:
            with self.assertRaises(ameria.MyAmeriaUnauthorized):
                cli.run_download(self.config())
        self.assertEqual(download.call_count, 2)

    def test_401_preserves_existing_file_and_normalizes_token(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "history.csv"
            path.write_text("existing history")
            response = Mock(status_code=401)
            for token in ["raw-token", "  Bearer raw-token  ", "bearer raw-token"]:
                with self.subTest(token=token), \
                        patch.object(ameria.requests, "get", return_value=response) as get:
                    with self.assertRaisesRegex(ameria.MyAmeriaUnauthorized, "HTTP 401"):
                        ameria.download_myameria_history(
                            str(path), token, "01-04-2024", "16-09-2026", "client")
                    self.assertEqual(get.call_args.kwargs["headers"]["Authorization"],
                                     "Bearer raw-token")
                    self.assertEqual(path.read_text(), "existing history")

    def test_success_writes_csv(self):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "history.csv"
            response = Mock(status_code=200, ok=True)
            response.json.return_value = {"data": {"entries": []}}
            with patch.object(ameria.requests, "get", return_value=response):
                ameria.download_myameria_history(
                    str(path), "token", "01-04-2024", "16-09-2026", "client")
            self.assertTrue(path.read_text().startswith("Date,FromAccount,ToAccount,"))


if __name__ == "__main__":
    unittest.main()
