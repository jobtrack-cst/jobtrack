package domain

import (
	"errors"
	"time"
)

type OfferStatus uint8

const (
	OfferStatusUnspecified OfferStatus = iota
	OfferStatusPending
	OfferStatusAccepted
	OfferStatusDeclined
)

var ErrOfferNotPending = errors.New("offer is not pending")

type Offer struct {
	ID            string
	ApplicationID string
	EmployerID    string
	ApplicantID   string
	Terms         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Status        OfferStatus
}

func (o *Offer) Accept() error {
	return o.decide(OfferStatusAccepted)
}

func (o *Offer) Decline() error {
	return o.decide(OfferStatusDeclined)
}

func (o *Offer) decide(next OfferStatus) error {
	if o.Status != OfferStatusPending {
		return ErrOfferNotPending
	}

	o.Status = next
	return nil
}
