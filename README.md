# Rhovic Marketplace Backend

> [!NOTE]
> This backend is optimized for Go 1.21+ and PostgreSQL.

The robust, Go-powered core of the Rhovic Emporium Marketplace. This backend handles everything from secure vendor authentication to complex multi-role order management and Paystack-integrated payment processing.

## 🚀 Quick Start

### Prerequisites
- [Go](https://go.dev/doc/install) (1.21 or later)
- [PostgreSQL](https://www.postgresql.org/download/)

### Setup Instructions

1. **Install Dependencies**:
   ```bash
   go mod download
   ```

2. **Configure Environment Variables**:
   Copy `.env.example` to `.env` (or create one) and update the values:
   ```env
   PORT=8080
   ENV=dev
   DB_URL=postgres://user:pass@localhost:5432/rhovic?sslmode=disable
   JWT_KEY=your-secure-random-string
   PAYSTACK_SECRET_KEY=your_paystack_secret
   ```

3. **Database Migrations**:
   Run the SQL migrations located in the `/migrations` folder against your PostgreSQL instance.

4. **Run the Server**:
   ```bash
   go run cmd/api/main.go
   ```

## 🛠 Technical Stack

| Component | Technology |
| :--- | :--- |
| **Language** | Go (Golang) |
| **Router** | [chi v5](https://github.com/go-chi/chi) |
| **Database Driver** | [pgx v5](https://github.com/jackc/pgx) |
| **Token Auth** | JWT ([golang-jwt](https://github.com/golang-jwt/jwt)) |
| **Password Hashing** | Bcrypt (`golang.org/x/crypto/bcrypt`) |
| **Payments** | Paystack Integration |

## 📁 Project Structure

```text
backend/
├── cmd/api/          # Entry point (main.go)
├── internal/
│   ├── handlers/     # HTTP/API Layer (request/response)
│   ├── services/     # Business Logic Layer
│   ├── repo/         # Data Access Layer (SQL queries)
│   ├── server/       # Route definitions & Middleware setup
│   ├── config/       # Environment loading
│   ├── db/           # Connection pooling
│   └── paystack/     # Payment gateway client
└── migrations/       # SQL schema definitions
```

## 📡 Core API Endpoints

### Authentication (`/auth`)
- `POST /auth/register` - Create a new account
- `POST /auth/login` - Get access & refresh tokens
- `POST /auth/refresh` - Refresh an expired access token

### Products (`/products`)
- `GET /products` - List all verified products
- `GET /products/{id}` - View product details

### Orders & Checkout (`/orders`)
- `POST /orders/checkout` - Initialize a purchase (Requires `buyer` role)

### Vendor Operations (`/vendor`)
- `POST /vendor/products` - List a new product
- `GET /vendor/orders` - View sales metrics
- `POST /vendor/payouts/request` - Request fund withdrawal

### Administration (`/admin`)
- `GET /admin/metrics` - Platform-wide statistics
- `PATCH /admin/payouts/{id}/approve` - Facilitate vendor payouts

## 🔒 Security Features
- **Role-Based Access Control (RBAC)**: Secure middleware enforcing `buyer`, `vendor`, and `admin` permissions.
- **Rate Limiting**: Integrated hardening on auth routes to prevent brute-force attacks.
- **Signature Verification**: Validating Paystack webhooks to ensure payment integrity.
