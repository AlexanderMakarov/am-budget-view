#!/usr/bin/env python3
import csv
import dataclasses
import datetime
import enum
import fnmatch
import os
import random
import string
from collections import namedtuple
import xml.etree.ElementTree as ET
from pathlib import Path
import yaml
import faker
import glob
from faker.providers import (
    bank, company, currency, date_time, person, phone_number
)
from openpyxl import Workbook


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
    average_sum_in_usd: float
    """Average sum of income transaction in USD."""
    average_sum_out_usd: float
    """Average sum of expense transaction in USD."""
    sum_variance: float
    """Variance of transaction amount. Smaller value - closer to average.

    E.g. if average_sum_in_usd=100 and sum_variance=0.3, then:
    - 68% of amounts will be between $70-$130,
    - 95% of amounts will be between $40-$160,
    - 99.7% of amounts will be between $10-$190,
    - 99.99% of amounts will be between $0-$200.

    If sum_variance is 0, then all amounts will be exactly average_sum_in_usd.
    """
    balance_min_usd: int
    """Minimal balance in USD.

    If transaction amount exceeds this value, it would be switched to income.
    """
    balance_max_usd: int
    """Maximum balance in USD.

    If transaction amount exceeds this value, it would be switched to expense.
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
    expense_my_to_foreign_accounts_ratio: float
    """Ratio of expense transactions receiver accounts.

    0 - no expense transactions to my accounts.
    0.8 - 80% of expense transactions to my accounts.
    1 - all expense transactions to my accounts.
    """
    income_my_to_foreign_accounts_ratio: float
    """Ratio of income transactions payer accounts.

    0 - no income transactions from my accounts.
    0.1 - 10% of income transactions from my accounts.
    1 - all income transactions from my accounts.
    """
    expense_categories: list[str]
    """Categories for expense transactions.
    
    Put one category multiple times to generate more transactions.
    """
    income_categories: list[str]
    """Categories for income transactions.
    
    Put one category multiple times to generate more transactions.
    """
    other_currencies_ratio: float = 0.0
    """Ratio of transactions in other currencies.

    0 - no transactions in other currencies.
    0.1 - 10% of transactions in other currencies.
    1 - all transactions in other currencies.
    """


class TaskType(enum.Enum):
    EVERYDAY = TypeDesc(
        n_per_day=1,  # In average 1 transaction per day.
        in_out_ratio=0.05,  # I.e. big transfer from "salary" account and next small expenses.
        average_sum_in_usd=500,
        average_sum_out_usd=5,
        sum_variance=0.5,
        balance_min_usd=0,
        balance_max_usd=1000,
        accounts_per_category=10,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_my_to_foreign_accounts_ratio=0.1,
        income_my_to_foreign_accounts_ratio=0.9,
        expense_categories=[
            "Groceries",
            "Groceries",
            "Groceries",
            "Entertainment",
            "Health",
            "Pharmacies",
            "Taxi",
            "Online shopping",
            "Subscriptions",
        ],
        income_categories=[
            "Transfer between my accounts",
            "Transfer between my accounts",
            "Transfer between my accounts",
            "Transfer between my accounts",
            "Transfer between my accounts",
            "Transfer between my accounts",
            "Taxi",
            "Online shopping",
        ],
    )
    BIGEVENTS = TypeDesc(
        n_per_day=0.1,  # Sometimes
        in_out_ratio=0.4,  # Less than 2 expenses per one income.
        average_sum_in_usd=1000,
        average_sum_out_usd=600,
        sum_variance=0.5,
        balance_min_usd=0,
        balance_max_usd=10000,
        accounts_per_category=3,
        new_accounts_per_transaction=0.8,  # Many new accounts per transaction.
        remove_accounts_ratio_per_transaction=0.1,  # Accounts disappear rarely.
        expense_my_to_foreign_accounts_ratio=0,  # All expenses to foreign accounts.
        income_my_to_foreign_accounts_ratio=1,  # All incomes from my accounts.
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
        average_sum_in_usd=500,
        average_sum_out_usd=500,
        sum_variance=0.2,
        balance_min_usd=0,
        balance_max_usd=10000,
        accounts_per_category=2,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0,
        expense_my_to_foreign_accounts_ratio=1,  # All expenses to my accounts.
        income_my_to_foreign_accounts_ratio=1,  # All incomes from my accounts.
        expense_categories=["Transfer between my accounts"],
        income_categories=["Transfer between my accounts"],
    )
    SALARY = TypeDesc(
        n_per_day=0.2,  # 3 times per month to get stable rate.
        in_out_ratio=0.5,  # Payments are passed through the account 1:1.
        average_sum_in_usd=500,
        average_sum_out_usd=500,
        sum_variance=0.2,
        balance_min_usd=0,
        balance_max_usd=4000,
        accounts_per_category=2,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_my_to_foreign_accounts_ratio=1,  # All expenses to my accounts.
        income_my_to_foreign_accounts_ratio=0,  # Only from foreign accounts.
        expense_categories=["Transfer between my accounts"],
        income_categories=["Salary"],
    )
    UTILITIES = TypeDesc(
        n_per_day=0.2,  # 5 payments per month plus 2 times transfer from income account.
        in_out_ratio=2 / 6.0,  # Put money twice and spend on rent, water, gas, phone, internet, etc.
        average_sum_in_usd=300,
        average_sum_out_usd=100,
        sum_variance=0.5,
        balance_min_usd=0,
        balance_max_usd=1000,
        accounts_per_category=3,
        new_accounts_per_transaction=0.01,
        remove_accounts_ratio_per_transaction=0.01,
        expense_my_to_foreign_accounts_ratio=0,  # All expenses to foreign accounts.
        income_my_to_foreign_accounts_ratio=1,  # Only from my accounts.
        expense_categories=["Utilities and rent"],
        income_categories=["Transfer between my accounts"],
    )


@dataclasses.dataclass(frozen=True)
class Task:
    generator_class: type["BaseGenerator"]
    end_date: datetime.datetime = datetime.datetime.now()
    days_back: int = 30
    type: TaskType = TaskType.EVERYDAY
    currency: str = "AMD"
    other_currencies: list[str] = dataclasses.field(default_factory=list)

    def _to_suffix(self) -> str:
        return f"{self.type.name}_{self.currency}_{self.days_back}"


@dataclasses.dataclass
class TaskContext:
    task: Task
    account_number: str
    other_my_account_numbers: list[str]
    account_currency: str
    income_categories: list[str]
    expense_categories: list[str]
    income_accounts: list[str]
    expense_accounts: list[str]
    opening_balance: float
    current_balance: float
    transactions_count: int = 0


Transaction = namedtuple(
    "Transaction", [
        "payer_account",
        "receiver_account",
        "is_income",
        "account_currency",
        "account_amount",
        "origin_currency",
        "origin_amount",
        "description",
    ])


def convert_usd_amount_to_currency(amount: float, currency: str) -> float:
    """Convert amount in USD to given currency."""
    match currency:
        case "USD":
            return round(amount, 2)  # USD
        case "AMD":
            return round(amount * 390, 2)  # Approximate AMD/USD rate
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


def format_date_to_dmy(date: datetime.datetime) -> str:
    return date.strftime("%d/%m/%Y")


class ContextManager:
    def __init__(self, config: dict):
        self.config = config
        self.random = random.Random()
        # Parse transaction categories from config groups.
        self.substring_per_category = {}
        if "groups" in config:
            for category, entry in config["groups"].items():
                if "substrings" in entry:
                    self.substring_per_category[category] = entry["substrings"]

    def _filter_valid_categories(self, categories: list[str]) -> list[str]:
        """Filter categories to only include those that exist."""
        return [
            cat for cat in categories
            if cat in self.substring_per_category
        ]

    def generate_account_number(self) -> str:
        """Generate a 16-digit account number."""
        return "".join(self.random.choices(string.digits, k=16))

    def generate_task_contexts(self, tasks: list[Task]) -> list[TaskContext]:
        """Generate task context with accounts pool, categories, balances.

        Args:
            tasks: Tasks to generate the context for.
        Returns:
            List of `TaskContext`-s.
        """
        my_account_numbers = [self.generate_account_number() for _ in tasks]
        result = []
        for task, account_number in zip(tasks, my_account_numbers):
            other_my_account_numbers = my_account_numbers.copy()
            other_my_account_numbers.remove(account_number)
            v = task.type.value
            in_categories = self._filter_valid_categories(v.income_categories)
            out_categories = self._filter_valid_categories(v.expense_categories)
            opening_balance = convert_usd_amount_to_currency(
                v.balance_min_usd + self.random.random() * v.balance_max_usd,
                task.currency,
            )
            task_context = TaskContext(
                task=task,
                account_number=account_number,
                other_my_account_numbers=other_my_account_numbers,
                account_currency=task.currency,
                income_categories=in_categories,
                expense_categories=out_categories,
                income_accounts=[
                    self.generate_account_number()
                    for _ in range(v.accounts_per_category - 1)
                ],
                expense_accounts=[
                    self.generate_account_number()
                    for _ in range(v.accounts_per_category - 1)
                ],
                opening_balance=opening_balance,
                current_balance=opening_balance,
            )
            result.append(task_context)
        return result

    def execute_tasks(
        self,
        task_contexts: list[TaskContext],
        folder: str,
        remove_old: bool = False,
    ) -> list[str]:
        """Execute tasks and return list of generated file paths.
        
        Args:
            tasks: List of tasks to execute.
            folder: Path of the folder to save the statements to match globs
            in config.
            remove_old: Remove old files in the folder.
        """
        result = []
        seen_generator_classes = set()
        for task_context in task_contexts:
            generator_class = task_context.task.generator_class
            current_remove_old = remove_old
            if generator_class in seen_generator_classes:
                current_remove_old = False
            seen_generator_classes.add(generator_class)
            generator = generator_class(self)
            result.append(
                generator.generate(
                    folder,
                    task_context,
                    current_remove_old,
                ),
            )
        return result


class BaseGenerator:
    def __init__(self, glob_pattern: str, context_manager: ContextManager):
        self.glob_pattern = glob_pattern
        self.cm = context_manager
        self.random = random.Random()
        self.fake = faker.Faker()
        self.fake.add_provider(bank)
        self.fake.add_provider(company)
        self.fake.add_provider(currency)
        self.fake.add_provider(date_time)
        self.fake.add_provider(person)
        self.fake.add_provider(phone_number)

    def _get_file_name(self, suffix: str) -> str:
        pattern = str(self.cm.config.get(self.glob_pattern))
        if pattern is None:
            raise ValueError(f"'{self.glob_pattern}' wasn't found in config.")
        return pattern.replace("*", f"_{suffix}_")

    def generate(
        self,
        folder: str,
        task_context: TaskContext,
        remove_old: bool = False,
    ) -> str:
        """Generate a statement file for a given task.

        Args:
            folder: Path of the folder to save the statement.
            task_context: Pre-generated task context with accounts pool.
            remove_old: Remove old files in the folder.
        Returns:
            str: File path of the generated statement.
        """
        task: Task = task_context.task
        if remove_old:
            pattern = str(self.cm.config.get(self.glob_pattern))
            full_glob = os.path.join(folder, pattern)
            for file_path in glob.glob(full_glob):
                os.remove(file_path)
                print(f"{self.glob_pattern}: Removed {os.path.basename(file_path)}")
        file_path = os.path.join(folder, self._get_file_name(task._to_suffix()))
        print(f"{self.glob_pattern}: Generating {file_path} for {task_context.task}")
        # Generate datetimes.
        datetimes = self._generate_datetimes(task)
        result = self._generate(file_path, task_context, datetimes)
        print(f"{self.glob_pattern}: ✓ {result} at {file_path}")
        return file_path

    def _generate(
        self,
        file_path: str,
        task_context: TaskContext,
        datetimes: list[datetime.datetime],
    ) -> str:
        raise NotImplementedError("Subclass must implement this method")

    def _generate_datetimes(self, task: Task) -> list[datetime.datetime]:
        # Start from the earliest date
        start_date = task.end_date - datetime.timedelta(days=task.days_back)
        # Calculate base interval to achieve desired transaction frequency
        # We want roughly n_per_day transactions, but with 2-week spacing
        base_interval_days = min(14, int(1 / task.type.value.n_per_day))
        # Generate dates with consistent intervals and jitter
        dates = []
        current_date = start_date
        while current_date < task.end_date:
            # Add jitter: +/- 20% of the interval
            jitter_days = self.random.randint(
                -int(base_interval_days * 0.2),
                int(base_interval_days * 0.2)
            )
            # Add some hour-level jitter too
            jitter_hours = self.random.randint(-12, 12)
            # Calculate next date with base interval and jitter
            next_date = current_date + datetime.timedelta(
                days=base_interval_days + jitter_days,
                hours=jitter_hours
            )
            # If we haven't exceeded the end date, add this date
            if next_date <= task.end_date:
                dates.append(next_date)
                current_date = next_date
            else:
                break
        return dates

    def _choose_account(
        self,
        category: str,
        my_account_numbers: list[str],
        foreign_account_numbers: list[str],
        my_to_foreign_ratio: float,
    ) -> str:
        if (
            category == "Transfer between my accounts"
            or self.random.random() < my_to_foreign_ratio
        ):
            accounts_list = my_account_numbers
        else:
            accounts_list = foreign_account_numbers
        if len(accounts_list) == 1:
            return accounts_list[0]
        return self.random.choice(accounts_list)

    def _generate_transaction(
        self,
        task_context: TaskContext,
    ) -> Transaction:
        """Generate a transaction accounts, amount and description.

        Args:
            task_context: Task context.

        Returns:
            A new `Transaction` instance.
        """
        task: Task = task_context.task

        # Decide if it's income or expense, generate base amount in USD.
        is_income = self.random.random() < task.type.value.in_out_ratio
        average_sum_usd = (
            task.type.value.average_sum_in_usd
            if is_income
            else task.type.value.average_sum_out_usd
        )
        usd_amount = -1
        while usd_amount < 0:
            usd_amount = self.random.gauss(
                mu=average_sum_usd,
                sigma=average_sum_usd * task.type.value.sum_variance,
            )
        amount = convert_usd_amount_to_currency(usd_amount, task.currency)
        # Check if balance is not exceeded.
        current_balance = task_context.current_balance
        expected_balance = (
            current_balance + amount
            if is_income
            else current_balance - amount
        )
        balance_min = convert_usd_amount_to_currency(
            task.type.value.balance_min_usd,
            task.currency,
        )
        balance_max = convert_usd_amount_to_currency(
            task.type.value.balance_max_usd,
            task.currency,
        )
        # Check for min balance.
        if expected_balance < balance_min:
            print(f"{self.glob_pattern}: Min balance exceeded: {expected_balance} < {balance_min}, is_income={is_income}")
            if is_income:
                amount = balance_min - current_balance
            else:
                is_income = True
        # Check for max balance.
        if expected_balance > balance_max:
            print(f"{self.glob_pattern}: Max balance exceeded: {expected_balance} > {balance_max}, is_income={is_income}")
            if is_income:
                is_income = False
            else:
                amount = balance_max - current_balance
        # In any case don't allow negative amount.
        if amount < 0:
            amount = -amount

        # Choose category and account for transaction.
        if is_income:
            category = self.random.choice(task_context.income_categories)
            payer_account = self._choose_account(
                category,
                task_context.other_my_account_numbers,
                task_context.income_accounts,
                task.type.value.income_my_to_foreign_accounts_ratio,
            )
            receiver_account = task_context.account_number
        else:
            category = self.random.choice(task_context.expense_categories)
            payer_account = task_context.account_number
            receiver_account = self._choose_account(
                category,
                task_context.other_my_account_numbers,
                task_context.expense_accounts,
                task.type.value.expense_my_to_foreign_accounts_ratio,
            )

        # Get description from the category.
        descriptions = self.cm.substring_per_category.get(category, ["Payment"])
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
            task_context.income_accounts.append(
                self.cm.generate_account_number()
            )
        if self.random.random() < task.type.value.remove_accounts_ratio_per_transaction:
            accounts = (
                task_context.income_accounts
                if is_income
                else task_context.expense_accounts
            )
            if len(accounts) > 1:
                idx_to_remove = self.random.randint(0, len(accounts) - 1)
                accounts = [
                    accounts[i]
                    for i in range(len(accounts))
                    if i != idx_to_remove
                ]
                if is_income:
                    task_context.income_accounts = accounts
                else:
                    task_context.expense_accounts = accounts

        # Update task context.
        task_context.transactions_count += 1
        task_context.current_balance += amount if is_income else -amount

        # Decide if it's in other currency.
        account_amount = amount
        origin_currency = task.currency
        origin_amount = usd_amount
        if (
            task.other_currencies
            and self.random.random() < task.type.value.other_currencies_ratio
        ):
            origin_currency = self.random.choice(task.other_currencies)
            origin_amount = convert_usd_amount_to_currency(usd_amount, origin_currency)
            account_amount = convert_usd_amount_to_currency(usd_amount, task.currency)

        return Transaction(
            is_income=is_income,
            payer_account=payer_account,
            receiver_account=receiver_account,
            account_currency=task.currency,
            account_amount=account_amount,
            origin_currency=origin_currency,
            origin_amount=origin_amount,
            description=description,
        )


class InecobankXmlGenerator(BaseGenerator):
    def __init__(self, context_manager: ContextManager):
        super().__init__("inecobankStatementXmlFilesGlob", context_manager)

    def _generate(
        self,
        file_path: str,
        task_context: TaskContext,
        datetimes: list[datetime.datetime],
    ) -> str:
        root = ET.Element("Statement")
        # Make head nodes.
        ET.SubElement(root, "Client").text = self.fake.name()
        file_account_number = task_context.account_number
        ET.SubElement(root, "AccountNumber").text = file_account_number
        ET.SubElement(root, "Currency").text = task_context.account_currency

        # Format Period node.
        sorted_datetimes = sorted(datetimes)
        start_date = format_date_to_dmy(sorted_datetimes[0])
        end_date = format_date_to_dmy(sorted_datetimes[-1])
        ET.SubElement(root, "Period").text = f"[{start_date} - {end_date}]"

        # Generate random opening balance and add node for closing balance.
        opening_balance = task_context.opening_balance
        ET.SubElement(root, "Openingbalance").text = f"{opening_balance:,.2f}"
        closing_balance_node = ET.SubElement(root, "Closingbalance")

        operations = ET.SubElement(root, "Operations")
        for current_datetime in datetimes:
            operation = ET.SubElement(operations, "Operation")
            # Generate a random transaction header.
            ET.SubElement(operation, "n-n").text = "".join(
                self.random.choices(string.digits, k=9)
            )
            ET.SubElement(operation, "Number").text = "".join(
                self.random.choices(string.digits, k=10)
            )
            ET.SubElement(operation, "Date").text = format_date_to_dmy(current_datetime)
            ET.SubElement(operation, "Currency").text = task_context.account_currency

            # Generate transaction direction, amount, category, description.
            transaction = self._generate_transaction(task_context)

            # Set Income/Expense values based on direction.
            # FYI: amount is in account currency but description contains
            # information about a mount in origin currency.
            if transaction.is_income:
                ET.SubElement(operation, "Income").text = f"{transaction.account_amount:,.2f}"
                ET.SubElement(operation, "Expense").text = "0.00"
            else:
                ET.SubElement(operation, "Income").text = "0.00"
                ET.SubElement(operation, "Expense").text = f"{transaction.account_amount:,.2f}"

            # Generate receiver/payer details
            ET.SubElement(operation, "Receiver-PayerAccount").text = (
                transaction.payer_account if transaction.is_income else transaction.receiver_account
            )

            # Use first word as receiver/payer name
            receiver_payer = (
                transaction.description.split(" ")[0]
                if " " in transaction.description
                else transaction.description
            )
            ET.SubElement(operation, "Receiver-Payer").text = receiver_payer

            # Create a more detailed transaction description
            details = (
                f"{transaction.description}, Անկանխիկ գործարք, "
                f"{file_account_number[:4]}***{file_account_number[-3:]}, "
                f"{receiver_payer}({transaction.origin_currency}), "
                f"{current_datetime.strftime('%d/%m/%Y %H:%M:%S')}"
            )
            if transaction.origin_currency != task_context.account_currency:
                details += f" {transaction.origin_amount:,.2f} {transaction.origin_currency}"
            ET.SubElement(operation, "Details").text = details

        closing_balance_node.text = f"{task_context.current_balance * 1000:,.2f}"
        tree = ET.ElementTree(root)
        tree.write(file_path, encoding="utf-8", xml_declaration=True)
        return (
            "Generated Inecobank XML statement with "
            f"{task_context.transactions_count} transactions"
        )


class InecobankExcelGenerator(BaseGenerator):
    def __init__(self, context_manager: ContextManager):
        super().__init__("inecobankStatementXlsxFilesGlob", context_manager)

    def _generate(
        self,
        file_path: str,
        task_context: TaskContext,
        datetimes: list[datetime.datetime],
    ) -> str:
        file_account_number = task_context.account_number
        client_name = self.fake.name()
        # Create Excel workbook and worksheet
        wb = Workbook()
        ws = wb.active
        if not ws:
            raise ValueError("Failed to create worksheet")
        ws.title = "Statement"
        # Add header information with Armenian labels
        ws.append(["Հաշվի համար՝", file_account_number])
        ws.append(["Հաշվի արժույթ՝", task_context.account_currency])
        ws.append(["Հաճախորդ՝", client_name])
        ws.append([])  # Empty row
        # Add period information
        sorted_datetimes = sorted(datetimes)
        if sorted_datetimes:
            start_date = format_date_to_dmy(sorted_datetimes[0])
            end_date = format_date_to_dmy(sorted_datetimes[-1])
        else:
            start_date = end_date = format_date_to_dmy(task_context.task.end_date)
        ws.append(["Ժամանակահատված՝", f"{start_date} - {end_date}"])
        ws.append([])  # Empty row
        # Add account type headers (regular account format)
        ws.append([
            "Գործարքներ/այլ գործառնություններ",
            "Գործարքի գումար հաշվի արժույթով", 
            "Կիրառվող փոխարժեք",
            "Հաշվի վերջնական մնացորդ",
            "Գործարքի նկարագրություն"
        ])
        # Add transaction headers that parser looks for
        ws.append([
            "Ամսաթիվ",  # Date (column 0)
            "Գումար",   # Amount (column 1) 
            "",         # Empty (column 2)
            "Արժույթ",  # Currency (column 3)
            "",         # Empty (column 4)
            "Մուտք",    # Income (column 5)
            "Ելք"       # Expense (column 6)
        ])
        # Extend headers to match parser expectations
        # The parser expects: exchange rate at column 7, details at column 17
        current_row = [""] * 18  # Create row with 18 columns
        current_row[7] = "Փոխարժեք"  # Exchange rate
        current_row[17] = "Մանրամասներ"  # Details
        ws.append(current_row)
        # Generate and add transaction data
        transactions_count = 0
        for current_datetime in datetimes:
            transaction = self._generate_transaction(task_context)
            # Create transaction row with proper column placement
            trans_row = [""] * 18
            trans_row[0] = format_date_to_dmy(current_datetime)  # Date
            trans_row[1] = f"{transaction.account_amount:.2f}"  # Amount in account currency
            trans_row[3] = transaction.account_currency  # Currency
            if transaction.is_income:
                trans_row[5] = f"{transaction.account_amount:.2f}"  # Income
                trans_row[6] = "0.00"  # Expense
            else:
                trans_row[5] = "0.00"  # Income  
                trans_row[6] = f"{transaction.account_amount:.2f}"  # Expense
            trans_row[7] = "1.0000"  # Exchange rate (default to 1)
            # Amounts in account currency but description contains
            # information about amount in origin currency.
            details = transaction.description
            if transaction.origin_currency != task_context.account_currency:
                details += f" {transaction.origin_amount:,.2f} {transaction.origin_currency}"
            trans_row[17] = details  # Details
            ws.append(trans_row)
            transactions_count += 1
        # Add final balance information
        ws.append([])  # Empty row
        ws.append(["Մնացորդ՝", f"{task_context.current_balance:.2f}"])
        # Save the Excel file
        wb.save(file_path)
        return (
            "Generated Inecobank Excel statement with "
            f"{task_context.transactions_count} transactions"
        )


class AmeriaCsvGenerator(BaseGenerator):
    def __init__(self, context_manager: ContextManager):
        super().__init__("ameriaCsvFilesGlob", context_manager)

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
        task_context: TaskContext,
        datetimes: list[datetime.datetime],
    ) -> str:
        # Generate account number for the file
        file_account_number = task_context.account_number
        client_name = self.fake.name().upper()

        # Sort datetimes to determine period
        sorted_datetimes = sorted(datetimes)
        start_date = sorted_datetimes[0] if sorted_datetimes else task_context.task.end_date
        end_date = sorted_datetimes[-1] if sorted_datetimes else task_context.task.end_date

        # Prepare opening and closing balances.
        opening_balance = task_context.opening_balance
        closing_balance = opening_balance  # If we won't get transactions.
        account_currency = task_context.account_currency

        # Create CSV content as list of rows.
        csv_rows = [
            ["Start of Period", None, None, format_date_to_dmy(start_date)],
            ["End of Period", None, None, format_date_to_dmy(end_date)],
            ["Account No.", None, None, file_account_number, client_name],
            ["Currency", None, None, account_currency, f"{account_currency} currency"],
            ["TIN", None, None, str(self.random.randint(10000000, 99999999))],
            [],  # Empty row
        ]

        # Generate transactions data
        transactions = []
        total_debit = 0.0
        total_credit = 0.0

        for current_datetime in datetimes:
            # Generate transaction direction, amount, category, description
            transaction = self._generate_transaction(task_context)

            # Generate document number
            doc_no = "".join(self.random.choices(string.digits, k=6))

            # Generate transaction type
            trans_type = self._generate_transaction_type(transaction.description)

            # Set debit/credit amounts (in account currency).
            if transaction.is_income:
                debit_amount = 0.0
                credit_amount = transaction.account_amount
                total_credit += transaction.account_amount
                closing_balance += transaction.account_amount
            else:
                debit_amount = transaction.account_amount
                credit_amount = 0.0
                total_debit += transaction.account_amount
                closing_balance -= transaction.account_amount

            # Create transaction description with Armenian text
            desc = transaction.description
            if "Transfer between my accounts" in desc:
                if transaction.is_income:
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
                "Date": format_date_to_dmy(current_datetime),
                "Doc.No.": doc_no,
                "Type": trans_type,
                "Account": transaction.payer_account if transaction.is_income else transaction.receiver_account,
                "Details": desc,
                "Debit": self._format_money(debit_amount),
                "Credit": self._format_money(credit_amount),
                "Remitter/Beneficiary": ""  # Usually empty.
            })

        # Add balance information
        csv_rows.append([
            f"Opening Balance on {format_date_to_dmy(start_date)}", None, None,
            self._format_money(opening_balance)
        ])
        csv_rows.append([
            "Debit Turnover", None, None, self._format_money(total_debit)
        ])
        csv_rows.append([
            "Credit Turnover", None, None, self._format_money(total_credit)
        ])
        csv_rows.append([
            f"Closing Balance on {format_date_to_dmy(end_date)}", None, None,
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
            "Days Count", None, None, str(task_context.task.days_back)
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


class GenericCsvGenerator(BaseGenerator):
    def __init__(self, context_manager: ContextManager):
        super().__init__("genericCsvFilesGlob", context_manager)

    def _generate(
        self,
        file_path: str,
        task_context: TaskContext,
        datetimes: list[datetime.datetime],
    ) -> str:
        headers = [
            "Date",
            "FromAccount",
            "ToAccount",
            "IsExpense",
            "Amount",
            "Details",
            "AccountCurrency",
            "OriginCurrency", 
            "OriginCurrencyAmount",
        ]
        account_currency = task_context.account_currency
        # Generate transaction data
        transactions_data = []
        for current_datetime in datetimes:
            transaction = self._generate_transaction(task_context)
            # Create transaction row
            row = [
                current_datetime.strftime("%Y-%m-%d"),  # Date in YYYY-MM-DD
                transaction.payer_account,  # FromAccount
                transaction.receiver_account,  # ToAccount
                str(not transaction.is_income).lower(),  # IsExpense
                f"{transaction.account_amount:.2f}",  # Amount
                transaction.description,  # Details
                account_currency,  # AccountCurrency
                "",  # OriginCurrency (empty for same currency)
                "",  # OriginCurrencyAmount (empty for same currency)
            ]
            transactions_data.append(row)
        # Write CSV file
        with open(file_path, 'w', newline='', encoding='utf-8') as csvfile:
            writer = csv.writer(csvfile)
            writer.writerow(headers)
            writer.writerows(transactions_data)
        return (
            "Generated Generic CSV with "
            f"{len(transactions_data)} transactions"
        )


if __name__ == "__main__":
    config_path = "config-demo.yaml"
    config, target_folder = parse_config(config_path)
    print(f"Generating statements to match globs in '{config_path}'...")
    # Prepare tasks.
    now = datetime.datetime.now()
    tasks = [
        Task(
            generator_class=InecobankXmlGenerator,
            days_back=365,
            type=TaskType.SALARY,
            currency="AMD",
        ),
        # Task(
        #     generator_class=GenericCsvGenerator,
        #     end_date=now - datetime.timedelta(days=90),
        #     days_back=60,
        #     type=TaskType.BIGEVENTS,
        #     currency="AMD",
        # ),
        Task(
            generator_class=AmeriaCsvGenerator,
            days_back=365,
            type=TaskType.EVERYDAY,
            currency="AMD",
        ),
        Task(
            generator_class=InecobankXmlGenerator,
            days_back=365,
            type=TaskType.UTILITIES,
            currency="AMD",
        ),
    ]

    # Generate all contexts together to be linked on each other.
    context_manager = ContextManager(config)
    contexts = context_manager.generate_task_contexts(tasks)
    context_manager.execute_tasks(contexts, target_folder, remove_old=True)
