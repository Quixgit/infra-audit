# CloudSecGuard — CLAUDE.md

## Что это за проект

**CloudSecGuard** (внутреннее название кодовой базы — InfraJump / infra-audit) — это самохостируемая SaaS-платформа для аудита безопасности инфраструктуры.

Два режима работы:
- **Web-платформа** — браузерный дашборд: подключения, аудиты, находки, отчёты, командная работа
- **CLI-движок** — безголовой сканер: полный аудит из командной строки → HTML/DOCX-отчёты

---

## Технологический стек

| Слой | Технология |
|------|-----------|
| Frontend | React 18, TypeScript, Vite, TailwindCSS, shadcn/ui, React Router, TanStack Query, Axios, Recharts |
| Backend | Go 1.24, chi router, pgx/v5 |
| База данных | PostgreSQL 16 |
| Auth | JWT (access + refresh tokens), опциональный Google OAuth, TOTP MFA |
| AI | Groq API (llama-3.1-8b-instant) — подсказки по ремедиации |
| Deploy | Docker Compose (3 контейнера: nginx, backend, postgres) |
| Отчёты | HTML + DOCX (кастомный генератор на Go) |
| Сканеры CLI | gitleaks, semgrep, trivy, npm audit, hclscan |

---

## Структура репозитория

```
infra-audit/
├── cmd/
│   ├── infra-audit/      # CLI: сканирование DigitalOcean инфраструктуры
│   ├── code-audit/       # CLI: сканирование кода и Terraform
│   ├── html2docx/        # CLI: конвертация HTML → DOCX
│   └── keygen/           # Генератор лицензионных ключей (внутренний)
├── internal/
│   ├── scanner/          # DO API сканер, Spaces сканер
│   ├── scanner/code/     # semgrep, gitleaks, trivy, npm, hclscan обёртки
│   ├── scanner/aws/      # AWS сканер (EC2, S3, IAM, RDS)
│   ├── rules/            # Реестр кастомных правил безопасности
│   ├── report/           # Генераторы HTML и DOCX отчётов
│   ├── model/            # Общие модели данных
│   └── doapi/            # DigitalOcean API клиент
├── web/
│   ├── backend/          # Go HTTP сервер — все .go файлы в package main
│   ├── frontend/src/     # React приложение
│   └── docker-compose.yml
├── assets/               # Дефолтные logo.png, watermark.png, footer-bg.png
├── keys/private.pem      # Приватный RSA ключ для генерации лицензий
└── web/backend/keys/public.pem  # Публичный ключ (embed в бинарник)
```

---

## Архитектура backend (Go)

Все файлы в `web/backend/` — один пакет `main`. Точка входа — `main.go`.

### Ключевые файлы backend

| Файл | Назначение |
|------|-----------|
| `main.go` | Chi роутер, middleware, регистрация всех маршрутов, запуск background-горутин |
| `models.go` | Все Go-типы: User, Connection, AuditJob, Schedule, LicenseInfo и т.д. |
| `db.go` | DB-пул, миграции (schemaMigrations), все SQL-запросы |
| `auth.go` | Login/register/logout, JWT генерация/верификация, refresh |
| `handlers.go` | HTTP-хендлеры: me, connections, jobs, share, tokens, team, modules |
| `audit.go` | `runAudit()` — DO сканирование, запись находок, генерация отчётов |
| `code_audit.go` | `runCodeAudit()` — клонирование репо, запуск gitleaks/semgrep/trivy/npm |
| `aws_audit.go` | `runAWSAudit()` — AWS сканирование через SDK v2, генерация отчётов |
| `ssl_audit.go` | `runSSLAudit()` — проверка TLS-сертификатов |
| `dns_audit.go` | `runDNSAudit()` — проверка DNS записей |
| `license.go` | RSA JWT лицензии, кэш, feature gates |
| `ai_suggest.go` | AI-подсказки через Groq API (llama-3.1-8b-instant) |
| `ws.go` | WebSocket hub для live-прогресса аудитов |
| `scheduler.go` | Background-горутина для запуска по расписанию |
| `monitoring.go` | Security scores, SLA breaches, тренды |
| `compliance.go` | SOC2/ISO27001/NIST/CIS frameworks, маппинг находок |
| `remediation.go` | Канбан задачи ремедиации, комментарии, verify-fix |
| `policies.go` | Политики безопасности, жизненный цикл, шаблоны |
| `evidence.go` | Хранение артефактов для compliance |
| `access_reviews.go` | Периодические ревью доступов |
| `auditor.go` | Внешний портал аудитора (token-based, без авторизации) |
| `github_webhook.go` | GitHub webhook — автозапуск аудита при PR |
| `encrypt.go` | AES-GCM шифрование токенов в БД |
| `email.go` | SMTP уведомления о завершении аудита |
| `weekly_digest.go` | Еженедельный email-дайджест |
| `tenant_auth.go` | Мультитенантность — привязка пользователей к tenants |
| `custom_frameworks.go` | Кастомные compliance-фреймворки |
| `findings.go` | Агрегация находок, overrides статусов |
| `util.go` | Вспомогательные функции: writeJSON, writeError, envOr |

