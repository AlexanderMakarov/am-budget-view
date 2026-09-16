# i18n test fixtures

These locale files are deliberately small synthetic fixtures for `i18n_test.go`.
The application embeds the complete product translations from `/locales`; tests
keep independent inputs so they can exercise loading and formatting without
silently changing when product copy is edited.
