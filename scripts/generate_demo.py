#!/usr/bin/env python3
import dataclasses
import datetime
import enum
import os
import random
import string
import xml.etree.ElementTree as ET
from pathlib import Path
import yaml
import pandas as pd
import faker
from faker.providers import (
    bank, company, currency, date_time, person, phone_number
)


def parse_config(path: str = "../config.yaml") -> tuple[dict, str]:
    """Parse configuration from YAML file.

    Args:
        path: Path to config file.
    Returns:
        Tuple containing:
        - The parsed config dictionary
        - The path to the folder where the config file is located
    """
    # Try to find the config file in different locations
    config_path = Path(path)
    if not config_path.is_absolute():
        script_dir = Path(__file__).parent
        possible_paths = [
            config_path,  # As provided
            script_dir / config_path,  # Relative to script
            script_dir.parent / path,  # Relative to workspace root
        ]

        for p in possible_paths:
            if p.exists():
                config_path = p
                break
    print(f"Loading config from: {config_path.absolute()}")
    with open(config_path, "r") as f:
        return yaml.safe_load(f), str(config_path.absolute().parent)


@dataclasses.dataclass
class TypeDesc:
    n_per_day: float
    """Number of transactions per day."""
    in_out_ratio: float
    """Ratio of a number of income transactions to expenses.
    0.1 - 1 income to 10 expenses.
    0.5 - in and out in pairs.
    0.95 - 20 income transactions to 1 expense.
    """
    average_sum_usd: float
    """Average sum of transaction in USD (need convert to account currency)."""
    variance_usd: float
    """Variance of transaction sum in USD. Smaller value - closer to average.

    E.g. if average_sum_usd=100 and variance_usd=0.3, then:
    - 68% of amounts will be between $70-$130,
    - 95% of amounts will be between $40-$160,
    - 99.7% of amounts will be between $10-$190,
    - 99.99% of amounts will be between $0-$200.

    If variance_usd is 0, then all amounts will be exactly average_sum_usd.
    """
    accounts_per_category: int
    """Number of accounts to generate at start per category."""
    new_accounts_per_transaction: float
    """How much new accounts appear in pool for each transaction.
    0 - same accounts always.
    0.1 - each 10 transactions new account appears.
    1 - new account each transaction.
    """
    remove_accounts_ratio_per_transaction: float
    """How much accounts are removed from pool for each transaction.
    0 - same accounts always.
    0.1 - each 10 transactions one account is removed.
    1 - one account is removed once per transaction.
    """
    expense_categories: list[str]
    """Categories for expense transactions."""
    income_categories: list[str]
    """Categories for income transactions."""


class TaskType(enum.Enum):
    EVERYDAY = TypeDesc(
        n_per_day=1,  # In average 1 transaction per day.
        in_out_ratio=0.05,  # I.e. big transfer from "salary" account and next small expenses.
        average_sum_usd=3,
        variance_usd=0.5,
        accounts_per_category=10,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_categories=[
            "Groceries",
            "Entertainment",
            "Health",
            "Pharmacies",
            "Taxi",
            "Online shopping",
            "Subscriptions",
        ],
        income_categories=["Transfer between my accounts", "Salary"],
    )
    BIGEVENTS = TypeDesc(
        n_per_day=0.1,  # Sometimes
        in_out_ratio=0.4,  # Less than 2 expenses per one income.
        average_sum_usd=500,
        variance_usd=0.5,
        accounts_per_category=3,
        new_accounts_per_transaction=0.8,  # Many new accounts per transaction.
        remove_accounts_ratio_per_transaction=0.1,  # Accounts disappear rarely.
        expense_categories=[
            "Cash",
            "Entertainment",
            "Health",
            "Online shopping",
        ],
        income_categories=["Transfer between my accounts"],
    )
    CURCONVERSIONS = TypeDesc(
        n_per_day=0.15,  # More often than twice per month.
        in_out_ratio=0.5,  # Payments are passed through the account 1:1.
        average_sum_usd=500,
        variance_usd=0.2,
        accounts_per_category=2,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0,
        expense_categories=["Transfer between my accounts"],
        income_categories=["Transfer between my accounts"],
    )
    SALARY = TypeDesc(
        n_per_day=0.033,  # Twice per month to get and immeditately out to card.
        in_out_ratio=0.5,  # Payments are passed through the account 1:1.
        average_sum_usd=2000,
        variance_usd=0.2,
        accounts_per_category=2,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_categories=["Transfer between my accounts"],
        income_categories=["Salary"],
    )
    UTILITIES = TypeDesc(
        n_per_day=0.13,  # 5 payments per month plus 2 times transfer from income account.
        in_out_ratio=2 / 6.0,  # Put money twice and spend on rent, water, gas, phone, internet, etc.
        average_sum_usd=1000,
        variance_usd=0.2,
        accounts_per_category=3,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_categories=["Utilities and rent"],
        income_categories=["Transfer between my accounts"],
    )


