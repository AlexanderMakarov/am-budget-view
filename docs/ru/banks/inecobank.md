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

Недоступно для Inecobank.
