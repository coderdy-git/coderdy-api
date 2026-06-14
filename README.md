# WhatsApp Gateway AI (Golang)

High-performance multi-instance WhatsApp Gateway built with Golang, integrated with LLM (OpenAI Compatible) for autonomous chat responses.

## 🚀 Fitur Utama
- **Multi-Instance:** Kelola banyak akun WhatsApp dalam satu server.
- **AI Autonomous:** Respon otomatis menggunakan model LLM (GPT-4, Groq, Sumopod, dll).
- **Human-Like Behavior:** Jeda waktu acak, status "Typing", dan "Mark Read" otomatis untuk mencegah ban.
- **Email Verification:** Sistem pendaftaran user dengan verifikasi email via Resend API.
- **Internal Webhook:** Menerima laporan aktivitas chat secara real-time ke URL yang dikonfigurasi.
- **SQLite Support:** Menggunakan SQLite dengan WAL Mode untuk performa database yang efisien.

## 🛠 Tech Stack
- **Language:** Go 1.22+
- **Framework:** Gin Gonic
- **WA Library:** [WhatsMeow](https://github.com/tulir/whatsmeow)
- **Database:** SQLite
- **Email:** Resend API
- **AI:** OpenAI SDK (Compatible)

## 📋 Persyaratan Sistem
- VPS dengan OS Linux (Ubuntu disarankan).
- Domain terhubung ke VPS (untuk akses API & Webhook).
- API Key dari Resend.com (dan domain yang sudah diverifikasi).
- API Key LLM (OpenAI, Groq, atau Sumopod).

## ⚙️ Konfigurasi (.env)
Buat file `.env` di root direktori:

```env
PORT=8080
APP_URL=https://api.coderdy.com
JWT_SECRET=your_secret_key

# AI Configuration
CUSTOM_API_KEY=sk-xxxx
CUSTOM_BASE_URL=https://ai.sumopod.com/v1
CUSTOM_MODEL=gpt-5-mini
DEFAULT_SYSTEM_PROMPT="Kamu adalah asisten WhatsApp yang ramah."

# Email Config (Resend)
RESEND_API_KEY=re_xxxx
EMAIL_FROM=no-reply@coderdy.com

# Webhook Config
WEBHOOK_URL=https://api.coderdy.com/api/v1/webhook/receiver
WEBHOOK_SECRET=your_webhook_secret
```

## 🚀 Instalasi & Menjalankan

1. **Clone Repository:**
   ```bash
   git clone https://github.com/coderdy-git/coderdy-api.git
   cd coderdy-api
   ```

2. **Install Dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build Aplikasi:**
   ```bash
   go build -o main main.go
   ```

4. **Jalankan:**
   ```bash
   ./main
   ```

## 📡 API Endpoints (v1)

### Public
- `POST /api/v1/auth/register` - Daftar akun baru.
- `POST /api/v1/auth/login` - Login & dapatkan JWT Token.
- `GET /api/v1/auth/verify-email` - Verifikasi akun via link email.
- `GET /api/v1/health` - Cek status server.

### Terproteksi (Bearer Token)
- `GET /api/v1/whatsapp/sessions/connect` - Ambil QR Code untuk pairing.
- `GET /api/v1/whatsapp/sessions/status` - Cek status koneksi WhatsApp.
- `POST /api/v1/whatsapp/settings/prompt` - Update System Prompt AI.
- `POST /api/v1/whatsapp/messages/send` - Kirim pesan manual.
- `POST /api/v1/webhook/receiver` - Endpoint internal penerima laporan chat.

## 🛡 Keamanan
- Gunakan HTTPS untuk semua request API.
- Jangan membagikan `JWT_SECRET` atau API Key Anda.
- Secara berkala cek dashboard Resend untuk monitoring pengiriman email.

---
Dikembangkan oleh **Coderdy-Git**.
