# 📘 API MONEY SAVER - API Documentation

Welcome to the API Documentation for **API MONEY SAVER**. This guide provides an in-depth reference for all API endpoints available in the system.

## 🔑 Global Configuration & Authentication

### Base URL
```
http://localhost:8080/api/v1
```

### Authentication Method
Authentication is handled using **JSON Web Tokens (JWT)** stored in secure, **HttpOnly Cookies**:
1. **`access_token`**: Used to authenticate request sessions (Expires in 15 minutes).
2. **`refresh_token`**: Used to retrieve a new `access_token` when it expires (Expires in 7 days).

Endpoints labeled with `[Protected]` require a valid `access_token` cookie.
Endpoints labeled with `[Workspace Owner]` require the user to be the owner of the workspace.
Endpoints labeled with `[Workspace Member]` require the user to be a member (or owner) of the workspace.

### Standard Response Formats

#### Success Response
Most endpoints return a standard JSON structure wrapped in the `APIResponse` format:
```json
{
  "status": "success",
  "message": "Action completed successfully",
  "data": null // can be object, array, or null
}
```

#### Error Response
When a request fails, it returns a standard error JSON format:
```json
{
  "status": "error",
  "message": "Detailed error message",
  "data": null
}
```

---

## 📂 Table of Contents
1. [Authentication & Profile](#1-authentication--profile)
2. [Workspace & Target Management](#2-workspace--target-management)
3. [Workspace Invitations](#3-workspace-invitations)
4. [Category Management](#4-category-management)
5. [Transactions](#5-transactions)
6. [Email Integrations & Pending Logs](#6-email-integrations--pending-logs)
7. [Debts & Split Bill](#7-debts--split-bill)
8. [Static Assets](#8-static-assets)

---

## 1. Authentication & Profile

### Register User
Create a new user account. Upon registration, an OTP code will be sent to the registered email.
* **Endpoint**: `POST /auth/register`
* **Content-Type**: `multipart/form-data`
* **Request Fields**:
  * `name` (string, required): Full name.
  * `email` (string, required): Active email address.
  * `password` (string, required): Minimum 6 characters.
  * `telegram_id` (integer, optional): Telegram account ID.
  * `email_parsing_enable` (string: `"true"` or `"false"`, optional): Enable Gmail processing.
  * `avatar` (file, optional): Profile picture.
* **Response (201 Created)**:
  ```json
  {
    "status": "success",
    "message": "Registration successful. Please verify your email.",
    "data": null
  }
  ```

### Verify Register OTP
Activate the registered user account using the OTP code received in email.
* **Endpoint**: `POST /auth/register/verify`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "code": "123456"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Account successfully activated. Please log in.",
    "data": null
  }
  ```

### User Login (Step 1)
Authenticate email and password. If correct, triggers an OTP code to be sent to email.
* **Endpoint**: `POST /auth/login`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "password": "yourpassword"
  }
  ```
* **Response (202 Accepted)**:
  ```json
  {
    "status": "success",
    "message": "Password verified. An OTP security code has been sent to your email.",
    "data": null
  }
  ```
* **Response (403 Forbidden - Account Unverified)**:
  ```json
  {
    "status": "error",
    "message": "Not verified",
    "data": {
      "code": "USER_NOT_VERIFIED",
      "email": "user@example.com"
    }
  }
  ```

### Verify Login OTP (Step 2)
Exchange the login OTP for `access_token` and `refresh_token` cookies.
* **Endpoint**: `POST /auth/login/verify`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "code": "123456"
  }
  ```
* **Headers Set**:
  * Set-Cookie: `access_token=...; HttpOnly; Path=/; Max-Age=900`
  * Set-Cookie: `refresh_token=...; HttpOnly; Path=/; Max-Age=604800`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Login verification successful. Welcome!",
    "data": null
  }
  ```

### Resend OTP
Resend OTP code for registration or login.
* **Endpoint**: `POST /auth/otp/resend`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "type": "login" // or "register"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "OTP code has been successfully resent to your email.",
    "data": null
  }
  ```

### Request Password Reset
Send a password reset link/OTP code to user's email.
* **Endpoint**: `POST /auth/forgot-password`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "OTP code for password reset has been sent to your email.",
    "data": null
  }
  ```

### Reset Password
Update user password using the reset OTP code.
* **Endpoint**: `POST /auth/forgot-password/verify`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "user@example.com",
    "code": "123456",
    "new_password": "newsecurepassword"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Your password has been successfully reset. Please log in with your new password.",
    "data": null
  }
  ```

### Refresh Session Tokens
Refresh session using the `refresh_token` cookie and obtain a new `access_token` cookie.
* **Endpoint**: `POST /auth/refresh`
* **Cookies Required**: `refresh_token`
* **Headers Set**:
  * Set-Cookie: `access_token=...; HttpOnly; Path=/; Max-Age=900`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Token successfully refreshed",
    "data": null
  }
  ```

### User Logout `[Protected]`
Invalidate current session tokens and clear authentication cookies.
* **Endpoint**: `POST /auth/logout`
* **Headers Set**: Clears `access_token` and `refresh_token` cookies.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Logout successful and session cleared",
    "data": null
  }
  ```

### Google SSO Login Link
Redirects user to Google OAuth2 consent screen.
* **Endpoint**: `GET /auth/sso/google/login`
* **Response**: `307 Temporary Redirect` to Google OAuth page.

### Google SSO Callback
Handles authentication code received from Google.
* **Endpoint**: `GET /auth/sso/google/callback`
* **Query Parameters**:
  * `code` (string): Authorization code.
  * `state` (string): State string for security validation.
* **Headers Set**: Sets `access_token` and `refresh_token` cookies.
* **Response**: `303 See Other` redirects user to dashboard (`http://localhost:5173/dashboard`).

### Get User Profile `[Protected]`
Fetch profile details of the authenticated user.
* **Endpoint**: `GET /auth/profile`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Profile retrieved",
    "data": {
      "ID": 1,
      "CreatedAt": "2026-06-22T15:16:18Z",
      "UpdatedAt": "2026-06-22T15:16:18Z",
      "name": "Alex",
      "email": "user@example.com",
      "avatar": "http://localhost:8080/uploads/avatars/1_1708899.jpg",
      "telegram_id": null,
      "email_parsing_enable": true,
      "account_tier": "free",
      "gmail_enabled": false
    }
  }
  ```

### Update User Profile `[Protected]`
Modify user's profile information.
* **Endpoint**: `PUT /auth/profile`
* **Content-Type**: `multipart/form-data`
* **Request Fields**:
  * `name` (string, required): New profile name.
  * `avatar` (file, optional): New profile picture.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Profil berhasil diupdate",
    "data": {
      "name": "Alex Smith",
      "avatar": "http://localhost:8080/uploads/avatars/1_1719000000.jpg"
    }
  }
  ```

### Get Telegram Binding Code `[Protected]`
Generate a code to bind the user account to the Telegram bot.
* **Endpoint**: `GET /auth/telegram/binding-code`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Binding code generated successfully",
    "data": {
      "binding_code": "TELE-BIND-12345",
      "instruction": "Send this code to the Telegram bot via /bind [code]"
    }
  }
  ```

### Get Gmail Integration Login URL `[Protected]`
Generates Gmail integration login link for active email scanner sync.
* **Endpoint**: `GET /auth/google/login`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Google Auth URL generated",
    "data": {
      "url": "https://accounts.google.com/o/oauth2/v2/auth?..."
    }
  }
  ```

### Google Gmail Integration Callback
Handles callback code from Google for email sync integration.
* **Endpoint**: `GET /auth/google/callback`
* **Query Parameters**:
  * `code` (string): Authorization code.
  * `state` (string): User ID mapped into state.
* **Response**: `307 Temporary Redirect` back to frontend profile page (`/profile?sync=success`).

---

## 2. Workspace & Target Management

### Create Workspace `[Protected]`
Initialize a new financial workspace.
* **Endpoint**: `POST /workspaces`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "name": "Family Expense",
    "type": "budgeting" // e.g., "budgeting" or "saving"
  }
  ```
* **Response (201 Created)**:
  ```json
  {
    "status": "success",
    "message": "Workspace created successfully",
    "data": {
      "id": 5,
      "name": "Family Expense",
      "type": "budgeting",
      "owner_id": 1,
      "created_at": "2026-06-22T22:30:00Z"
    }
  }
  ```

### Get My Workspaces `[Protected]`
Fetch all workspaces owned by or shared with the authenticated user.
* **Endpoint**: `GET /workspaces`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Workspaces retrieved successfully",
    "data": [
      {
        "ID": 5,
        "name": "Family Expense",
        "owner_id": 1,
        "type": "budgeting"
      }
    ]
  }
  ```

