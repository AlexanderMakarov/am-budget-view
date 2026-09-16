# Inecobank

## Поддерживаемые форматы

### [FULL] XML (.xml) — рекомендуется

Скачивайте отдельную выписку по каждому счёту из [онлайн-банка Inecobank](https://online.inecobank.am/vcAccount/List): откройте счёт, выберите период и нажмите иконку скачивания в правом нижнем углу.

- Поддерживает все функции приложения и отчёты Beancount.
- Настройка в `config.yaml`: `inecobankStatementXmlFilesGlob`
- Парсер: `ineco_xml_parser.go`

### [NONE] Excel (.xls) с сайта

Скачивается с той же страницы, что и XML — **не поддерживается**. Используйте XML.

### [PARTIAL] Excel (.xlsx) из email

Inecobank присылает защищённые паролем `.xlsx` по email. В них нет номеров счетов получателя/отправителя. Сначала снимите защиту паролем ([MS Office](https://support.microsoft.com/en-us/office/change-or-remove-workbook-passwords-1c17af87-25e2-4dc6-94f0-19ce21ad0b68), [LibreOffice](https://ask.libreoffice.org/t/remove-file-password-protection/30982)).

- Настройка в `config.yaml`: `inecobankStatementXlsxFilesGlob`
- Парсер: `ineco_excel_parser.go`

## Ручное скачивание

1. Войдите на https://online.inecobank.am/vcAccount/List
2. Откройте счёт и выберите период
3. Скачайте XML-выписку
4. Сохраните её так, чтобы путь соответствовал `inecobankStatementXmlFilesGlob`

## Скачивание в приложении

Прямое скачивание для Inecobank недоступно. CLI ниже проверяет покрытие существующих XML и точно указывает, какие выписки ещё нужны.

## Список ручных скачиваний через CLI

Настройте шаблон XML, счета и даты в том же конфиге приложения, который использует Go-приложение:

```yaml
inecobankStatementXmlFilesGlob: "Statement *.xml"

bankDownloads:
  inecobank:
    sinceDate: "01-04-2024"
    # untilDate: "31-12-2024"  # Необязательно; по умолчанию сегодня.
    accounts:
      - number: "0000000000000001"  # Номера счетов указывайте в кавычках.
        name: "Текущий счёт AMD"
        type: account
      - number: "0000000000000002"
        name: "Карточный счёт AMD"
        type: card
        sinceDate: "01-06-2024"  # Необязательное переопределение для счёта.
        # untilDate: "31-12-2024"
```

Запустите проверку с обычным `config.yaml`:

```bash
python3 scripts/bank_downloader.py --manual-only
```

Другой конфиг приложения, например `tmp-my.yaml`, выбирается так:

```bash
python3 scripts/bank_downloader.py --manual-only --config tmp-my.yaml
```

Для каждого счёта скрипт читает `AccountNumber` и `Period` из подходящих XML, объединяет пересекающиеся и соседние периоды и показывает все недостающие диапазоны включительно. Проверяются и пропуски внутри истории. Пустая корректная выписка считается покрытием; имена файлов и даты транзакций покрытие не определяют.

Повреждённые XML, HTML-страницы ошибок и файлы без корректных метаданных показываются как предупреждения и не учитываются. Excel не учитывается. Предлагаемые пути не перезаписывают существующие файлы и по возможности соответствуют `inecobankStatementXmlFilesGlob`.

Скачайте перечисленные XML вручную и повторяйте команду, пока пропусков не останется. Команда только читает файлы и печатает инструкции; она не скачивает, не переименовывает и не перезаписывает выписки. Экспорт по сегодняшний день содержит только операции, доступные в момент скачивания.

Существующие настройки могут сохранить старую секцию `inecobank` в `scripts/bank_dowloader_config.yaml`; настройки из конфига приложения выше имеют приоритет. Параметр `--download-config path/to/downloads.yaml` по-прежнему доступен для этого совместимого формата и настроек CLI для MyAmeria/AmeriaBank.
