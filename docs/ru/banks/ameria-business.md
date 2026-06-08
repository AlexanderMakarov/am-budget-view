# AmeriaBank (Ameria for Business)

## Скачивание в приложении (рекомендуется)

Используйте страницу [Файлы](/files), чтобы скачивать CSV-выписки без ручного редактирования `config.yaml`.

### 1. Сохраните настройки без секретов

1. Откройте [Файлы](/files) → **Настройки скачивания**.
2. В блоке **AmeriaBank CSV statement** укажите:
   - **Дата начала** — с какой даты загружать транзакции (`ДД-ММ-ГГГГ`, например `01-01-2024`).
   - **Папка для файлов** — необязательная подпапка в рабочей директории приложения (пусто = рядом с `config.yaml`).
3. Нажмите **Сохранить настройки**. Эти значения сохраняются в `config.yaml`.

### 2. Вставьте свежий cookie сессии

1. Войдите на [business.myameria.am](https://business.myameria.am) (OTP из мобильного приложения: Меню → Настройки → OTP → Copy Code).
2. Откройте **DevTools** (F12) → вкладка **Network**.
3. Обновите страницу или откройте счёт. Найдите запрос к **gateway-businessmyameria.ameriabank.am**.
4. В **Request headers** скопируйте полное значение **Cookie**. Оно должно содержать `RefreshToken` (обычно также `AccessToken` и cookie `TS*`). Короткий cookie только с `TS*` даст ошибку HTTP 401.
5. На странице [Файлы](/files) нажмите **Скачать сейчас** в строке AmeriaBank и вставьте cookie в окно.
6. Нажмите **Скачать сейчас**. Новые CSV появятся в списке файлов после обновления.

**Важно:** cookie передаётся только для этого запроса скачивания. Он **не сохраняется** в `config.yaml` и на странице настроек. Копируйте новый cookie после выхода или истечения сессии.

## Поддерживаемые форматы

### [FULL] CSV (.csv) — рекомендуется

Скачивайте по каждому счёту из [интернет-банка Ameria](https://online.ameriabank.am/InternetBank/MainForm.wgx): счёт → Выписка, выберите период (для произвольного — поля FromDate и To), включите **«Show equivalent in AMD»** (для курсов), нажмите **Export to CSV** (иконка справа вверху).

- Поддерживает все функции приложения и отчёты Beancount.
- Настройка в `config.yaml`: `ameriaCsvFilesGlob`
- Парсер: `ameria_csv_parser.go`
- Также поддерживаются CSV с https://business.myameria.am (новый REST API сайт).

### [NONE] XML (.xml) с сайта

Скачивается с того же места, что CSV — **не поддерживается** (нет номера счёта и валюты).

### [NONE] XLSX (.xlsx) из email

Присылается AmeriaBank по email — **не поддерживается** (нет номеров счетов и курсов).

## Ручное скачивание с сайта

1. Войдите на https://online.ameriabank.am/InternetBank/MainForm.wgx (или https://business.myameria.am)
2. Счёт → Выписка, укажите период и **Show equivalent in AMD**
3. Экспортируйте в CSV
4. Положите файлы рядом с приложением (по шаблону `ameriaCsvFilesGlob`)

## Как скопировать заголовок Cookie

1. Откройте [business.myameria.am](https://business.myameria.am) и войдите.
2. Нажмите **F12** → **Network**.
3. Вызовите любой API-запрос (обновление страницы, открытие счетов, выписки).
4. Выберите запрос с URL `gateway-businessmyameria.ameriabank.am`.
5. В **Request headers** → **Cookie** скопируйте всё значение (часто 2000+ символов).
6. Вставьте в окно **Скачать сейчас** на странице Файлы.

При ошибке **401** cookie неполный или устарел — войдите снова и скопируйте Cookie с `RefreshToken`.

## Скачивание через CLI (`make bank-downloader`) — для продвинутых

Полуавтоматическое скачивание через [bank_downloader.py](/scripts/bank_downloader.py):

1. Войдите на https://business.myameria.am (OTP из мобильного приложения).
2. Скопируйте заголовок **Cookie** из DevTools (те же шаги, что выше).
3. В `scripts/bank_dowloader_config.yaml` укажите `ameriabank.cookie` и `ameriabank.since-DD-MM-YYYY`. При необходимости — `folder_path` и `accounts`.
4. Запустите `make bank-downloader`.

Cookie сессии истекает — копируйте заново после повторного входа. Хелперы: [scripts/bank_helpers_ameria.py](/scripts/bank_helpers_ameria.py).

Учётные данные в CLI-конфиге хранятся локально — не передавайте и не коммитьте их.
