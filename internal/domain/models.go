package domain

import "time"

// VendorRegisterProfile carries all profile fields submitted during vendor registration.
type VendorRegisterProfile struct {
	FirstName   string
	LastName    string
	ShopName    string
	ShopURL     string
	Phone       string
	Street      string
	Street2     string
	City        string
	ZipCode     string
	Country     string
	State       string
	CompanyName string
	CompanyID   string
	VatID       string
	BankName    string
	AccountIBAN string
}

type Role string

const (
	RoleBuyer      Role = "buyer"
	RoleVendor     Role = "vendor"
	RoleAdminSuper Role = "super_admin"
	RoleAdminOps   Role = "ops_admin"
	RoleAdminFin   Role = "finance_admin"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

type Vendor struct {
	ID     string
	UserID string
	// Personal info
	FirstName string
	LastName  string
	// Shop info
	BusinessName string // shop_name
	ShopURL      string
	Phone        string
	// Address
	Street  string
	Street2 string
	City    string
	ZipCode string
	Country string
	State   string
	// Company / tax
	CompanyName string
	CompanyID   string // Company ID/EUID Number
	VatID       string // VAT/TAX ID
	// Banking / payout
	BankName      string
	AccountNumber string // Account / IBAN
	// Marketplace metadata
	Status             string // pending, approved, suspended, rejected
	CommissionOverride *float64
	CreatedAt          time.Time
}

type Product struct {
	ID                  string
	VendorID            string
	CategoryID          *string
	Name                string
	Description         string
	Price               int64  // kobo
	PricingUnit         string // yard, item, etc.
	StockQuantity       string // decimal stored as text for simplicity v1
	Status              string // draft,published
	ImageURL            *string
	AdminCommissionRate *float64
	CreatedAt           time.Time
}

type Order struct {
	ID          string
	BuyerID     string
	TotalAmount int64
	Status      string // pending,paid,shipped,completed,cancelled
	CreatedAt   time.Time
}

type OrderItem struct {
	ID               string
	OrderID          string
	VendorID         string
	ProductID        string
	Quantity         string // decimal text
	UnitPrice        int64
	Subtotal         int64
	CommissionAmount int64
	CreatedAt        time.Time
}

type Payment struct {
	ID             string
	OrderID        string
	Provider       string
	ProviderRef    string
	Status         string // initiated,success,failed
	Amount         int64
	IdempotencyKey string
	CreatedAt      time.Time
}

type LedgerEntry struct {
	ID        string
	VendorID  string
	Type      string // credit,debit
	Amount    int64
	Reference string
	CreatedAt time.Time
}

type AdminLog struct {
	ID        string
	AdminID   string
	Action    string
	Entity    string
	EntityID  string
	OldValue  *string
	NewValue  *string
	CreatedAt time.Time
}