### Update Workspace `[Protected]` `[Workspace Owner]`
Update workspace parameters.
* **Endpoint**: `PUT /workspaces/{id}`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "name": "Office Expense"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Workspace updated successfully",
    "data": null
  }
  ```

### Delete Workspace `[Protected]` `[Workspace Owner]`
Remove a workspace permanently.
* **Endpoint**: `DELETE /workspaces/{id}`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Workspace deleted successfully",
    "data": null
  }
  ```

### Set Workspace Budget Limits `[Protected]`
Set maximum spending limit and savings target for a workspace during a specific monthly period.
* **Endpoint**: `POST /workspaces/target`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "workspace_id": 5,
    "period": "2026-06",
    "amount_limit": 5000000,
    "savings_target": 2000000
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Target for period 2026-06 has been set successfully",
    "data": null
  }
  ```

### Get Workspace Members `[Protected]`
Retrieve lists of users who belong to the workspace.
* **Endpoint**: `GET /workspaces/{id}/members`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Workspace members retrieved successfully",
    "data": [
      {
        "user_id": 1,
        "user_name": "Alex",
        "email": "user@example.com",
        "role": "owner"
      }
    ]
  }
  ```

### Get Workspace Monthly Summary `[Protected]`
Retrieve budget status and summary of spending details for each workspace member in a period.
* **Endpoint**: `GET /workspaces/{id}/summary`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Query Parameters**:
  * `period` (string, optional): Format `YYYY-MM`. Defaults to the current month.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Summary retrieved",
    "data": {
      "period": "2026-06",
      "limit_amount": 5000000.00,
      "total_expense": 1500000.00,
      "remaining_budget": 3500000.00,
      "expense_details": [
        {
          "user_name": "Alex",
          "total": 1500000.00
        }
      ]
    }
  }
  ```

### Export Transactions as PDF `[Protected]` `[Workspace Member]`
Generate and download workspace transaction reports for a specific month.
* **Endpoint**: `GET /workspaces/{id}/transactions/export`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Query Parameters**:
  * `month` (string, optional): Format `YYYY-MM`. Defaults to all transactions if empty.
* **Response (200 OK)**: File attachment streaming.
  * Headers:
    * `Content-Type: application/pdf`
    * `Content-Disposition: attachment; filename=report_workspace_5_2026-06.pdf`

---

## 3. Workspace Invitations

### Get Pending Invitations `[Protected]`
Fetch list of workspaces that have invited the authenticated user.
* **Endpoint**: `GET /workspaces/invitations/pending`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Pending invitations retrieved",
    "data": [
      {
        "id": 12,
        "workspaceName": "Project Funding",
        "sender": "John Doe",
        "status": "pending"
      }
    ]
  }
  ```

