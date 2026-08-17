package model

import "time"

type ReportStatus string
type DeletionStatus string

type ReportReason string
type DeletionReason string

const (
	ReportStatusOpen      ReportStatus = "open"
	ReportStatusResolved  ReportStatus = "resolved"
	ReportStatusDismissed ReportStatus = "dismissed"
)

const (
	ReportReasonDuplicate             ReportReason = "DUPLICATE"
	ReportReasonInappropriateContent  ReportReason = "INAPPROPRIATE_CONTENT"
	ReportReasonIncorrectInformation  ReportReason = "INCORRECT_INFORMATION"
	ReportReasonOutOfProduction       ReportReason = "OUT_OF_PRODUCTION"
	ReportReasonOther                 ReportReason = "OTHER"
)

const (
	DeletionStatusPending   DeletionStatus = "pending"
	DeletionStatusApproved  DeletionStatus = "approved"
	DeletionStatusRejected  DeletionStatus = "rejected"
)

const (
	DeletionReasonDuplicate     DeletionReason = "DUPLICATE"
	DeletionReasonInappropriate DeletionReason = "INAPPROPRIATE_CONTENT"
	DeletionReasonOutOfProduction DeletionReason = "OUT_OF_PRODUCTION"
	DeletionReasonOther         DeletionReason = "OTHER"
)

type BeerReport struct {
	ID          string      `json:"id"`
	BeerID      string      `json:"beerId"`
	UserID      string      `json:"userId"`
	Reason      ReportReason `json:"reason"`
	Description string      `json:"description"`
	Status      ReportStatus `json:"status"`
	CreatedAt   time.Time   `json:"createdAt"`
	ResolvedBy  *string     `json:"resolvedBy,omitempty"`
	ResolvedAt  *time.Time  `json:"resolvedAt,omitempty"`
}

type BeerDeletionRequest struct {
	ID        string            `json:"id"`
	BeerID    string            `json:"beerId"`
	UserID    string            `json:"userId"`
	Reason    DeletionReason    `json:"reason"`
	Details   string            `json:"details"`
	Status    DeletionStatus    `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	ReviewedBy *string          `json:"reviewedBy,omitempty"`
	ReviewedAt *time.Time       `json:"reviewedAt,omitempty"`
}

type BeerReportInput struct {
	Reason      ReportReason `json:"reason" validate:"required,oneof=DUPLICATE INAPPROPRIATE_CONTENT INCORRECT_INFORMATION OUT_OF_PRODUCTION OTHER"`
	Description string       `json:"description" validate:"required,min=10,max=1000"`
}

type BeerDeletionRequestInput struct {
	Reason  DeletionReason `json:"reason" validate:"required,oneof=DUPLICATE INAPPROPRIATE_CONTENT OUT_OF_PRODUCTION OTHER"`
	Details string         `json:"details" validate:"required,min=10,max=1000"`
}

type ReportFilter struct {
	BeerID  string
	UserID  string
	Status  ReportStatus
	Limit   int
	Offset  int
}

type DeletionRequestFilter struct {
	BeerID  string
	UserID  string
	Status  DeletionStatus
	Limit   int
	Offset  int
}