@dataclasses.dataclass
class Task:
    end_date: datetime.datetime = datetime.datetime.now()
    days_back: int = 30
    type: TaskType = TaskType.EVERYDAY
    currency: str = "AMD"

    def _to_suffix(self) -> str:
        return f"{self.type.name}_{self.currency}_{self.days_back}"


class BaseGenerator:
    def __init__(self, glob_pattern: str, config: dict):
        self.glob_pattern = glob_pattern
        self.config = config
        self.random = random.Random()
        self.fake = faker.Faker()
        self.fake.add_provider(bank)
        self.fake.add_provider(company)
        self.fake.add_provider(currency)
        self.fake.add_provider(date_time)
        self.fake.add_provider(person)
        self.fake.add_provider(phone_number)
        # Parse transaction categories from config groups.
        self.substring_per_category = {}
        if "groups" in config:
            for category, entry in config["groups"].items():
                if "substrings" in entry:
                    self.substring_per_category[category] = entry["substrings"]

    def generate(self, folder: str, task: Task) -> str:
        """Generate a statement file for a given task.

        Args:
            folder: Path of the folder to save the statement.
            task: Task to generate the statement for.
        Returns:
            str: Message about the generated statement.
        """
        file_path = os.path.join(folder, self._get_file_name(task._to_suffix()))
        print(f"{self.glob_pattern}: Generating {file_path} with {task}")
        # Generate accounts pool and datetimes.
        accounts_pool = self._generate_accounts_and_categories(task.type)
        datetimes = self._generate_datetimes(task)
        result = self._generate(file_path, task, accounts_pool, datetimes)
        print(f"{self.glob_pattern}: ✓ {result} at {file_path}")
        return file_path

    def _generate(
        self,
        file_path: str,
        task: Task,
        accounts_pool: dict[str, dict[str, list[str]]],
        datetimes: list[datetime.datetime],
    ) -> str:
        raise NotImplementedError("Subclasses must implement this method")

    def _get_file_name(self, suffix: str) -> str:
        pattern = str(self.config.get(self.glob_pattern))
        if pattern is None:
            raise ValueError(f"'{self.glob_pattern}' wasn't found in config.")
        return pattern.replace("*", suffix)

    def _generate_datetimes(self, task: Task) -> list[datetime.datetime]:
        transactions_count = int(task.days_back * task.type.value.n_per_day * (self.random.random() + 0.5))
        dates = []
        # Generate `transactions_count` random dates in the range.
        for _ in range(transactions_count):
            dates.append(task.end_date - datetime.timedelta(
                days=self.random.randint(0, task.days_back),
                hours=self.random.randint(-6, 6),
            ))
        return dates

    def _filter_valid_categories(self, categories: list[str]) -> list[str]:
        """Filter categories to only include those that exist."""
        return [
            cat for cat in categories 
            if cat in self.substring_per_category
        ]

    def _generate_account_number(self) -> str:
        """Generate a 16-digit account number."""
        return "".join(random.choices(string.digits, k=16))

    def _generate_accounts_and_categories(
        self,
        task_type: TaskType,
    ) -> dict[str, dict[str, list[str]]]:
        """Generate initial accounts pool and categories for the given task.

        Args:
            type: Account type.
        Returns:
            Dict with "income" and "expense" keys.
            Each key has "categories" and "accounts" keys.
            "categories" is a list of categories.
            "accounts" is a list of account numbers.
        """
        v = task_type.value
        in_categories = self._filter_valid_categories(v.income_categories)
        out_categories = self._filter_valid_categories(v.expense_categories)
        result = {
            "income": {
                "categories": in_categories,
                "accounts": [
                    self._generate_account_number()
                    for _ in range(v.accounts_per_category)
                ],
            },
            "expense": {
                "categories": out_categories,
                "accounts": [
                    self._generate_account_number()
                    for _ in range(v.accounts_per_category)
                ],
            },
        }
        return result

    def _convert_amount_to_currency(self, amount: float, currency: str) -> float:
        """Convert amount to given currency."""
        match currency:
            case "USD":
                return round(amount, 2)  # USD
            case "AMD":
                return round(amount * 400, 2)  # Approximate AMD/USD rate
            case "EUR":
                return round(amount * 0.9, 2)  # Approximate EUR/USD rate
            case "RUB":
                return round(amount * 80, 2)  # Approximate RUB/USD rate
            case "TRY":
                return round(amount * 20, 2)  # Approximate TRY/USD rate
            case "GBP":
                return round(amount * 0.8, 2)  # Approximate GBP/USD rate
            case "AED":
                return round(amount * 3.67, 2)  # Approximate AED/USD rate
            case _:
                raise ValueError(f"Unsupported currency: {currency}")

    def _generate_main_components(
        self,
        task: Task,
        accounts_pool: dict[str, dict[str, list[str]]],
        is_income: bool,
        file_account_number: str,
    ) -> tuple[str, str, float, str]:
        """Generate a transaction accounts, amount and description.

        Args:
            task: Task to generate the transaction for.
            accounts_pool: Accounts pool to choose from.
            is_income: Whether the transaction is income.
            file_account_number: Account number of the file.

        Returns:
            A tuple of:
            - str payer_account,
            - str receiver_account,
            - float amount,
            - str description,
        """
        # Find accounts for transaction.
        ac_key = "income"
        if is_income:
            payer_account = self.random.choice(accounts_pool[ac_key]["accounts"])
            receiver_account = file_account_number
        else:
            ac_key = "expense"
            payer_account = file_account_number
            receiver_account = self.random.choice(accounts_pool[ac_key]["accounts"])

        # Generate amount. Make it not negative.
        average_sum_usd = task.type.value.average_sum_usd
        base_amount = self.random.gauss(
            mu=average_sum_usd,
            sigma=average_sum_usd * task.type.value.variance_usd,
        )
        if base_amount < 0:
            base_amount = -base_amount
        amount = self._convert_amount_to_currency(base_amount, task.currency)

        # Get description from the category.
        category = self.random.choice(
            accounts_pool[ac_key]["categories"]
        )
        descriptions = self.substring_per_category.get(category, ["Payment"])
        description = self.random.choice(descriptions)

        # Add some unique details to make transactions more varied
        if category in ["Groceries", "Entertainment", "Pharmacies", "Health"]:
            # Add amount details for purchases
            items = self.random.randint(1, 5)
            if self.random.random() < 0.3:  # Sometimes add item details
                description += f" {items} ITEMS"

        # Add location or reference numbers sometimes
        if self.random.random() < 0.4:
            if category != "Salary":
                # Add location for non-salary transactions
                description += f" {self.fake.city()[:10]}"
            else:
                # Add reference for salary
                description += f" REF:{self.fake.bothify('????###')}"
        # Add transaction ID sometimes.
        if self.random.random() < 0.3:
            description += f" ID:{self.fake.bothify('#######')}"

        # Recalculate accounts pool.
        if self.random.random() < task.type.value.new_accounts_per_transaction:
            accounts_pool[ac_key]["accounts"].append(self._generate_account_number())
        if self.random.random() < task.type.value.remove_accounts_ratio_per_transaction:
            accounts = accounts_pool[ac_key]["accounts"]
            idx_to_remove = self.random.randint(0, len(accounts) - 1)
            accounts_pool[ac_key]["accounts"] = [
                accounts[i]
                for i in range(len(accounts))
                if i != idx_to_remove
            ]
        # Return category, description and amount.
        return payer_account, receiver_account, amount, description