### Invite Member `[Protected]`
Send an invitation to join a workspace using the invitee's email.
* **Endpoint**: `POST /workspaces/{id}/invite`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "email": "invited_user@example.com"
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Invitation sent successfully to invited_user@example.com",
    "data": null
  }
  ```

### Accept Invitation `[Protected]`
Accept a pending workspace invitation.
* **Endpoint**: `POST /workspaces/invitations/{id}/accept`
* **Path Parameters**:
  * `id` (integer): Invitation ID.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Successfully joined the workspace",
    "data": null
  }
  ```

### Reject Invitation `[Protected]`
Reject a pending workspace invitation.
* **Endpoint**: `POST /workspaces/invitations/{id}/reject`
* **Path Parameters**:
  * `id` (integer): Invitation ID.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Invitation rejected successfully",
    "data": null
  }
  ```

---

## 4. Category Management

### Create Category `[Protected]` `[Workspace Owner]`
Create a custom category for expense/income tracking inside a workspace.
* **Endpoint**: `POST /workspaces/{id}/categories`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "name": "Food & Beverage",
    "type": "expense", // "expense" or "income"
    "icon": "fast-food" // optional icon key
  }
  ```
* **Response (201 Created)**:
  ```json
  {
    "ID": 10,
    "CreatedAt": "2026-06-22T22:30:00Z",
    "UpdatedAt": "2026-06-22T22:30:00Z",
    "name": "Food & Beverage",
    "workspace_id": 5,
    "type": "expense",
    "icon": "fast-food"
  }
  ```

