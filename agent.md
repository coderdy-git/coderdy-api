# Project Context: WhatsApp Gateway AI (Golang)

## Overview
A high-performance, multi-instance WhatsApp Gateway built with Golang. This system allows multiple users to register, verify their email, and connect their own WhatsApp accounts via QR Code. Each instance is integrated with an LLM (currently using Sumopod/OpenAI compatible API) for autonomous chat responses with custom system prompts per user.

## Tech Stack
- **Language:** Go (Golang) 1.22+
- **WA Library:** [WhatsMeow](https://github.com/tulir/whatsmeow) (Socket-based, non-browser, very low RAM).
- **Web Framework:** [Gin Gonic](https://github.com/gin-gonic/gin).
- **Database:** SQLite (WAL Mode enabled for production-grade concurrency).
- **Security:** JWT (JSON Web Token) for session management & Bcrypt for password hashing.
- **Email Service:** Resend API (HTTP-based email delivery to bypass VPS SMTP blocks).
- **AI Integration:** OpenAI SDK (Compatible with Sumopod, Groq, OpenRouter).
- **Middleware:** Rate Limiting (ulule/limiter) & Custom Auth Middleware.

## Database Schema (SQLite)
- `users`: Stores account info (`username`, `email`, `password`, `is_verified`, `verification_token`, `system_prompt`).
- `wa_sessions.db`: Managed by `whatsmeow/sqlstore` to store multiple WhatsApp session blobs.
- `wa_history.db`: Stores recent chat history (max 10 messages) for AI context memory.

## Key Features & Logic
1. **Multi-Instance:** Maps every `username` to a unique WhatsApp session. Managed via a global `map[string]*whatsmeow.Client`.
2. **Anti-Ban Human Behavior:**
   - Random response delays (1-3s before seen, 2-4s during typing).
   - Automated "Chat Seen" (Mark Read) status.
   - "Composing" (Typing) indicator during AI processing.
3. **Email Verification:** Registration requires email verification via Resend API before login is allowed.
4. **Custom AI Personalities:** Users can update their own `system_prompt` via API to change their bot's behavior.
5. **Webhook:** Every AI response triggers a POST request to a configured `WEBHOOK_URL` for external monitoring/logging.
6. **Rate Limiting:** Protects endpoints from brute force and spam (Global: 30 requests/min).

## API Endpoints (v1)
### Public
- `GET /api/v1/health`: Uptime & status check.
- `POST /api/v1/auth/register`: Create account (needs Email & Username).
- `POST /api/v1/auth/login`: Get JWT token (Identifier: Username or Email).
- `GET /api/v1/auth/verify-email`: Confirm registration via token.

### Protected (Requires JWT)
- `GET /api/v1/whatsapp/sessions/connect`: Retrieve QR Code for scanning.
- `GET /api/v1/whatsapp/sessions/status`: Check connection status.
- `POST /api/v1/whatsapp/settings/prompt`: Update bot's system instructions.
- `POST /api/v1/whatsapp/messages/send`: Manual message broadcast.

## Development Status
- **LLM:** Currently configured with `gpt-5-mini` on Sumopod.
- **Vision Ready:** Architecture supports multimodal models (image reading) via OpenAI-compatible endpoints.
- **Deployment:** Includes `install.sh` for automated VPS setup with Systemd service for auto-restart.

## Environment Variables (.env)
- `JWT_SECRET`: Secret key for signing tokens.
- `CUSTOM_API_KEY`: LLM API Key.
- `CUSTOM_BASE_URL`: AI Provider endpoint.
- `CUSTOM_MODEL`: AI Model name.
- `RESEND_API_KEY`: Email delivery key.
- `WEBHOOK_URL`: Target for event notifications.
- `DEFAULT_SYSTEM_PROMPT`: Fallback bot personality.
- `APP_URL`: Base URL for verification links.