class InecobankXmlGenerator(BaseGenerator):
    def __init__(self, config: dict):
        super().__init__("inecobankStatementXmlFilesGlob", config)

    def _format_date(self, date: datetime.datetime) -> str:
        return date.strftime("%d/%m/%Y")

    def _generate(
        self,
        file_path: str,
        task: Task,
        accounts_pool: list[str],
        datetimes: list[datetime.datetime],
    ) -> str:
        root = ET.Element("Statement")
        # Make head nodes.
        ET.SubElement(root, "Client").text = self.fake.name()
        file_account_number = self._generate_account_number()
        ET.SubElement(root, "AccountNumber").text = file_account_number
        ET.SubElement(root, "Currency").text = task.currency

        # Format Period node.
        sorted_datetimes = sorted(datetimes)
        start_date = sorted_datetimes[0].strftime("%d/%m/%Y")
        end_date = sorted_datetimes[-1].strftime("%d/%m/%Y")
        ET.SubElement(root, "Period").text = f"[{start_date} - {end_date}]"

        # Generate random opening balance and add node for closing balance.
        opening_balance = self._convert_amount_to_currency(
            self.random.randint(10, 100) * task.type.value.average_sum_usd,
            task.currency,
        )
        ET.SubElement(root, "Openingbalance").text = f"{opening_balance:,.2f}"
        closing_balance_node = ET.SubElement(root, "Closingbalance")
        closing_balance = opening_balance

        operations = ET.SubElement(root, "Operations")
        transactions_count = 0

        for current_datetime in datetimes:
            operation = ET.SubElement(operations, "Operation")
            # Generate a random transaction header.
            ET.SubElement(operation, "n-n").text = "".join(self.random.choices(string.digits, k=9))
            ET.SubElement(operation, "Number").text = "".join(self.random.choices(string.digits, k=10))
            ET.SubElement(operation, "Date").text = current_datetime.strftime("%d/%m/%Y")
            ET.SubElement(operation, "Currency").text = task.currency

            # Generate transaction direction, amount, category, description.
            is_income = self.random.random() < task.type.value.in_out_ratio
            p_account, r_account, amount, desc = self._generate_main_components(
                task, accounts_pool, is_income, file_account_number
            )

            # Set Income/Expense values based on direction
            if is_income:
                ET.SubElement(operation, "Income").text = f"{amount:,.2f}"
                ET.SubElement(operation, "Expense").text = "0.00"
            else:
                ET.SubElement(operation, "Income").text = "0.00"
                ET.SubElement(operation, "Expense").text = f"{amount:,.2f}"

            # Generate receiver/payer details
            ET.SubElement(operation, "Receiver-PayerAccount").text = (
                p_account if is_income else r_account
            )

            # Use first word as receiver/payer name
            receiver_payer = desc.split(" ")[0] if " " in desc else desc
            ET.SubElement(operation, "Receiver-Payer").text = receiver_payer

            # Create a more detailed transaction description
            details = (
                f"{desc}, Անկանխիկ գործարք, "
                f"{file_account_number[:4]}***{file_account_number[-3:]}, "
                f"{receiver_payer}({task.currency}), "
                f"{current_datetime.strftime('%d/%m/%Y %H:%M:%S')}"
            )
            ET.SubElement(operation, "Details").text = details

            # Update loop variables.
            closing_balance += amount
            transactions_count += 1

        closing_balance_node.text = f"{closing_balance * 1000:,.2f}"
        tree = ET.ElementTree(root)
        tree.write(file_path, encoding="utf-8", xml_declaration=True)
        return f"Generated Inecobank XML statement with {transactions_count} transactions"