### Get Categories `[Protected]`
Retrieve all categories available inside a workspace.
* **Endpoint**: `GET /workspaces/{id}/categories`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Response (200 OK)**:
  ```json
  {
    "data": [
      {
        "ID": 1,
        "name": "General Income",
        "workspace_id": 5,
        "type": "income",
        "icon": "wallet"
      },
      {
        "ID": 10,
        "name": "Food & Beverage",
        "workspace_id": 5,
        "type": "expense",
        "icon": "fast-food"
      }
    ]
  }
  ```

---

## 5. Transactions

### Record Transaction Manually `[Protected]`
Add a transaction to the workspace manually.
* **Endpoint**: `POST /transactions/manual`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "workspace_id": 5,
    "category_id": 10,
    "amount": 75000,
    "type": "expense", // "expense" or "income"
    "date": "2026-06-22T22:00:00Z",
    "note": "Lunch with team",
    "merchant": "McDonalds",
    "source": "web", // "web" or "telegram"
    "method": "QRIS", // e.g. "Cash", "Debit", "QRIS"
    "gmail_id": "" // optional identifier
  }
  ```
* **Response (201 Created)**:
  ```json
  {
    "status_code": 201,
    "message": "Manual transaction recorded successfully",
    "data": {
      "ID": 101,
      "user_id": 1,
      "workspace_id": 5,
      "category_id": 10,
      "amount": 75000,
      "type": "expense",
      "method": "QRIS",
      "date": "2026-06-22T00:00:00Z",
      "note": "Lunch with team",
      "merchant": "McDonalds",
      "source": "web",
      "status": "approved"
    }
  }
  ```

### Get Transaction History `[Protected]` `[Workspace Member]`
Retrieve list of verified transactions in the workspace with pagination.
* **Endpoint**: `GET /workspaces/{id}/transactions`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Query Parameters**:
  * `page` (integer, optional): Page index. Default is `1`.
  * `limit` (integer, optional): Number of items per page. Default is `10`.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Transaction history retrieved successfully",
    "data": [
      {
        "ID": 101,
        "user_id": 1,
        "workspace_id": 5,
        "amount": 75000,
        "type": "expense",
        "date": "2026-06-22T00:00:00Z",
        "merchant": "McDonalds",
        "status": "approved"
      }
    ],
    "meta": {
      "page": 1,
      "limit": 10,
      "total_items": 1,
      "total_pages": 1
    }
  }
  ```

