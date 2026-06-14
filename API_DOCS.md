# API Documentation - WhatsApp Gateway AI

**Base URL:** `https://api.coderdy.com/api/v1`

## 🔐 Authentication
Sebagian besar endpoint memerlukan **JWT Bearer Token**. Dapatkan token dengan melakukan Login, lalu sertakan pada header request:
`Authorization: Bearer <your_token>`

---

## 🟢 Public Endpoints

### 1. Health Check
Cek apakah server berjalan dengan baik.
- **URL:** `/health`
- **Method:** `GET`
- **Response:**
```json
{
  "code": 200,
  "status": "success",
  "message": "Server is running",
  "data": { "uptime": "1h2m3s" }
}
```

### 2. Register
Mendaftarkan akun baru. System akan mengirimkan email verifikasi.
- **URL:** `/auth/register`
- **Method:** `POST`
- **Body:**
```json
{
  "username": "coderdy_user",
  "email": "user@example.com",
  "password": "secretpassword"
}
```

### 3. Login
Mendapatkan token akses.
- **URL:** `/auth/login`
- **Method:** `POST`
- **Body:**
```json
{
  "identifier": "coderdy_user", // Bisa username atau email
  "password": "secretpassword"
}
```
- **Response:**
```json
{
  "code": 200,
  "status": "success",
  "data": { "token": "eyJhbG..." }
}
```

### 4. Get Current User (Me)
Mendapatkan informasi profil user yang sedang login.
- **URL:** `/auth/me`
- **Method:** `GET`
- **Headers:** `Authorization: Bearer <token>`
- **Response:**
```json
{
  "code": 200,
  "status": "success",
  "data": {
    "username": "coderdy",
    "email": "admin@coderdy.com"
  }
}
```

### 5. Verify Email
Endpoint yang dipanggil dari link di email.
- **URL:** `/auth/verify-email?token=TOKEN_DARI_EMAIL`
- **Method:** `GET`

---

## 🔒 Protected Endpoints (Requires JWT)

### 1. Connect WhatsApp (Pairing)
Memulai sesi WhatsApp dan mendapatkan QR Code.
- **URL:** `/whatsapp/sessions/connect`
- **Method:** `GET`
- **Response (Jika perlu scan):**
```json
{
  "code": 200,
  "status": "scan_required",
  "data": { "qr_code": "1@..." }
}
```

### 2. Check Connection Status
- **URL:** `/whatsapp/sessions/status`
- **Method:** `GET`
- **Response:**
```json
{
  "code": 200,
  "status": "connected",
  "data": { "jid": "628xxx@s.whatsapp.net" }
}
```

### 3. Update AI System Prompt
Mengubah kepribadian atau instruksi bot.
- **URL:** `/whatsapp/settings/prompt`
- **Method:** `POST`
- **Body:**
```json
{
  "prompt": "Kamu adalah admin toko online yang sangat sopan dan teknis."
}
```

### 4. Send Message (Manual)
Kirim pesan WhatsApp secara manual melalui API.
- **URL:** `/whatsapp/messages/send`
- **Method:** `POST`
- **Body:**
```json
{
  "to": "628123456789", // Format angka tanpa @s.whatsapp.net
  "message": "Halo, ini pesan dari API."
}
```

---

## ⚡ Webhook Receiver
Endpoint internal (atau eksternal) yang menerima laporan setiap kali AI membalas pesan.
- **URL:** `/webhook/receiver`
- **Method:** `POST`
- **Headers:** `X-Webhook-Secret: <your_secret>`
- **Payload:**
```json
{
  "event": "message_replied",
  "from": "628xxx@s.whatsapp.net",
  "message": "Halo bot!",
  "reply": "Halo! Ada yang bisa saya bantu?",
  "time": "2024-06-14T10:00:00Z"
}
```