### Middleware и контекст

```
authMiddleware → ctxUserID, ctxTenantID, ctxUserRole в context
adminOnly      → только admin/owner роли
```

### Background-горутины (запускаются в main)

- `startScheduler` — каждую минуту проверяет расписания, запускает аудиты
- `startSLAChecker` — каждый час проверяет SLA нарушения
- `startWeeklyDigest` — каждое воскресенье отправляет дайджест

---

## Архитектура frontend (React/TypeScript)

### Ключевые файлы и директории

| Путь | Назначение |
|------|-----------|
| `src/App.tsx` | BrowserRouter, маршруты, RequireAuth HOC, ErrorBoundary |
| `src/lib/api.ts` | **Все API-вызовы** — axios с interceptors (auth + refresh), типы для всех сущностей |
| `src/store/useAuthStore.ts` | Zustand стор: accessToken, refreshToken, user |
| `src/store/useThemeStore.ts` | Zustand стор: тема (dark/light) |
| `src/components/AppLayout.tsx` | Главный layout с Sidebar |
| `src/components/Sidebar.tsx` | Навигация, module toggles |
| `src/pages/Dashboard.tsx` | Главный дашборд, auto-refresh 30s |
| `src/pages/CloudAudits.tsx` | DigitalOcean + AWS подключения, bulk run |
| `src/pages/CodeIaC.tsx` | Code/IaC подключения |
| `src/pages/ConnectionForm.tsx` | Форма создания/редактирования подключений |
| `src/pages/JobDetail.tsx` | Live-прогресс через WebSocket, скачивание отчётов |
| `src/pages/Findings.tsx` | Агрегированный список находок, фильтры |
| `src/pages/Remediation.tsx` | Канбан-доска задач |
| `src/pages/Compliance.tsx` | Compliance frameworks |
| `src/pages/Monitoring.tsx` | Security scores, SLA |
| `src/pages/Settings.tsx` | 7 вкладок: Profile, Security, Branding, Team, External, Modules, Activity |
| `src/pages/Plans.tsx` | Тарифные планы, активация лицензии |
| `src/pages/AuditorPortal.tsx` | Внешний портал (без авторизации) |
| `src/pages/ShareView.tsx` | Публичная ссылка на отчёт |

### Паттерн работы с API

Все API-функции сгруппированы в `src/lib/api.ts` по модулям:
`authApi`, `connectionsApi`, `jobsApi`, `findingsApi`, `dashboardApi`, `schedulesApi`,
`teamApi`, `shareApi`, `licenseApi`, `monitoringApi`, `slaApi`, `policiesApi`,
`evidenceApi`, `accessReviewsApi`, `remediationApi`, `auditorPortalApi`, `customFrameworksApi` и др.

Axios interceptor автоматически:
1. Добавляет `Authorization: Bearer <token>` из localStorage
2. При 401 делает refresh и повторяет запрос

TanStack Query используется для кэширования и invalidation в страницах.

---

## База данных (PostgreSQL)

### Ключевые таблицы

| Таблица | Описание |
|---------|---------|
| `users` | Пользователи (email, bcrypt hash, auditor_org, mfa_secret) |
| `tenants` | Рабочие пространства (workspace) |
| `tenant_members` | Many-to-many users↔tenants с ролями (owner/admin/viewer) |
| `connections` | Подключения (do/code/ssl/dns/aws) — токены шифруются AES-GCM |
| `audit_jobs` | Задания аудита (queued/running/done/failed) |
| `refresh_tokens` | JWT refresh tokens (hashed) |
| `share_links` | Публичные ссылки на отчёты |
| `api_tokens` | Программные API токены (SHA256 hash) |
| `schedules` | Расписания автоаудитов |
| `finding_overrides` | Статусы находок (open/in_progress/fixed/accepted_risk/false_positive) |
| `evidence_items` | Compliance-артефакты |
| `evidence_mappings` | Привязка артефактов к контролям |
| `remediation_tasks` | Задачи ремедиации (канбан) |
| `remediation_comments` | Комментарии к задачам |
| `policies` | Политики безопасности |
| `policy_control_mappings` | Привязка политик к контролям |
| `sla_rules` | SLA правила (max_days_open по severity) |
| `finding_sla_breaches` | Нарушения SLA |
| `security_scores` | Оценки безопасности по подключениям |
| `finding_changes` | История изменений находок |
| `access_reviews` | Ревью доступов |
| `access_review_items` | Записи ревью (user + decision) |
| `auditor_invites` | Инвайты на внешний аудиторский портал |
| `auditor_comments` | Комментарии внешних аудиторов |
| `activity_log` | Лог действий (для compliance audit trail) |
| `custom_frameworks` | Кастомные compliance фреймворки |
| `custom_controls` | Контроли кастомных фреймворков |
| `github_webhook_events` | События GitHub webhook |
| `digest_log` | Трекинг еженедельных дайджестов |
| `notify_requests` | Запросы "уведоми меня" для будущих фич |
| `settings` | Key-value настройки (license_key, admin_preview_plan, module_*) |

