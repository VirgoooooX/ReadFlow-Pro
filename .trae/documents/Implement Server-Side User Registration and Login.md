# Implement Server-Side User Registration and Login

## 1. Backend Changes (ReadFlow Gateway)

### Database Schema Update
- Modify `internal/db/schema.go`: Add `password_hash` column to `users` table.
- Modify `internal/db/db.go`:
    - Update `User` struct to include `PasswordHash string`.
    - Update `migrate` function to ensure `password_hash` column exists in `users` table.

### Database Operations
- Modify `internal/db/operations.go`:
    - Add `CreateUser(username, passwordHash string) (*User, error)` function.
    - Update `GetUserByUsername`, `GetUserByID`, `GetUserByToken` to scan `password_hash`.

### Authentication Logic
- Modify `internal/api/auth.go`:
    - Add `RegisterRequest` struct.
    - Add `Register` method to `AuthService`.
    - Update `Login` method to verify password using `bcrypt` instead of global server password.
    - Add helper functions `HashPassword` and `CheckPassword` using `golang.org/x/crypto/bcrypt`.

### API Routes
- Modify `cmd/server/main.go`:
    - Register `POST /api/auth/register` route.

## 2. Frontend Changes (ReadFlow Pro)

### Settings Configuration
- Modify `src/services/SettingsService.ts`:
    - Update `getProxyModeConfig` to return `http://192.168.31.27:8080` as the default `serverUrl` if not configured.

### Authentication Service
- Modify `src/services/AuthService.ts`:
    - Replace `mockLogin` and `mockRegister` with real API calls.
    - Use `SettingsService` to get the server URL (defaulting to the one we set).
    - Implement `login` to call `/api/auth/login`.
    - Implement `register` to call `/api/auth/register`.
    - On successful login, update `SettingsService` proxy config with the new token so that `ProxyRSSService` works correctly.
    - Update `validateToken` to verify token against server (or at least check format/expiry locally).

## 3. Verification
- Verify database migration adds the column.
- Verify User Registration works (creates user with hashed password).
- Verify User Login works (verifies password and issues token).
- Verify Client connects to `192.168.31.27:8080`.
