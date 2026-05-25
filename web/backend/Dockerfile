FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod ./
RUN go mod download || true

COPY . .
RUN go mod tidy && go build -o /backend ./web/backend/

# ── Tool downloader stage ──────────────────────────────────────────────────────
FROM alpine:3.19 AS tools
RUN apk add --no-cache curl tar

# gitleaks
ARG GITLEAKS_VERSION=8.30.1
RUN curl -sSL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
    | tar -xz -C /usr/local/bin gitleaks && chmod +x /usr/local/bin/gitleaks

# trivy
ARG TRIVY_VERSION=0.70.0
RUN curl -sSL "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" \
    | tar -xz -C /usr/local/bin trivy && chmod +x /usr/local/bin/trivy

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata git openssh-client

COPY --from=builder /backend /backend
COPY --from=tools /usr/local/bin/gitleaks /usr/local/bin/gitleaks
COPY --from=tools /usr/local/bin/trivy /usr/local/bin/trivy

# Disable git safe.directory restrictions for cloned repos
RUN git config --global --add safe.directory '*'

EXPOSE 8080

ENTRYPOINT ["/backend"]
