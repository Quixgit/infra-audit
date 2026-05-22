# infra-audit — инструкция по использованию

## Предварительные требования

Инструменты должны быть установлены на сервере:
- `gitleaks` — поиск секретов
- `semgrep` — статический анализ кода
- `trivy` — анализ Terraform
- `npm` — аудит зависимостей Node.js

Проверка:
```bash
which gitleaks semgrep trivy npm
```

---

## 1. Отчёт по инфраструктуре DigitalOcean

Сканирует живую инфраструктуру через DO API.

```bash
cd ~/infra-audit && go run ./cmd/infra-audit all-do \
  --client "Название клиента" \
  --project "Название проекта" \
  --do-project-id "UUID проекта в DO" \
  --scope-mode hybrid \
  --spaces-buckets "bucket-name:region:sensitivity" \
  --prepared-by "Имя аудитора / InfraJump Security Team" \
  --auditor-org "InfraJump, Inc." \
  --auditor-address $'8 the Grn Ste A\nDover, DE 19901' \
  --auditor-email "delivery@infrajump.com" \
  --auditor-website "infrajump.com" \
  --auditor-phone "+1 650 4847938" \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png \
  --watermark assets/watermark.png \
  --footer-bg assets/footer-bg.png \
  --out out/client-name
```

**Параметры:**

| Флаг | Описание |
|------|----------|
| `--do-project-id` | UUID проекта в DigitalOcean (Dashboard → Projects) |
| `--scope-mode` | `project` — только проект, `hybrid` — проект + связанные ресурсы, `account` — весь аккаунт |
| `--spaces-buckets` | Список бакетов: `name:region:sensitivity` через запятую. Sensitivity: `public`/`sensitive`/`internal` |

**Результат:**
```
out/client-name/
  evidence/do_inventory.json       # собранные данные
  report/
    client_project.html            # HTML отчёт
    client_project.docx            # DOCX отчёт
    client_project_findings.json   # findings JSON
```

**Требуется токен DO:**
```bash
export DO_TOKEN="dop_v1_..."
```

---

## 2. Отчёт по коду и Terraform

Сканирует локально склонированный репозиторий клиента.

```bash
# Сначала клонируем репу
git clone git@github.com:client/repo.git ~/work/client/repo

# Запускаем сканер
cd ~/infra-audit && go run ./cmd/code-audit \
  --repo /path/to/client/repo \
  --client "Название клиента" \
  --project "Название проекта Code Security Audit" \
  --prepared-by "Имя аудитора / InfraJump Security Team" \
  --auditor-org "InfraJump, Inc." \
  --auditor-address $'8 the Grn Ste A\nDover, DE 19901' \
  --auditor-email "delivery@infrajump.com" \
  --auditor-website "infrajump.com" \
  --auditor-phone "+1 650 4847938" \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png \
  --watermark assets/watermark.png \
  --footer-bg assets/footer-bg.png \
  --out out/client-name-code
```

**Что сканируется автоматически:**

| Стек | Инструменты |
|------|-------------|
| Node.js / TypeScript | semgrep, npm audit |
| Docker | semgrep (dockerfile rules) |
| GitHub Actions | semgrep (CI/CD rules) |
| Terraform | trivy config + hclscan (кастомные DO-правила) |
| Все файлы | gitleaks (секреты и credentials) |

**Результат:**
```
out/client-name-code/
  report/
    client_project.html   # HTML отчёт
    client_project.docx   # DOCX отчёт
```

---

## 3. Полный аудит — оба отчёта

Запускаем последовательно для одного клиента:

```bash
export DO_TOKEN="dop_v1_..."
CLIENT="Work Order Platform"
PROJECT_ID="be6bf36b-e07e-48ed-90e1-2604b1455e52"
REPO="/home/quix/work/exclusvierentals/work-order"
OUT="out/work-order"

cd ~/infra-audit

# 1. Инфраструктура DO
go run ./cmd/infra-audit all-do \
  --client "$CLIENT" \
  --project "$CLIENT Infrastructure Audit" \
  --do-project-id "$PROJECT_ID" \
  --scope-mode hybrid \
  --prepared-by "Oleg / InfraJump Security Team" \
  --auditor-org "InfraJump, Inc." \
  --auditor-address $'8 the Grn Ste A\nDover, DE 19901' \
  --auditor-email "delivery@infrajump.com" \
  --auditor-website "infrajump.com" \
  --auditor-phone "+1 650 4847938" \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png \
  --watermark assets/watermark.png \
  --footer-bg assets/footer-bg.png \
  --out "$OUT/infra"

# 2. Код и Terraform
go run ./cmd/code-audit \
  --repo "$REPO" \
  --client "$CLIENT" \
  --project "$CLIENT Code Security Audit" \
  --prepared-by "Oleg / InfraJump Security Team" \
  --auditor-org "InfraJump, Inc." \
  --auditor-address $'8 the Grn Ste A\nDover, DE 19901' \
  --auditor-email "delivery@infrajump.com" \
  --auditor-website "infrajump.com" \
  --auditor-phone "+1 650 4847938" \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png \
  --watermark assets/watermark.png \
  --footer-bg assets/footer-bg.png \
  --out "$OUT/code"

echo "Reports ready:"
ls "$OUT/infra/report/"
ls "$OUT/code/report/"
```

---

## Готовые отчёты

После запуска передаёшь клиенту два файла:

| Файл | Содержание |
|------|------------|
| `*_infrastructure_audit.docx` | Инфраструктура DO: firewall, databases, apps, secrets |
| `*_code_security_audit.docx` | Код: secrets в репе, Dockerfile, GitHub Actions, Terraform |

HTML версии — для внутреннего просмотра и печати в PDF через браузер.
