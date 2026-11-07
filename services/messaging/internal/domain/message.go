package domain

import (
	"strconv"
	"time"
)

type Message struct {
	ID, ApplicationID, SenderID, RecipientID, Content string
	CreatedAt, UpdatedAt                              time.Time
}
type Page struct {
	Size   int
	Offset int64
}

func ParsePage(size int32, token string) (Page, error) {
	if size < 0 {
		return Page{}, ErrInvalidPagination
	}
	if size == 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	var offset int64
	if token != "" {
		for _, c := range token {
			if c < '0' || c > '9' {
				return Page{}, ErrInvalidPagination
			}
		}
		var err error
		offset, err = strconv.ParseInt(token, 10, 64)
		if err != nil {
			return Page{}, ErrInvalidPagination
		}
	}
	return Page{int(size), offset}, nil
}
func (p ApplicationParticipants) Recipient(c Caller) (string, error) {
	if c.Role == "applicant" && c.UserID == p.ApplicantID {
		return p.EmployerID, nil
	}
	if c.Role == "employer" && c.UserID == p.EmployerID {
		return p.ApplicantID, nil
	}
	return "", ErrPermissionDenied
}
