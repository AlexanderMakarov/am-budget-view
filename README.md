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

List of supported banks and file extensions:
- Inecobank XML files (downloaded per-account from https://online.inecobank.am/vcAccount/List, click on account, icon it right bottom corner)
- Inecobank Excel/xlsx files (Inecobank sends them in emails with password protection)
- MyAmeria Excel/xls files (downloaded from https://myameria.am/events)
- AmeriaBank for Businesses CSV files (downloaded per-account from https://online.ameriabank.am/InternetBank/MainForm.wgx, click on account -> Transactions, icon at right top corner)

To add new bank support please provide file with transactions (in private, because it may contain sensitive information)
and instructions on how to get this file in your bank application.

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

On each run application would generate [Beancount](https://github.com/beancount/beancount) file
which then may be used with [Fava UI](https://github.com/beancount/fava).
Actually Beancount report contains more data than usual TXT report, it allows to track currencies.

First of all need to install Fava UI (built in Python) with something like `pip3 install fava`.

After getting log like `Built Beancount file 'Bank Aggregated Statement.beancount' with 1818 transactions.`
command (in console) like `fava Bank\ Aggregated\ Statement.beancount` will print
`Starting Fava on http://127.0.0.1:5000` - open this link in browser and it will show
graphs and other accounting visualization, financial statistic about your transactions.

# Limitations

- Application does not support currencies handling.
  Therefore if you are handling transactions/statements from multiple accounts then make sure that they have the same currency.
- Application does not support a way to group/assign transactions in a different way for different accounts.
  So your configuration should handle both. See `ignoreSubstrings` parameter description to handle some edge cases.

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
- [ ] (?) Support different schema with parsing. Aka "parse anything".
- [ ] (?) More tests coverage.
- [x] (?) Build translator to https://github.com/beancount/beancount
      Check in https://fava.pythonanywhere.com/example-beancount-file/editor/#
- [ ] Build UI with Fyne and https://github.com/wcharczuk/go-chart
      (https://github.com/Jacalz/sparta/commit/f9927d8b502e388bda1ab21b3028693b939e9eb2).
- [ ] Add multi-currency support: config for rates. Also see how Beancount handles it.
- [ ] Add multi-currency support: call https://open.er-api.com/v6/latest/AMD