**Миграции** применяются при запуске через `schemaMigrations` в `db.go` (идемпотентные ALTER TABLE IF NOT EXISTS / CREATE TABLE IF NOT EXISTS).

---

## Система лицензий

- Лицензионные ключи — JWT-токены, подписанные RSA приватным ключом (`keys/private.pem`)
- Публичный ключ embed в бинарник (`//go:embed keys/public.pem`)
- Планы: `community` (free) → `starter` → `professional` → `business` → `enterprise`
- Feature gates через `hasFeature(claims, "feature_name")`
- Кэш лицензии в памяти с TTL 1 час
- Admin preview plan — позволяет тестировать фичи без реальной лицензии

### Лимиты планов

| План | Подключения | Аудиты/мес | Пользователи | Ключевые фичи |
|------|-------------|------------|--------------|---------------|
| Community | 5 | 20 | 1 | Базовые аудиты, share links |
| Starter | 10 | 30 | 2 | + Расписание |
| Professional | 30 | 100 | 5 | + Code/AWS аудиты, кастомный брендинг |
| Business | 9999 | 9999 | 15 | + API токены, управление командой |
| Enterprise | 9999 | 9999 | 9999 | + Все фичи |

---

## Типы подключений и сканеры

| Тип | conn_type | Что сканирует | Отчёт |
|-----|-----------|---------------|-------|
| DigitalOcean | `do` | Droplets, firewalls, databases, App Platform, Spaces, VPCs, LBs | HTML + DOCX |
| AWS | `aws` | EC2 security groups, S3 ACLs, IAM, RDS | HTML + DOCX |
| Code/IaC | `code` | gitleaks (secrets), semgrep (SAST), trivy (Terraform), npm audit | HTML + DOCX |
| SSL/TLS | `ssl` | Сертификаты, TLS версии, шифры, HSTS, OCSP | Находки в платформе |
| DNS | `dns` | SPF, DKIM, DMARC, DNSSEC, CAA, zone transfer | Находки в платформе |

---

## Жизненный цикл аудита

```
Run Audit → job создаётся (status: pending)
         → горутина: runAudit/runCodeAudit/runAWSAudit/runSSLAudit/runDNSAudit
         → updateJobProgress() → WebSocket broadcast → live UI update
         → updateJobDone() → HTML+DOCX сохраняются в DATA_DIR/reports/{job_id}/
         → security score пересчитывается
         → SLA checker отслеживает находки
         → email уведомление (если notify_email=true)
```

Находки хранятся как файлы в папке отчёта:
- `findings.json` — DO/AWS/SSL/DNS находки
- `tf_findings.json` — Terraform находки
- `report.html`, `report.docx`

---

## WebSocket

- Эндпоинт: `GET /api/ws/jobs/{id}` (auth через query param `?token=...`)
- Hub в памяти: `broadcast(jobID, wsMessage)` → все подписанные клиенты
- Клиент (`JobDetail.tsx`) подключается при открытии страницы задания

---

## Шифрование токенов

Все секреты в БД (DO token, repo_token, AWS keys, GitHub webhook secret) шифруются AES-GCM.
Ключ: `ENCRYPTION_KEY` env var или случайный при первом запуске.

---

## Переменные окружения

