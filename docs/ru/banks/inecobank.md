# Inecobank

## Поддерживаемые форматы

### [FULL] XML (.xml) — рекомендуется

Скачивайте по каждому счёту с [онлайн-банка Inecobank](https://online.inecobank.am/vcAccount/List): выберите счёт, укажите период, нажмите иконку скачивания в правом нижнем углу.

- Поддерживает все функции приложения и отчёты Beancount.
- Настройка в `config.yaml`: `inecobankStatementXmlFilesGlob`
- Парсер: `ineco_xml_parser.go`

### [NONE] Excel (.xls) с сайта

Скачивается с той же страницы, что и XML — **не поддерживается**. Используйте XML.

### [PARTIAL] Excel (.xlsx) из email

Inecobank присылает защищённые паролем `.xlsx` по email. В них нет номеров счетов получателя/отправителя.

Для использования сначала снимите защиту паролем ([MS Office](https://support.microsoft.com/en-us/office/change-or-remove-workbook-passwords-1c17af87-25e2-4dc6-94f0-19ce21ad0b68), [LibreOffice](https://ask.libreoffice.org/t/remove-file-password-protection/30982)).

- Настройка в `config.yaml`: `inecobankStatementXlsxFilesGlob`
- Парсер: `ineco_excel_parser.go`

## Ручное скачивание

1. Войдите на https://online.inecobank.am/vcAccount/List
2. Откройте счёт и выберите период
3. Скачайте XML-выписку
4. Положите файл рядом с исполняемым файлом приложения (по шаблону `inecobankStatementXmlFilesGlob`)

## Скачивание в приложении

Недоступно для Inecobank.

## Скачивание через CLI

`make bank-downloader` выводит список недостающих XML-выписок по счетам из секции `inecobank` в `scripts/bank_dowloader_config.yaml`. Автоматизация интерфейса и cookie Inecobank не используются. Пример настроек есть в `scripts/bank_dowloader_config.yaml.template` и в [английской инструкции](../../en/banks/inecobank.md).

Укажите `folder_path` (относительно каталога `scripts/`), `statement_glob`, начальную дату `since-DD-MM-YYYY` и список `accounts` с номерами счетов в кавычках. Конечная дата по умолчанию — сегодня; `until-DD-MM-YYYY` позволяет задать её явно. Даты можно переопределить для отдельного счёта.

Скрипт читает `AccountNumber` и `Period` из существующих XML, объединяет периоды и показывает пропуски, включая пропуски внутри истории. Для каждого пропуска он печатает номер счёта, даты включительно и предлагаемое имя файла. Повреждённые файлы и файлы без корректных метаданных не учитываются. Excel не учитывается. Проверяется заявленный период, а не полнота транзакций; выписка за сегодня содержит только операции, доступные на момент скачивания.

Скачайте указанные XML вручную и повторите проверку без ввода данных других банков:

```bash
python3 scripts/bank_downloader.py --manual-only
```

Команда только читает файлы и печатает инструкции. Она не перезаписывает выписки. Убедитесь, что `inecobankStatementXmlFilesGlob` в настройках приложения соответствует сохранённым файлам.
