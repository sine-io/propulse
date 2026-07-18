package capacity

import (
	"context"
	"errors"
	"time"

	appasset "github.com/sine-io/propulse/internal/application/asset"
	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

var ErrCalculationNotFound = errors.New("capacity calculation not found")
var ErrInvalidCalculationQuery = errors.New("invalid capacity calculation query")
var ErrPolicyNotFound = errors.New("capacity policy not found")
var ErrPolicyConflict = errors.New("capacity policy effective range conflicts with an existing version")
var ErrInvalidPolicy = errors.New("invalid capacity policy")
var ErrInvalidSelection = errors.New("invalid capacity property selection")
var ErrSelectedAssetNotFound = errors.New("selected property asset not found")
var ErrTargetListingNotFound = errors.New("selected target listing not found")
var ErrTargetListingUnavailable = errors.New("selected target listing unavailable")

type CalculationRepository interface {
	Save(ctx context.Context, record CalculationRecord) (CalculationRecord, error)
	FindByUser(ctx context.Context, userID, id string) (CalculationRecord, error)
	FindLatestByUser(ctx context.Context, userID string) (CalculationRecord, error)
	ListByUser(ctx context.Context, filter CalculationListFilter) (CalculationHistoryPage, error)
}

type PolicyRepository interface {
	FindEffective(ctx context.Context, city string, asOf time.Time) (domaincapacity.HousingPolicyVersion, error)
	List(ctx context.Context, city string) ([]domaincapacity.HousingPolicyVersion, error)
	Create(ctx context.Context, policy domaincapacity.HousingPolicyVersion) (domaincapacity.HousingPolicyVersion, error)
}

type AssetReader interface {
	GetAsset(context.Context, appasset.GetAssetQuery) (domainasset.Asset, error)
}

type TargetListingReader interface {
	GetListing(context.Context, appcommunitymarket.GetListingQuery) (appcommunitymarket.MarketListingDetail, error)
}

type CalculationRecord struct {
	ID               string
	UserID           string
	Input            domaincapacity.HousingCapacityInput
	Result           domaincapacity.HousingCapacityResult
	SelectionContext *SelectionContext
	CreatedAt        time.Time
}

type CalculationListFilter struct {
	UserID   string
	Query    string
	Page     int
	PageSize int
}

type CalculationSummary struct {
	ID                     string
	CreatedAt              time.Time
	PressureLevel          domaincapacity.PressureLevel
	TargetTotalPrice       float64
	TargetNeighborhoodName string
	TargetLayout           string
	OldHomeName            string
}

type CalculationHistoryPage struct {
	Items    []CalculationSummary
	Total    int64
	Page     int
	PageSize int
}

type LoanOption struct {
	Type                         domaincapacity.LoanType
	DownPaymentRate              float64
	CommercialAnnualInterestRate *float64
	ProvidentAnnualInterestRate  *float64
}

type AssumptionsView struct {
	Legacy            domaincapacity.Assumptions
	Policy            *domaincapacity.HousingPolicyVersion
	HomePurchaseOrder domaincapacity.HomePurchaseOrder
	LoanTermMonths    int
	LoanOptions       []LoanOption
	Disclaimer        string
}
