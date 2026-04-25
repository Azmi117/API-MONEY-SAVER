# 🚀 Backlog & Future Enhancements

Daftar fitur tambahan yang akan dikembangkan setelah Core Feature selesai.

---

### 🛡️ High Priority Security
* [ ] **Email OTP Verification** *Tujuan: Memastikan user menggunakan email asli dan mencegah spam dummy account.*
* [ ] **Global Rate Limiter** *Tujuan: Proteksi seluruh endpoint API dari serangan DDoS dan Brute Force (Hemat resource server).*

### ✨ User Experience (UX)
* [ ] **OAuth2 Integration** *Tujuan: Login cepat via Google atau GitHub.*
* [ ] **Audit Logs** *Tujuan: Mencatat aktivitas penting user (siapa ngelakuin apa) buat keperluan tracking.*

### 🧹 Database Maintenance
* [ ] **Automatic Pending Cleanup (Worker)** *Tujuan: Membersihkan data transaksi berstatus pending yang lebih tua dari 24 jam secara otomatis menggunakan Goroutine/Ticker (Menjaga kebersihan storage).*

### 💳 Monetization & Payment
* [ ] **Payment Gateway Integration (Midtrans/Xendit)** *Tujuan: Implementasi fitur donasi atau subscription premium untuk akses fitur advanced (Unlimited scan/AI insights)*

---