### Delete Transaction `[Protected]`
Deletes a transaction from the workspace.
* **Endpoint**: `DELETE /transactions/{id}`
* **Path Parameters**:
  * `id` (integer): Transaction ID.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Transaction deleted successfully",
    "data": null
  }
  ```

### Get Pending Transactions `[Protected]` `[Workspace Member]`
Retrieve pending receipt uploads / email drafts that are awaiting user review and confirmation.
* **Endpoint**: `GET /workspaces/{id}/pending-transactions`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Query Parameters**:
  * `page` (integer, optional): Page index. Default `1`.
  * `limit` (integer, optional): Items per page. Default `10`.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Pending transactions retrieved successfully",
    "data": [
      {
        "id": 14,
        "user_id": 1,
        "workspace_id": 5,
        "source": "telegram_alt",
        "raw_data": "{\"merchant\": \"Starbucks\", \"amount\": 45000, ...}",
        "status": "pending",
        "image_path": "uploads/pending/receipt_14.jpg",
        "created_at": "2026-06-22T21:40:00Z"
      }
    ],
    "meta": {
      "page": 1,
      "limit": 10,
      "total_items": 1,
      "total_pages": 1
    }
  }
  ```

### Confirm/Edit Pending Transaction `[Protected]`
Confirm a pending receipt transaction with customized fields (like category selection, item splits, etc.).
* **Endpoint**: `PATCH /transactions/{id}/confirm`
* **Path Parameters**:
  * `id` (integer): Pending transaction ID.
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "workspace_id": 5,
    "merchant": "Starbucks",
    "amount": 45000,
    "payer_id": 1,
    "date": "2026-06-22",
    "type": "expense",
    "category_id": 10,
    "note": "Coffee sync",
    "method": "QRIS",
    "gmail_id": "",
    "items": [
      {
        "description": "Caffe Latte",
        "price": 45000,
        "quantity": 1,
        "user_id": 1
      }
    ]
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Transaction confirmed successfully",
    "data": {
      "ID": 102,
      "user_id": 1,
      "workspace_id": 5,
      "amount": 45000,
      "type": "expense",
      "status": "approved",
      "items": [
        {
          "transaction_id": 102,
          "description": "Caffe Latte",
          "quantity": 1,
          "price": 45000,
          "total": 45000
        }
      ]
    }
  }
  ```

### Scan Receipt Hybrid (Tesseract + Gemini) `[Protected]`
Upload a receipt picture to be processed by a hybrid OCR parser (Tesseract OCR followed by LLM refinement).
* **Endpoint**: `POST /transactions/scan-hybrid2`
* **Content-Type**: `multipart/form-data`
* **Request Fields**:
  * `image` (file, required): Image file of receipt (jpeg/png). Max 5MB.
  * `workspace_id` (integer, required): ID of target workspace.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Receipt scanned successfully (Pending)",
    "data": {
      "transaction": {
        "user_id": 1,
        "workspace_id": 5,
        "amount": 89000,
        "merchant": "Superindo",
        "date": "2026-06-22T00:00:00Z",
        "items": [
          {
            "description": "Fresh Milk",
            "quantity": 2,
            "price": 24500,
            "total": 49000
          },
          {
            "description": "Snacks Pack",
            "quantity": 1,
            "price": 40000,
            "total": 40000
          }
        ]
      },
      "engine": "hybrid",
      "confidence": 85,
      "fallback_used": false
    },
    "pending_id": 15
  }
  ```

### Scan Receipt Alternative (OCR Space) `[Protected]`
Upload receipt using OCR Space external service processor.
* **Endpoint**: `POST /transactions/scan-alt`
* **Content-Type**: `multipart/form-data`
* **Request Fields**:
  * `file` (file, required): Receipt image file. Max 10MB.
  * `workspace_id` (integer, required): ID of target workspace.
* **Response (200 OK)**:
  ```json
  {
    "status_code": 200,
    "message": "Scan successful, pending confirmation",
    "data": {
      "transaction": {
        "user_id": 1,
        "workspace_id": 5,
        "amount": 120000,
        "merchant": "Pizza Hut",
        "items": []
      }
    },
    "pending_id": 16
  }
  ```

