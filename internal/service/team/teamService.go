package teamService

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"

	// "github.com/nikhil/eaven/internal/cache"
	// "github.com/nikhil/eaven/internal/database"
	"github.com/nikhil/eaven/internal/database.go"
	"github.com/nikhil/eaven/internal/logger"
	"github.com/nikhil/eaven/internal/middleware"
	"github.com/nikhil/eaven/internal/models"
	// "github.com/nikhil/eaven/internal/validator"
)

// TeamService handles team-related operations
type TeamService struct {
	DB *sql.DB
	// Cache cache.CacheInterface
	Log *logger.Logger
}

// CreateTeamRequest represents the request body for team creation
type CreateTeamRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateTeamRequest represents the request body for team updates
type UpdateTeamRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// PaginationResponse wraps paginated team results
type PaginationResponse struct {
	Teams      []models.Team `json:"teams"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
}

// NewTeamService initializes a new team service
func NewTeamService() *TeamService {
	return &TeamService{
		DB: database.DB,
		// Cache: cache.NewRedisCache(),
		Log: logger.NewLogger("team-service"),
	}
}

// CreateTeam handles the creation of a new team
func (ts *TeamService) CreateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		ts.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		ts.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse and validate request body
	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ts.Log.Error("Failed to decode request body", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	// if err := validator.Validate(req); err != nil {
	// 	ts.Log.Error("Validation failed", "error", err)
	// 	respondWithError(w, http.StatusBadRequest, err.Error())
	// 	return
	// }

	// Begin transaction
	tx, err := ts.DB.BeginTx(ctx, nil)
	if err != nil {
		ts.Log.Error("Failed to begin transaction", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback() // Will be ignored if transaction is committed

	// Insert team into database
	currentTime := time.Now().UTC().Unix()
	query := `
		INSERT INTO teams (team_name,  created_by, created_at) 
		VALUES (?, ?, ?)
	`
	result, err := tx.ExecContext(ctx, query, req.Name, userID, currentTime)
	if err != nil {
		ts.Log.Error("Failed to create team", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create team")
		return
	}

	// Get the ID of the newly created team
	teamID, err := result.LastInsertId()
	if err != nil {
		ts.Log.Error("Failed to get team ID", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get team ID")
		return
	}

	// Create team-user relationship (add creator as team owner)
	query = `
		INSERT INTO user_teams_mapper (team_id, user_id, role, joined_at, invited_by) 
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, query, teamID, userID, 1, currentTime, userID)
	if err != nil {
		ts.Log.Error("Failed to add user to team", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to team")
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		ts.Log.Error("Failed to commit transaction", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Return the created team
	newTeam := models.Team{
		ID:        teamID,
		Name:      req.Name,
		CreatedBy: userID,
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}

	// Invalidate cache for this user's teams
	// cacheKey := fmt.Sprintf("user_teams:%d", userID)
	// if err := ts.Cache.Delete(ctx, cacheKey); err != nil {
	// 	ts.Log.Error("Failed to invalidate cache", "error", err, "key", cacheKey)
	// 	// Continue execution despite cache error
	// }

	// Audit log
	ts.Log.Info("Team created", "team_id", teamID, "user_id", userID)

	respondWithJSON(w, http.StatusCreated, newTeam)
}

// GetUserTeams retrieves all teams associated with the current user
func (ts *TeamService) GetUserTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		ts.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		ts.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20 // Default to 20 items per page
	}
	offset := (page - 1) * perPage

	// Try to get from cache first
	// cacheKey := fmt.Sprintf("user_teams:%d:page:%d:per_page:%d", userID, page, perPage)
	var response PaginationResponse

	// if cached, err := ts.Cache.Get(ctx, cacheKey); err == nil {
	// 	if err := json.Unmarshal([]byte(cached), &response); err == nil {
	// 		ts.Log.Info("Teams fetched from cache", "user_id", userID)
	// 		respondWithJSON(w, http.StatusOK, response)
	// 		return
	// 	}
	// }

	// Count total teams for pagination
	var totalCount int
	countQuery := `
		SELECT COUNT(*) 
		FROM teams t
		JOIN user_teams_mapper tm ON t.team_id = tm.team_id
		WHERE tm.user_id = ?
	`
	err = ts.DB.QueryRowContext(ctx, countQuery, userID).Scan(&totalCount)
	if err != nil {
		ts.Log.Error("Failed to count teams", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get teams")
		return
	}

	// Query to get teams with pagination
	query := `
		SELECT t.team_id, t.team_name,  t.created_by, t.created_at
		FROM teams t
		JOIN user_teams_mapper tm ON t.team_id = tm.team_id
		WHERE tm.user_id = ?
		ORDER BY t.created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := ts.DB.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		ts.Log.Error("Failed to query teams", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get teams")
		return
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			ts.Log.Error("Failed to scan team row", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to process teams data")
			return
		}
		teams = append(teams, t)
	}

	if err := rows.Err(); err != nil {
		ts.Log.Error("Error iterating teams rows", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Error processing teams data")
		return
	}

	// Build response
	response = PaginationResponse{
		Teams:      teams,
		TotalCount: totalCount,
		Page:       page,
		PerPage:    perPage,
	}

	// Cache the result (with 5 minute expiry)
	// if data, err := json.Marshal(response); err == nil {
	// 	if err := ts.Cache.Set(ctx, cacheKey, string(data), 5*time.Minute); err != nil {
	// 		ts.Log.Error("Failed to cache teams", "error", err)
	// 		// Continue despite cache error
	// 	}
	// }

	ts.Log.Info("Teams fetched from database", "user_id", userID, "count", len(teams))
	// respondWithJSON(w, http.StatusOK, response)
	json.NewEncoder(w).Encode(response)
}

// GetTeam retrieves a specific team by ID
func (ts *TeamService) GetTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		ts.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["id"]), 10, 64)
	if err != nil {
		ts.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get team ID from URL parameters
	vars := mux.Vars(r)
	teamID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		ts.Log.Error("Invalid team ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Check if user has access to this team
	var membershipExists bool
	memberQuery := `SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = ? AND user_id = ?)`
	err = ts.DB.QueryRowContext(ctx, memberQuery, teamID, userID).Scan(&membershipExists)
	if err != nil {
		ts.Log.Error("Failed to check team membership", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify team access")
		return
	}

	if !membershipExists {
		ts.Log.Warn("Unauthorized team access attempt", "team_id", teamID, "user_id", userID)
		respondWithError(w, http.StatusForbidden, "You don't have access to this team")
		return
	}

	// Try to get from cache
	// cacheKey := fmt.Sprintf("team:%d", teamID)
	var team models.Team

	// if cached, err := ts.Cache.Get(ctx, cacheKey); err == nil {
	// 	if err := json.Unmarshal([]byte(cached), &team); err == nil {
	// 		respondWithJSON(w, http.StatusOK, team)
	// 		return
	// 	}
	// }

	// Get team details
	query := `
		SELECT id, name, description, created_by, created_at, updated_at
		FROM teams WHERE id = ?
	`
	err = ts.DB.QueryRowContext(ctx, query, teamID).Scan(
		&team.ID, &team.Name,
		&team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ts.Log.Warn("Team not found", "team_id", teamID)
			respondWithError(w, http.StatusNotFound, "Team not found")
		} else {
			ts.Log.Error("Failed to query team", "error", err, "team_id", teamID)
			respondWithError(w, http.StatusInternalServerError, "Failed to get team details")
		}
		return
	}

	// Cache the result (with 5 minute expiry)
	// if data, err := json.Marshal(team); err == nil {
	// 	if err := ts.Cache.Set(ctx, cacheKey, string(data), 5*time.Minute); err != nil {
	// 		ts.Log.Error("Failed to cache team", "error", err)
	// 		// Continue despite cache error
	// 	}
	// }

	respondWithJSON(w, http.StatusOK, team)
}

// UpdateTeam updates a team's name and description
func (ts *TeamService) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		ts.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["id"]), 10, 64)
	if err != nil {
		ts.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get team ID from URL parameters
	vars := mux.Vars(r)
	teamID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		ts.Log.Error("Invalid team ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Parse and validate request body
	var req UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ts.Log.Error("Failed to decode request body", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	// if err := validator.Validate(req); err != nil {
	// 	ts.Log.Error("Validation failed", "error", err)
	// 	respondWithError(w, http.StatusBadRequest, err.Error())
	// 	return
	// }

	// Check if user has admin or owner role in the team
	var role string
	roleQuery := `SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`
	err = ts.DB.QueryRowContext(ctx, roleQuery, teamID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ts.Log.Warn("Unauthorized team update attempt", "team_id", teamID, "user_id", userID)
			respondWithError(w, http.StatusForbidden, "You don't have permission to update this team")
		} else {
			ts.Log.Error("Failed to check team permissions", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to check permissions")
		}
		return
	}

	// Only owners and admins can update team details
	if role != "owner" && role != "admin" {
		ts.Log.Warn("Insufficient permissions for team update", "team_id", teamID, "user_id", userID, "role", role)
		respondWithError(w, http.StatusForbidden, "You don't have permission to update this team")
		return
	}

	// Update team details
	currentTime := time.Now().UTC()
	updateQuery := `UPDATE teams SET name = ?, description = ?, updated_at = ? WHERE id = ?`
	result, err := ts.DB.ExecContext(ctx, updateQuery, req.Name, req.Description, currentTime, teamID)
	if err != nil {
		ts.Log.Error("Failed to update team", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update team")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		ts.Log.Error("Failed to get rows affected", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify update")
		return
	}

	if rowsAffected == 0 {
		ts.Log.Warn("Team not found for update", "team_id", teamID)
		respondWithError(w, http.StatusNotFound, "Team not found")
		return
	}

	// Get the updated team
	var updatedTeam models.Team
	query := `
		SELECT id, name, description, created_by, created_at, updated_at
		FROM teams WHERE id = ?
	`
	err = ts.DB.QueryRowContext(ctx, query, teamID).Scan(
		&updatedTeam.ID, &updatedTeam.Name,
		&updatedTeam.CreatedBy, &updatedTeam.CreatedAt, &updatedTeam.UpdatedAt,
	)
	if err != nil {
		ts.Log.Error("Failed to get updated team", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve updated team")
		return
	}

	// Invalidate caches
	// cacheKeys := []string{
	// 	fmt.Sprintf("team:%d", teamID),
	// 	fmt.Sprintf("user_teams:%d", userID),
	// }

	// for _, key := range cacheKeys {
	// 	if err := ts.Cache.Delete(ctx, key); err != nil {
	// 		ts.Log.Error("Failed to invalidate cache", "error", err, "key", key)
	// 		// Continue despite cache error
	// 	}
	// }

	// Log the update
	ts.Log.Info("Team updated", "team_id", teamID, "updated_by", userID)

	respondWithJSON(w, http.StatusOK, updatedTeam)
}

// Helper functions for HTTP responses
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
