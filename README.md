# AM BudgetView
It is a local tool to investigate your expenses and incomes by bank transactions.

Was renamed from [aggregate-inecobank-statement](https://github.com/AlexanderMakarov/aggregate-inecobank-statement) after new features were added.

----

To control your expenses you need to know them, right?
But it is too boring to note all these details somewhere, each day.
Fortunately banks do it for us.
If you are using a bank plastic card or NFC application on the smartphone you probably have this listing already.

For example Armenian's [Inecobank](https://online.inecobank.am)
and [Ameria Bank](https://ameriabank.am)
provide list of monthly transactions in downloadable files.

This is a simple tool which allows to aggregate all transactions (hundreds of them) from
multiple accounts and banks into customisable and personalizable groups.
Result is a small structured piece of text provides valuable insights into your budget.
See example (numbers are made up, sum may not match):
```
2023-08-01..2023-08-31:
  Income (2, sum=1,493,878.00):
    Main salary                        : 1,345,343.00
  Expenses (13, sum= 920,636.38):
    Rent                               :   300,000.00
    Tom's health                       :   240,000.00
    Cash withdrowal                    :   178,000.00
    Groceries                          :   112,831.00
    Kindergarten                       :    90,000.00
    Kate's health                      :    61,000.00
    Taxi                               :    17,600.00
    Entertainment                      :    14,000.00
    Subscriptions                      :     7,787.78
    Pharmacies                         :     5,957.60
    Online stores                      :     3,090.00
2023-09-01..2023-09-30:
  Income (2, sum=1,516,629.00):
...
```
Some banks provide similar dashboards on their websites, but they can't assign good categories suitable for everyone.
This application allows you to configure it for your personal set of groups and ways to assign transactions to the specific group.

## List of supported banks, file formats and relevant notes
- [FULL] Inecobank XML (.xml) files downloaded per-account from https://online.inecobank.am/vcAccount/List
  (click on account, choose dates range, icon to download in right bottom corner).
  Supports all features native to app and Beancount reports.
  In `config.yaml` is referenced by `inecobankStatementXmlFilesGlob` setting.
  Parsed by [ineco_xml_parser.go](/ineco_xml_parser.go).
- [NONE] Inecobank Excel (.xls) files downloaded per-account from https://online.inecobank.am/vcAccount/List
  (the same place as XML above) - ARE NOT SUPPORTED because XML downloaded from the same place
  is more light and predictable format for parsing.
- [PARTIAL] Inecobank Excel (.xlsx) files which Inecobank sends in emails with password protection.
  Don't have Reciever/Payer account number so resulting Beancount report won't be full.
  To allow app use such files need to unprotect them first (
  [MS Office instruction](https://support.microsoft.com/en-us/office/change-or-remove-workbook-passwords-1c17af87-25e2-4dc6-94f0-19ce21ad0b68),
  [LibreOffice instruction](https://ask.libreoffice.org/t/remove-file-password-protection/30982)).
  In `config.yaml` is referenced by `inecobankStatementXlsxFilesGlob` setting.
  Parsed by [ineco_excel_parser.go](/ineco_excel_parser.go).
- [FULL] AmeriaBank for Businesses CSV (.CSV) files downloaded per-account from
  https://online.ameriabank.am/InternetBank/MainForm.wgx, click on account -> Statement,
  chose period (for custom use "FromDate" and "To" date pickers),
  set "Show equivalent in AMD" checkbox, press "Export to CSV" icon is placed at right top corner.
  Supports all features native to app and Beancount reports.
  In `config.yaml` is referenced by `ameriaCsvFilesGlob` setting.
  Parsed by [ameria_csv_parser.go](/ameria_csv_parser.go).
- [NONE] AmeriaBank for Businesses XML (.XML) files downloaded per-account from
  https://online.ameriabank.am/InternetBank/MainForm.wgx
  (the same place as CSV above) - ARE NOT SUPPORTED
  XML files from "Transactions" don't contain its own Reciever/Payer account number and currency.
- [FULL] MyAmeria Account Statements Excel (.xls) dowloaded from pages like
  https://myameria.am/cards-and-accounts/account-statement/******
  From the web site choose Cards and Accounts -> Current account -> Statement,
  here select period (last option usually), if available the set "In AMD" (under "Show also"),
  choose "Excel" format, press "Download". Almost the same in mobile app.
  Default file name "<account_number> account statement.xls".
  Supports all features native to app and Beancount reports.
  In `config.yaml` is referenced by `myAmeriaAccountStatementXlsxFilesGlob` setting.
  Parsed by [ameria_stmt_parser.go](/ameria_stmt_parser.go).
- [PARTIAL] MyAmeria History Excel (.xls) files downloaded from https://myameria.am/events
  Page doesn't work on the bank's web site and mobile app for now (September 2024)
  but is supported for previously downloaded reports.
  In `config.yaml` is referenced by `myAmeriaHistoryXlsFilesGlob` setting.
  Note that it should be accompanied by `myAmeriaMyAccounts` setting because files
  don't have account number or by `myAmeriaIncomeSubstrings` setting
  to at least distinguish incomes from expenses in the file.
  Parsed by [ameria_history_parser.go](/ameria_history_parser.go).

To add new bank support please provide file with transactions (in private or with obfuscsated data,
because it contains sensitive information) downloaded from bank application
and instructions how you got this file.

# JUST FOR INECOBANK XML - How to use

[![Watch the video](https://img.youtube.com/vi/4MZN-SK15HE/hqdefault.jpg)](https://www.youtube.com/embed/4MZN-SK15HE)

1. Download application binary ("aggregate-inecobank-statements-\*-\*") file for your operating system from
   [Releases](https://github.com/AlexanderMakarov/aggregate-inecobank-statement/releases) page.
   About what to choose:
 	- For Windows use "aggregate-inecobank-statements-windows-amd64.exe". Even if you have an Intel CPU.
 	- For Mac OS X with M1+ CPU/core use "aggregate-inecobank-statements-darwin-arm64".
   	For older Macbooks use "aggregate-inecobank-statements-darwin-amd64".
 	- For Linux-es usually "aggregate-inecobank-statements-windows-amd64".
2. Download "Statement ....xml" files from https://online.inecobank.am for interesting period and
   put them near the "aggregate-inecobank-statements-\*-\*" file.
   Namely, on [main page](https://online.inecobank.am) click on the chosen account,
   specify into 'From' and 'To' fields dates you want to analyze,
   press 'Search', scroll page to bottom and here at the right corner will be 5 icons to download statements.
   Press XML button and save file near "aggregate-inecobank-statements-\*-\*" file.
3. Save [config.yaml](https://raw.githubusercontent.com/AlexanderMakarov/aggregate-inecobank-statement/master/config.yaml)
   as an example of configuration. Don't need to update it yet, see step 5.
4. Run application ("aggregate-inecobank-statements-\*-\*" file).
   It would open a text file with the list of groups with a lot of transactions it consists of.
   Most probably it would also have an "unknown" group with not yet categorized transactions.
5. Investigate your personal transaction information and update configuration file groups with unique
   for specific transaction substrings to aggregate transactions into these groups.
   "unknown" group is the first item to address.
   See examples in configuration file - you may remove not needed and add your own groups.
   Be careful about syntax and indentations, but in case of any error the resulting file would contain
   an error description which may help to understand the reason.
6. Run application again, and repeat configuration changes if needed.
   Next set `detailedOutput` to `false` in the configuration file to hide detalization by transactions.
   If you still want to see all these "unknown" transactions then consider to set
   `groupAllUnknownTransactions` to `false` - it will group these "I don't know group" transactions into
   individual groups with name equal to "Details" field value.
7. Run application one more time to get a clean report for manual investigation, comparing months, etc.
8. Next month it is enough to download "Statements" with new transactions and run application again.

Note that it is a command line application and may work completely in the terminal.
Run in it terminal with `-h` for details.
It would explain how to work with multiple configuration files and see information directly in terminal.

# Use with Beancount and Fava UI

Application may generate [Beancount](https://github.com/beancount/beancount) file
which then may be used with [Fava UI](https://github.com/beancount/fava).
Beancount report contains more data than usual TXT report, it allows to do full double-entry accounting.
But Fava UI is an accounting tool, therefore hard to understand for those who don't have
solid accounting knowledge.

First of all need to install Fava UI (built in Python) with something like `pip3 install fava`.

After getting log like `Built Beancount file 'Bank Aggregated Statement.beancount' with 1818 transactions.`
run in console `fava Bank\ Aggregated\ Statement.beancount` which should print
`Starting Fava on http://127.0.0.1:5000` - open this link in browser and it would show
graphs and other accounting visualization, financial statistic about your transactions.
If run am-budget-view one more time (for example with corrected configuration) then
Fava UI would catch up changes without restart.

# Limitations

- Application does not support currencies handling in simple TXT report.
  Therefore if you are handling transactions/statements from multiple accounts then make sure that they have the same currency. Or use Beancount report and Fava UI.
- Application does not support a way to categorize transactions in a different way for different accounts.
  So your configuration should handle both. Use `ignoreSubstrings` parameter to handle edge cases but
  be aware that such "skipped" transactions affect output numbers!
  Beancount report is not affected by `ignoreSubstrings` parameter.

# Contributions

Feel free to contribute your features, fixes and so on.

It is usual Go repo with some useful shortcuts in [Makefile](/Makefile).

Also please help to fix Armenian subtitles in the [YouTube video](https://www.youtube.com/embed/4MZN-SK15HE?cc_load_policy=1) - I believe that Google Translator provided
me with pretty mediocre translation but my Armenian knowledge is not enough to make subtitles better.

# Development

## Setup

- Install Go v1.21+
- `go mod init`
- Made your changes, run test via [Makefile](/Makefile) targets and test manually with `go run .`
- Make PR.

## Release
Merge to "master" and push tag with name "releaseX.X.X". CI will do the rest.

## TODO/Roadmap

- [x] Fail if wrong field in config found.
- [x] Add CI for pull requests (different branches).
- [x] Parse CSV-s from online.ameriabank.am.
- [x] Propagate not fatal errors from parsing files into report.
- [x] Parse XLS-s from myameria.am.
- [x] Parse InecoBank XLS files which are sent in emails and
      InecoBank doesn't allow to download data older than 2 years.
- [x] Rename repo to don't be tied to Inecobank.
- [ ] Write instruction about both options for Ameriabank transactions. Record new video(s).
- [ ] Create default configuration file of first start.
- [ ] (?) Support different schema with parsing. Aka "parse anything".
- [ ] (?) More tests coverage.
- [x] (?) Build translator to https://github.com/beancount/beancount
      Check in https://fava.pythonanywhere.com/example-beancount-file/editor/#
- [ ] Build UI with Fyne and https://github.com/wcharczuk/go-chart
      (https://github.com/Jacalz/sparta/commit/f9927d8b502e388bda1ab21b3028693b939e9eb2).
- [ ] Add multi-currency support: config for rates. Also see how Beancount handles it.
- [ ] Add multi-currency support: call https://open.er-api.com/v6/latest/AMD