class InecobankExcelGenerator(BaseGenerator):
    def __init__(self, config: dict):
        super().__init__("inecobankStatementXlsxFilesGlob", config)

    def generate(self, path: os.PathLike, task: Task) -> str:
        data = []
        current_date = task.start_date
        while current_date <= task.end_date:
            if self.random.random() < task.type.value.n_per_day:
                # Determine if this is income or expense
                is_income = self.random.random() < task.type.value.in_out_ratio

                # Generate description based on account type and transaction direction
                category, desc, amount = self._generate_main_components(
                    task.type, task.currency, task.type.value.average_sum_usd, is_income
                )

                data.append(
                    {
                        "Date": current_date,
                        "Amount": amount,
                        "Description": desc,
                        "Category": category,
                        "Account": self._generate_account_number(),
                    }
                )
            current_date += datetime.timedelta(days=1)

        df = pd.DataFrame(data)
        df.to_excel(path, index=False)
        return f"Generated Inecobank Excel statement with {len(data)} transactions"


class AmeriaCsvGenerator(BaseGenerator):
    def __init__(self, config: dict):
        super().__init__("ameriaCsvFilesGlob", config)

    def _format_date(self, date: datetime.datetime) -> str:
        return date.strftime("%d/%m/%Y")

    def _format_money(self, amount: float) -> str:
        return f"{amount:,.2f}"

    def _generate_transaction_type(self, description: str) -> str:
        """Generate transaction type based on description."""
        if "Currency Exchange" in description:
            return "CEX"
        elif "Card Replenishment" in description:
            return "TRF"
        elif "fee" in description.lower() or "Fee" in description:
            return "FEE"
        else:
            return "MSC"

    def _generate(
        self,
        file_path: str,
        task: Task,
        accounts_pool: dict[str, dict[str, list[str]]],
        datetimes: list[datetime.datetime],
    ) -> str:
        # Generate account number for the file
        file_account_number = self._generate_account_number()
        client_name = self.fake.name().upper()

        # Sort datetimes to determine period
        sorted_datetimes = sorted(datetimes)
        start_date = sorted_datetimes[0] if sorted_datetimes else task.end_date
        end_date = sorted_datetimes[-1] if sorted_datetimes else task.end_date

        # Generate random opening balance
        average_sum_usd = task.type.value.average_sum_usd
        opening_balance = self._convert_amount_to_currency(
            self.random.random() * average_sum_usd * 100,
            task.currency,
        )
        closing_balance = opening_balance  # If we won't get transactions.

        # Create CSV content as list of rows.
        csv_rows = [
            ["Start of Period", None, None, self._format_date(start_date)],
            ["End of Period", None, None, self._format_date(end_date)],
            ["Account No.", None, None, file_account_number, client_name],
            ["Currency", None, None, task.currency, f"{task.currency} currency"],
            ["TIN", None, None, str(self.random.randint(10000000, 99999999))],
            [],  # Empty row
        ]

        # Generate transactions data
        transactions = []
        total_debit = 0.0
        total_credit = 0.0

        prev_amount = None
        for current_datetime in datetimes:
            # Generate transaction direction, amount, category, description
            is_income = self.random.random() < task.type.value.in_out_ratio
            p_account, r_account, amount, desc = self._generate_main_components(
                task, accounts_pool, is_income, file_account_number
            )
            # Keep amount similar to previous transaction
            # to avoid negative closing balance.
            if prev_amount:
                ratio = prev_amount / amount
                if ratio > 1.5:
                    amount = prev_amount
                elif ratio < 0.5:
                    amount = prev_amount
            prev_amount = amount

            # Generate document number
            doc_no = "".join(self.random.choices(string.digits, k=6))

            # Generate transaction type
            trans_type = self._generate_transaction_type(desc)

            # Set debit/credit amounts
            if is_income:
                debit_amount = 0.0
                credit_amount = amount
                total_credit += amount
                closing_balance += amount
            else:
                debit_amount = amount
                credit_amount = 0.0
                total_debit += amount
                closing_balance -= amount

            # Create transaction description with Armenian text
            if "Transfer between my accounts" in desc:
                if is_income:
                    desc = "Card Replenishment"
                else:
                    desc = (
                        f"{desc} {file_account_number[:4]}***{file_account_number[-3:]}"
                    )
            else:
                if self.random.random() < 0.3:
                    desc += (
                        f"\\Purchase POS {self.fake.city()[:8].upper()}"
                    )

            transactions.append({
                "Date": self._format_date(current_datetime),
                "Doc.No.": doc_no,
                "Type": trans_type,
                "Account": p_account if is_income else r_account,
                "Details": desc,
                "Debit": self._format_money(debit_amount),
                "Credit": self._format_money(credit_amount),
                "Remitter/Beneficiary": ""  # Usually empty.
            })

        # Add balance information
        csv_rows.append([
            f"Opening Balance on {self._format_date(start_date)}", None, None,
            self._format_money(opening_balance)
        ])
        csv_rows.append([
            "Debit Turnover", None, None, self._format_money(total_debit)
        ])
        csv_rows.append([
            "Credit Turnover", None, None, self._format_money(total_credit)
        ])
        csv_rows.append([
            f"Closing Balance on {self._format_date(end_date)}", None, None,
            self._format_money(closing_balance)
        ])
        csv_rows.append([
            "Closing Available Balance", None, None,
            self._format_money(closing_balance)
        ])
        csv_rows.append([])  # Empty row

        # Add transaction headers
        csv_rows.append([
            "Date", "Doc.No.", "Type", "Account", "Details",
            "Debit", "Credit", "Remitter/Beneficiary"
        ])

        # Add transaction data
        for transaction in transactions:
            csv_rows.append([
                transaction["Date"],
                transaction["Doc.No."],
                transaction["Type"],
                transaction["Account"],
                transaction["Details"],
                transaction["Debit"],
                transaction["Credit"],
                transaction["Remitter/Beneficiary"]
            ])

        # Add footer
        csv_rows.append([])  # Empty row
        csv_rows.append([
            "Days Count", None, None, str(task.days_back)
        ])

        # Write CSV file with tab delimiter and UTF-16 encoding.
        csv_content = []
        for row in csv_rows:
            # Convert None values to empty strings and format the row
            formatted_row = []
            for cell in row:
                if isinstance(cell, str):
                    formatted_row.append(f'"{cell}"')
                else:
                    formatted_row.append(str(cell))
            csv_content.append('\t'.join(formatted_row))
        # Join all rows with newlines
        csv_text = '\n'.join(csv_content)
        # Write file in UTF-16 with BOM
        with open(file_path, 'w', encoding='utf-16', newline='') as csvfile:
            csvfile.write(csv_text)
        return (
            f"Generated Ameria CSV statement with {len(transactions)} "
            "transactions"
        )


