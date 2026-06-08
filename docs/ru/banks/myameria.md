# MyAmeria (Ameria для физлиц)

## Скачивание в приложении (рекомендуется)

Используйте страницу [Файлы](/files), чтобы скачать Generic CSV через API банка без ручного редактирования `config.yaml`.

### 1. Сохраните настройки без секретов

1. Откройте [Файлы](/files) → **Настройки скачивания**.
2. В блоке **MyAmeria generic CSV** укажите:
   - **Client ID** — заголовок `Client-Id` из DevTools браузера (см. ниже).
   - **Дата начала** — с какой даты загружать транзакции (`ДД-ММ-ГГГГ`, например `01-01-2024`).
3. Нажмите **Сохранить настройки**. Эти значения сохраняются в `config.yaml`.

Также настройте `myAmeriaMyAccounts` в `config.yaml` (номер счёта → валюта). Экспорт API не содержит валюту по счетам.

### 2. Вставьте свежий auth token

1. Войдите на [account.myameria.am](https://account.myameria.am).
2. Откройте **DevTools** (F12) → вкладка **Network**.
3. Обновите страницу или откройте историю операций. Найдите запрос к **ob.myameria.am** (или другому API-хосту MyAmeria).
4. В **Request headers** скопируйте полное значение **Authorization** (начинается с `Bearer `). Токен истекает примерно через **15 минут**.
5. На странице [Файлы](/files) нажмите **Скачать сейчас** в строке MyAmeria и вставьте токен в окно.
6. Нажмите **Скачать сейчас**. Новый CSV появится в списке файлов после обновления.

**Важно:** auth token передаётся только для этого запроса. Он **не сохраняется** в `config.yaml` и на странице настроек. Вставляйте новый токен при каждом истечении.

Скачивание в приложении создаёт файл **Generic CSV** (не Excel History MyAmeria).

## Поддерживаемые форматы

### [FULL] History Excel (.xls) — рекомендуется (2025+)

Скачивайте с https://myameria.am/history: нажмите **Filter** (справа), укажите даты, затем **Excel** в разделе Actions.

- Один файл содержит все счета и карты.
- Настройка в `config.yaml`: `myAmeriaHistoryXlsFilesGlob`
- Требуется словарь `myAmeriaMyAccounts` (номер счёта → валюта) — файл не содержит эти данные.
- Поддерживает функции приложения и Beancount, кроме курсов (их нет в файле).
- Парсер: `ameria_history_parser.go`

### [УСТАРЕЛО] Account Statements Excel (.xls)

Скачивались со страниц вроде https://myameria.am/cards-and-accounts/account-statement/****** до 2025. Не работали для карт, только для счетов. Оставлено для совместимости со старыми файлами.

- Настройка в `config.yaml`: `myAmeriaAccountStatementXlsxFilesGlob`
- Парсер: `ameria_stmt_parser.go`
- С 2025 используйте History Excel.

### [NONE] CSV выписок по счёту/карте (2025+)

Скачиваются со страниц выписок в 2025+ — **не поддерживаются** (нет номеров счетов и сумм в валюте счёта).

## Ручное скачивание с сайта

1. Войдите на https://myameria.am/history
2. Отфильтруйте по датам и экспортируйте в Excel
3. Настройте `myAmeriaMyAccounts` в `config.yaml`
4. Положите файл рядом с приложением (по шаблону `myAmeriaHistoryXlsFilesGlob`)

## Как скопировать Client-Id и Authorization

1. Откройте [account.myameria.am](https://account.myameria.am) и войдите.
2. Нажмите **F12** → **Network**.
3. Обновите страницу или откройте **History**, чтобы появились API-запросы.
4. Выберите запрос к **ob.myameria.am** (или другому `*.myameria.am` API).
5. В **Request headers**:
   - **Client-Id** — скопируйте в **Настройки скачивания** на странице Файлы (сохраняется в config).
   - **Authorization** — скопируйте полное значение (`Bearer eyJ…`) в окно **Скачать сейчас** (не сохраняется).

![Как скопировать заголовок Authorization из DevTools](/static/docs/devtools-auth-header.png)

## Скачивание через CLI (`make bank-downloader`) — для продвинутых

Полуавтоматическое скачивание через [bank_downloader.py](/scripts/bank_downloader.py):

1. Скопируйте [scripts/bank_dowloader_config.yaml.template](/scripts/bank_dowloader_config.yaml.template) в `scripts/bank_dowloader_config.yaml`.
2. Войдите на https://account.myameria.am, откройте DevTools → Network.
3. Скопируйте **Client-Id** → `client_id`, **Authorization** → `auth_token` в YAML.
4. Укажите `since-DD-MM-YYYY` и при необходимости `history_path`.
5. Запустите `make bank-downloader`.

**Заметки:**

1. Скрипт использует API банка и создаёт **Generic CSV**, а не Excel History MyAmeria.
2. Обновляйте `auth_token` при каждом истечении (~15 минут).

Учётные данные в CLI-конфиге хранятся локально — не передавайте и не коммитьте их.
