# Cuan

> The self-hosted agentic workspace and operating system for creators, solopreneurs, and small businesses. Run autonomous AI agents on your own server with zero token markups, zero vendor lock-in, and built-in team collaboration.

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/buchin/cuan?color=green)](https://github.com/buchin/cuan/releases)
[![Docs](https://img.shields.io/badge/docs-cuan.work-blue)](https://cuan.work/docs)

---

## ⚡ 30-Second Quick Start

The fastest way to get Cuan running on your server or local machine:

### Option 1: Docker (Recommended)

```bash
curl -fsSL https://cuan.work/docker-compose.yml -o docker-compose.yml
docker compose up -d
```
Visit `http://localhost:8080` to complete the initial setup wizard.

### Option 2: 1-Line Native Linux Install

```bash
curl -fsSL https://cuan.work/install.sh | bash
```

### Option 3: Build from Source

```bash
git clone https://github.com/buchin/cuan.git
cd cuan
go run ./cmd/server
```

---

## 📖 Documentation

All guides, architecture references, and API documentation are maintained at **[cuan.work/docs](https://cuan.work/docs)**:

- [🚀 Getting Started Guide](https://cuan.work/docs/getting-started)
- [🤖 Connecting AI Providers (Claude, OpenAI, DeepSeek, AGY)](https://cuan.work/docs/ai-providers)
- [👥 Multi-User Team & Staff Onboarding (RBAC)](https://cuan.work/docs/team)
- [💬 Multi-Channel Inbox & WhatsApp Setup](https://cuan.work/docs/inbox)
- [🛠️ Self-Hosting, Backup & Disaster Recovery](https://cuan.work/docs/self-hosting)

---

## 💡 Why Cuan?

- **Zero Token Markup:** Bring your own API keys or official CLI subscriptions (`claude`, `codex`, `agy`) directly at cost.
- **Self-Contained & Private:** Standalone Go binary with SQLite/PostgreSQL. Your credentials, databases, and customer chats never leave your server.
- **Autonomous & Self-Healing:** Task execution with live terminal streaming, automatic error diagnosis, and verified outcomes.
- **Calm Interface:** Designed for zero cognitive fatigue inspired by Things 3 and Bear Notes.

---

## 🤝 Community & Sponsors

<!-- SPONSORS:START -->
<!-- SPONSORS:END -->

Want to support open-source development? Back Cuan on [cuan.work/sponsor](https://cuan.work/sponsor).

---

## 📄 License

Cuan is open-source software licensed under the [Apache License 2.0](LICENSE).  
Copyright © 2026 Buchin and Cuan contributors.
