package domain

type OfferStatus string

type JobStatus string

type PayloadKind string

const (
	OfferActive    OfferStatus = "ACTIVE"
	OfferPaused    OfferStatus = "PAUSED"
	OfferCancelled OfferStatus = "CANCELLED"
	OfferExpired   OfferStatus = "EXPIRED"
)

const (
	JobRequested     JobStatus = "REQUESTED"
	JobChargeCreated JobStatus = "CHARGE_CREATED"
	JobPaid          JobStatus = "PAID"
	JobDelivered     JobStatus = "DELIVERED"
	JobCancelled     JobStatus = "CANCELLED"
	JobExpired       JobStatus = "EXPIRED"
)

const (
	PayloadRequest     PayloadKind = "request"
	PayloadDeliverable PayloadKind = "deliverable"
	PayloadMessage     PayloadKind = "message"
)
