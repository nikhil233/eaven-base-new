package services

import (
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikhil/eaven/internal/database.go"
	models "github.com/nikhil/eaven/internal/models"
	"github.com/nikhil/eaven/pkg/utils"
)

type AuthService struct {
	DB *sql.DB
}

// NewAuthService creates a new instance of AuthService
func NewAuthService() *AuthService {
	return &AuthService{
		DB: database.DB,
	}
}

// Signup handles user registration
func (s *AuthService) Signup(user models.User) (int64, error) {
	hashedPassword, err := utils.HashPassword(user.Password)
	if err != nil {
		return 0, err
	}
	var existingUserID int
	userquery := "SELECT user_id FROM users WHERE email = ?"
	err = s.DB.QueryRow(userquery, user.Email).Scan(&existingUserID)

	if err == nil {
		return 0, errors.New("Email already registered")
	}

	query := "INSERT INTO users (email, password , contact_number , first_name , last_name , created_at	) VALUES (?, ? , ? , ? , ? , ?)"
	value, err := s.DB.Exec(query, user.Email, hashedPassword, user.ContactNumber, user.FirstName, user.LastName, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	id, err := value.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Login authenticates a user
func (s *AuthService) Login(email, password string) (string, models.User, error) {
	var user models.User
	query := "SELECT user_id, email, password , contact_number , first_name , last_name FROM users WHERE email = ?"
	err := s.DB.QueryRow(query, email).Scan(&user.UserID, &user.Email, &user.Password, &user.ContactNumber, &user.FirstName, &user.LastName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", models.User{}, errors.New("user not found")
		}
		return "", models.User{}, err
	}
	if err := utils.CheckPassword(user.Password, password); err != nil {
		return "", models.User{}, err
	}

	token, err := s.GenerateJWT(user.Email, user.UserID)
	user.Password = ""
	if err != nil {
		return "", models.User{}, err
	}

	return token, user, nil
}

// GenerateJWT creates a JWT token for authentication
func (s *AuthService) GenerateJWT(email string, userID int64) (string, error) {
	secretKey := os.Getenv("JWT_SECRET")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email":   email,
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(secretKey))
}