### Confirm Alternative Scan `[Protected]`
Directly confirm parsed data from Alternative Scan and save as a final transaction.
* **Endpoint**: `POST /transactions/scan-alt/confirm`
* **Content-Type**: `application/json`
* **Request Body**: (Same format as `Confirm/Edit Pending Transaction`)
* **Response (201 Created)**:
  ```json
  {
    "status_code": 201,
    "message": "Transaction confirmed and saved",
    "data": {
      "ID": 103,
      "amount": 120000,
      "merchant": "Pizza Hut",
      "status": "approved"
    }
  }
  ```

---

## 6. Email Integrations & Pending Logs

### Get Pending Emails `[Protected]`
Retrieve parsed bank transaction notifications received from user's synced Gmail inbox that require review.
* **Endpoint**: `GET /emails/pending`
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Pending email logs retrieved successfully",
    "data": [
      {
        "ID": 3,
        "user_id": 1,
        "amount": 350000,
        "merchant": "Transfer Out - Budi",
        "parsed_date": "2026-06-22T00:00:00Z",
        "status": "Pending",
        "bank_source": "Mandiri",
        "gmail_id": "GMAIL-123456",
        "method": "Transfer",
        "type": "expense"
      }
    ]
  }
  ```

### Approve Parsed Email `[Protected]`
Approve a pending parsed email and record it as a transaction inside a specific workspace.
* **Endpoint**: `POST /emails/{id}/approve`
* **Path Parameters**:
  * `id` (integer): ID of parsed email.
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "workspace_id": 5
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Email approved and transaction created",
    "data": {
      "budget_status": {
        "period": "2026-06",
        "limit_amount": 5000000.00,
        "total_expense": 1850000.00,
        "remaining_budget": 3150000.00,
        "expense_details": []
      }
    }
  }
  ```

### Reject Parsed Email `[Protected]`
Dismiss a parsed email log notification.
* **Endpoint**: `POST /emails/{id}/reject`
* **Path Parameters**:
  * `id` (integer): ID of parsed email.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Email log rejected successfully",
    "data": null
  }
  ```

---

## 7. Debts & Split Bill

### Get Workspace Debts/Claims `[Protected]`
Fetch active mutual bills, debts, and claims between members in a workspace.
* **Endpoint**: `GET /workspaces/{id}/debts`
* **Path Parameters**:
  * `id` (integer): Workspace ID.
* **Response (200 OK)**:
  ```json
  {
    "message": "Daftar tagihan berhasil ditarik",
    "data": [
      {
        "ID": 21,
        "workspace_id": 5,
        "from_user_id": 2,
        "to_user_id": 1,
        "amount": 25000.00,
        "short_code": "D39F1",
        "note": "Lunch McDonalds split - Burger",
        "is_paid": false,
        "from_user": { "name": "Budi", "email": "budi@example.com" },
        "to_user": { "name": "Alex", "email": "user@example.com" }
      }
    ]
  }
  ```

### Assign Split Bill `[Protected]`
Split transaction items among workspace members. This automatically generates debt ledger claims in the database.
* **Endpoint**: `POST /transactions/split`
* **Content-Type**: `application/json`
* **Request Body**:
  ```json
  {
    "transaction_id": 102,
    "items": [
      {
        "item_name": "Caffe Latte",
        "user_id": 2,
        "quantity": 1,
        "price": 45000
      }
    ]
  }
  ```
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Split bill processed successfully",
    "data": null
  }
  ```

### Pay / Mark Debt as Paid `[Protected]`
Mark a debt as paid.
* **Endpoint**: `PATCH /debts/{id}/pay`
* **Path Parameters**:
  * `id` (integer): Debt ID.
* **Response (200 OK)**:
  ```json
  {
    "status": "success",
    "message": "Debt successfully marked as paid!",
    "data": null
  }
  ```

---

## 8. Static Assets

### Get Uploaded File
Direct access route to serve static user uploads (like profile avatars or scanned transaction images).
* **Endpoint**: `GET /uploads/{filepath}`
* **Path Parameters**:
  * `filepath` (string): Relative path to file (e.g., `avatars/1_1708899.jpg` or `pending/receipt_14.jpg`).
* **Response**: Binary stream of the request resource with appropriate MIME header.