class MyAmeriaHistoryGenerator(BaseGenerator):
    def __init__(self, config: dict):
        super().__init__("myAmeriaHistoryXlsFilesGlob", config)

    def generate(self, path: str, task: Task) -> str:
        data = []
        current_date = task.start_date
        while current_date <= task.end_date:
            if self.random.random() < task.type.value.n_per_day:
                # Determine if this is income or expense
                is_income = self.random.random() < task.type.value.in_out_ratio

                # Generate description based on account type and transaction direction
                category, desc, amount = self._generate_main_components(
                    task.type,
                    task.currency,
                    task.type.value.average_sum_usd,
                    is_income,
                )

                data.append(
                    {
                        "Date": current_date,
                        "Amount": amount,
                        "Description": desc,
                        "Category": category,
                        "Account": self._generate_account_number(),
                        "Beneficiary Account": self._generate_account_number(),
                    }
                )
            current_date += datetime.timedelta(days=1)

        df = pd.DataFrame(data)
        # Use .xlsx extension instead of .xls for newer Excel format
        output_path = path
        if path.endswith(".xls"):
            output_path = path.replace(".xls", ".xlsx")
        df.to_excel(output_path, index=False)
        return f"Generated MyAmeria History Excel with {len(data)} transactions"


class GenericCsvGenerator(BaseGenerator):
    def __init__(self, config: dict):
        super().__init__("genericCsvFilesGlob", config)

    def generate(self, path: str, task: Task) -> str:
        data = []
        current_date = task.start_date
        while current_date <= task.end_date:
            if self.random.random() < task.type.value.n_per_day:
                # Determine if this is income or expense
                is_income = self.random.random() < task.type.value.in_out_ratio
                category, desc, amount = self._generate_main_components(
                    task.type,
                    task.currency,
                    task.type.value.average_sum_usd,
                    is_income,
                )
                data.append(
                    {
                        "Date": current_date,
                        "Amount": amount,
                        "Description": desc,
                        "Category": category,
                        "Account": self._generate_account_number(),
                        "Currency": task.currency,
                    }
                )
            current_date += datetime.timedelta(days=1)

        df = pd.DataFrame(data)
        df.to_csv(path, index=False)
        return f"Generated Generic CSV with {len(data)} transactions"


if __name__ == "__main__":
    config, target_folder = parse_config("tmp-demo.yaml")
    # Create a demo directory if it doesn't exist
    # demo_dir = os.path.join(target_folder, "demo")
    # os.makedirs(demo_dir, exist_ok=True)
    print(f"Generating demo files in '{target_folder}':")

    # "inecobankStatementXmlFilesGlob": InecobankXmlGenerator(config),
    # 'inecobankStatementXlsxFilesGlob': InecobankExcelGenerator(config),
    # 'ameriaCsvFilesGlob': AmeriaCsvGenerator(config),
    # 'myAmeriaHistoryXlsFilesGlob': MyAmeriaHistoryGenerator(config),
    # 'genericCsvFilesGlob': GenericCsvGenerator(config)

    InecobankXmlGenerator(config).generate(target_folder, Task(
        days_back=365,
        type=TaskType.EVERYDAY,
        currency="AMD",
    ))
    AmeriaCsvGenerator(config).generate(target_folder, Task(
        days_back=300,
        type=TaskType.CURCONVERSIONS,
        currency="AMD",
    ))
