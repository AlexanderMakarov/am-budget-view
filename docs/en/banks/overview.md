# Supported banks overview

Supported sources:

- [Inecobank](inecobank.md) accounts
- [AmeriaBank](ameria-business.md) business (legal) accounts
- [MyAmeria](myameria.md) individual accounts
- [Ardshinbank](ardshinbank.md) individual accounts
- [ACBA](acba.md) accounts
- [Generic](generic.md) manually/custom-mapped CSV files

Banks usually send transactions/statements by email monthly or yearly and allow downloading transaction lists on their websites.

**Note:** files received via email are usually less usable (except Ardshinbank) because they may be password-protected or lack Receiver/Payer account numbers. That makes analysis harder:

1. Password protection is difficult to handle automatically.
2. Account-based categorization won't work.
3. Transfers between your own accounts can't be detected and will be counted as "other income" and "other expense", distorting statistics.
4. Beancount reports cannot be built.

## Download transaction files in the app

For **MyAmeria** and **AmeriaBank (business)**, the easiest path is the [Files](/files) page:

1. **Download settings** — save Client ID, since date, and output folder in `config.yaml` (no secrets).
2. **Download now** — paste a fresh **Authorization** token (MyAmeria) or **Cookie** (Ameria Business) from browser DevTools. Credentials are used once and **not saved**.
3. Per-bank docs explain how to copy values from DevTools — linked from each source row and from the download popup.

Other banks: download files manually from the bank website and place them next to the app (see per-bank pages below).

## Where to find instructions

- **In the app:** [Files](/files) → **Download settings** / **Download now**, and per-source links to `/docs/bank/{sourceId}`.
- **In the repository:** `docs/en/banks/` (English) and `docs/ru/banks/` (Russian).
- **Automated banks:** [MyAmeria](myameria.md) and [AmeriaBank business](ameria-business.md) — UI download first, then manual website export, then optional CLI.

To add support for a new bank, please [create an issue](https://github.com/AlexanderMakarov/am-budget-view/issues) with an example transaction file and instructions on how you obtained it. The file may be redacted to hide sensitive information but should keep the same format, length, and character set as the original.