| Переменная | Обязательная | Описание |
|------------|-------------|---------|
| `DATABASE_URL` | ✅ | PostgreSQL DSN |
| `JWT_SECRET` | | Секрет для подписи JWT (авто-генерируется если пусто) |
| `ENCRYPTION_KEY` | | AES ключ для шифрования токенов |
| `ASSETS_DIR` | | Путь к assets (logo, watermark, footer-bg). Default: `/app/assets` |
| `DATA_DIR` | | Хранилище отчётов и user-assets. Default: `/app/data` |
| `PORT` | | Порт backend. Default: `8080` |
| `CORS_ORIGINS` | | Разрешённые origins. Default: `http://localhost:3000` |
| `GROQ_API_KEY` | | Groq API ключ для AI-подсказок по ремедиации |
| `GOOGLE_CLIENT_ID` | | Google OAuth (опционально) |
| `GOOGLE_CLIENT_SECRET` | | Google OAuth (опционально) |
| `GOOGLE_REDIRECT_URL` | | Google OAuth callback URL |
| `FRONTEND_URL` | | URL фронтенда (для email ссылок) |
| `APP_URL` | | Публичный URL приложения |
| `ADMIN_EMAIL` | | Email первого admin (default: `admin@localhost`) |
| `SMTP_HOST/PORT/USER/PASS/FROM` | | SMTP для email уведомлений (опционально) |

---

## Запуск для разработки

### Docker (рекомендуется)
```bash
cd web
cp .env.example .env
docker compose up -d --build
# UI: http://localhost:3000  Backend: http://localhost:8080
docker compose logs backend | grep -i "admin\|password"
```

### Dev mode (hot reload)
```bash
# 1. PostgreSQL
docker run -d --name csg-pg \
  -e POSTGRES_DB=infra_audit -e POSTGRES_USER=audit -e POSTGRES_PASSWORD=changeme \
  -p 5432:5432 postgres:16-alpine

# 2. Backend
DATABASE_URL="postgres://audit:changeme@localhost:5432/infra_audit?sslmode=disable" \
ASSETS_DIR="$(pwd)/assets" DATA_DIR="$(pwd)/web/data" \
go run ./web/backend/

# 3. Frontend (отдельный терминал)
cd web/frontend && npm install && npm run dev
# http://localhost:5173 — Vite проксирует /api → localhost:8080
```

---

## Git-конвенции

- **Ветка разработки:** `dev`
- **Продакшн:** `main`
- **Remote:** `git@github-quixgit:Quixgit/CloudSecGuard.git`
- Коммит-сообщения: `type: описание` (feat, fix, refactor, docs, chore, ui, security)
- **Никаких Co-Authored-By в коммитах**
- После завершения фичи: merge `dev` → `main`, push обеих веток

---

## Роадмап (приоритеты)

### В работе / ближайшие задачи
- AWS сканер: расширение правил (CloudTrail, VPC Flow Logs, Config)
- GCP/Azure/K8s сканеры (в stub-режиме, `scanner_gcp`, `scanner_azure`, `scanner_k8s`)
- Slack уведомления (webhook настроен в workspace settings)
- Jira/Linear интеграция из remediation tasks
- Auto-import findings → remediation tasks (связка)

### Планируется
- SAML 2.0 / OIDC SSO (`sso` feature flag уже есть в планах)
- Micro-frontend модульная федерация
- Container image scanning через trivy
- SBOM генерация (CycloneDX, SPDX)
- Jira/PagerDuty webhook-интеграции
- PCI-DSS, HIPAA compliance frameworks

---

## Compliance фреймворки (встроенные)

| Фреймворк | Slug |
|-----------|------|
| SOC 2 | `soc2` |
| ISO 27001 | `iso27001` |
| NIST CSF | `nist_csf` |
| CIS Controls v8 | `cis_v8` |

Плюс кастомные фреймворки (`custom_frameworks` table).

---

## Важные паттерны кода

### Backend
- Все хендлеры — методы `(srv *server)`, никаких глобальных переменных
- Tenant isolation: каждый запрос проверяет `tenant_id` из контекста
- Секреты никогда не возвращаются в API ответах (`connToResponse()`)
- License enforcement: `hasFeature(srv.getEffectiveClaims(...), "feature")`
- Activity logging: `srv.logActivity(...)` при любом важном действии
- Job execution: всегда в горутинах с `recover()` для предотвращения паник

### Frontend
- Все API вызовы через `src/lib/api.ts` — никаких прямых axios вызовов в компонентах
- TanStack Query для всех данных с сервера
- `useAuthStore` (Zustand) — единственный источник истины для auth state
- Tokens в `localStorage` (`access_token`, `refresh_token`)
- Feature gates в UI: проверяем `license.features.includes("feature_name")`

---

## Известные ограничения / нюансы

- Файлы отчётов хранятся локально на диске (`DATA_DIR`) — при горизонтальном масштабировании нужен shared storage
- WebSocket hub в памяти — при нескольких инстансах backend нужен Redis Pub/Sub
- `schemaMigrations` применяются при каждом запуске — они идемпотентны, но не rollback
- `getUserByRefreshToken` не сканирует `mfa_enabled` — нужно починить при расширении auth
- Admin preview plan работает только для admin/owner ролей
- GitHub webhook secret не обновляется через edit connection UI (только при создании)
