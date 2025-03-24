package models

type User struct {
	UserID        int64  `json:"user_id"`
	Email         string `json:"email"`
	Password      string `json:"password,omitempty"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	ContactNumber string `json:"contact_number"`
}
