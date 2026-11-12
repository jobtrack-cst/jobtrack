package domain

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits in characters, after trimming spaces. Noted in the JT-32 PR
// and used for boundary value tests.
const (
	MaxCompanyNameLength = 150
	MaxDescriptionLength = 2000
	MaxWebsiteLength     = 255
	MaxLocationLength    = 100
)

// VerificationStatus values match jobtrack.employer.v1.VerificationStatus.
type VerificationStatus uint8

const (
	VerificationUnspecified VerificationStatus = iota
	VerificationUnverified
	VerificationVerified
	VerificationRejected
)

// CompanyProfile is one employer's profile, keyed by the employer's user ID.
type CompanyProfile struct {
	EmployerID         string
	CompanyName        string
	Description        string
	Website            string
	Location           string
	VerificationStatus VerificationStatus
	RejectionReason    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ProfileFields are the editable fields, validated and trimmed.
type ProfileFields struct {
	CompanyName string
	Description string
	Website     string
	Location    string
}

// NewProfileFields validates input for both create and update.
func NewProfileFields(companyName, description, website, location string) (ProfileFields, error) {
	f := ProfileFields{
		CompanyName: strings.TrimSpace(companyName),
		Description: strings.TrimSpace(description),
		Website:     strings.TrimSpace(website),
		Location:    strings.TrimSpace(location),
	}
	if n := utf8.RuneCountInString(f.CompanyName); n < 1 || n > MaxCompanyNameLength {
		return ProfileFields{}, ErrInvalidCompanyName
	}
	if utf8.RuneCountInString(f.Description) > MaxDescriptionLength {
		return ProfileFields{}, ErrInvalidDescription
	}
	if err := validateWebsite(f.Website); err != nil {
		return ProfileFields{}, err
	}
	if utf8.RuneCountInString(f.Location) > MaxLocationLength {
		return ProfileFields{}, ErrInvalidLocation
	}
	return f, nil
}

// validateWebsite allows empty, otherwise an absolute http or https URL with a host.
func validateWebsite(website string) error {
	if website == "" {
		return nil
	}
	if utf8.RuneCountInString(website) > MaxWebsiteLength {
		return ErrInvalidWebsite
	}
	u, err := url.Parse(website)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ErrInvalidWebsite
	}
	return nil
}

// NewCompanyProfile starts every profile as UNVERIFIED (verified flag false).
func NewCompanyProfile(employerID string, f ProfileFields, now time.Time) CompanyProfile {
	now = now.UTC()
	return CompanyProfile{
		EmployerID:         employerID,
		CompanyName:        f.CompanyName,
		Description:        f.Description,
		Website:            f.Website,
		Location:           f.Location,
		VerificationStatus: VerificationUnverified,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// WithFields replaces the editable fields. Verification status and reason
// are kept, because only verification events change them (JT-56).
func (p CompanyProfile) WithFields(f ProfileFields, now time.Time) CompanyProfile {
	p.CompanyName = f.CompanyName
	p.Description = f.Description
	p.Website = f.Website
	p.Location = f.Location
	p.UpdatedAt = now.UTC()
	return p
}

// ViewFor hides rejection_reason from everyone except the owner and admins.
func (p CompanyProfile) ViewFor(c Caller) CompanyProfile {
	isOwner := c.Role == RoleEmployer && c.UserID == p.EmployerID
	if !isOwner && c.Role != RoleAdmin {
		p.RejectionReason = ""
	}
	return p
}
