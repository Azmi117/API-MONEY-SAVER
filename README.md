# 💰 API Money Saver - Backend SaaS

Aplikasi backend REST API untuk manajemen keuangan personal yang terintegrasi dengan Telegram Bot, OCR Receipt Scanner, dan Email Parsing.

---

## 🚀 Panduan Instalasi & Menjalankan Proyek (Setup Guide)

Ikuti urutan langkah di bawah ini untuk mendirikan dan menjalankan aplikasi di lingkungan lokal.

### 1. Clone Repository
Silakan *clone repository* ini dan masuk ke dalam direktori utama proyek:
```bash
git clone [https://github.com/Azmi117/API-MONEY-SAVER.git] (Tanpa kurung siku)
cd API-MONEY-SAVER

### 2. Instalasi Package / Dependensi
gunakan command : go mod tidy


### 3. Setup Environment Variables (.env)

Duplikat file template .env.example menjadi file .env asli:

Linux/Mac/GitBash: cp .env.example .env

Windows (PowerShell): Copy-Item .env.example -Destination .env

Buka file .env baru tersebut, lalu isi dan sesuaikan semua value (seperti kredensial database, API Key, token Telegram, dll) dengan data lokal.


### 4. Setup Database & Migrasi

Sebelum masuk ke perintah migrasi kode, kita harus buat DB manual dulu di PostgreSQL.

Buka PostgreSQL (bisa lewat pgAdmin, DBeaver, atau terminal psql).

Jalankan perintah SQL untuk membuat database baru yang namanya disesuaikan dengan isi DB_NAME di file .env (EX: CREATE DATABASE api_money_saver;)

Jika database manual sudah terbuat, jalankan script migrasi dari Go untuk mengisi tabel-tabelnya otomatis (EX: go run cmd/migrate/main.go)


### 5. Setup Database & Migrasi

instal aplikasi Air secara global di komputer (jika belum punya) lewat perintah:

go install [github.com/air-verse/air@latest] (Tanpa kurung siku)

setelah menginstall cukup ketik air pada terminal lalu tekan ENTER


### 6. API DOCUMENTATION

Proyek ini sudah dilengkapi dengan UI interaktif yang tertanam langsung di dalam server backend untuk mempermudah pengujian kontrak endpoint.

A. Pastikan server aplikasi sudah berjalan (baik via go run atau air).

B. Buka browser, lalu akses URL berikut: http://localhost:8080/docs/