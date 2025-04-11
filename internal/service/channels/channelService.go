package channelService

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

	"github.com/nikhil/eaven/internal/database.go"
	"github.com/nikhil/eaven/internal/logger"
	"github.com/nikhil/eaven/internal/middleware"
	"github.com/nikhil/eaven/internal/models"
	messageService "github.com/nikhil/eaven/internal/service/messages"
)

// ChannelService handles channel-related operations
type ChannelService struct {
	DB  *sql.DB
	Log *logger.Logger
}

// CreateChannelRequest represents the request body for channel creation
type CreateChannelRequest struct {
	TeamID      int64  `json:"team_id" validate:"required"`
	Name        string `json:"name" validate:"required,min=1,max=80"`
	Description string `json:"description" validate:"max=300"`
	IsPrivate   bool   `json:"is_private"`
}

// UpdateChannelRequest represents the request body for channel updates
type UpdateChannelRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=80"`
	Description string `json:"description" validate:"max=300"`
}

// PaginationResponse wraps paginated channel results
type PaginationResponse struct {
	Channels   []models.Channel `json:"channels"`
	TotalCount int              `json:"total_count"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
}

// NewChannelService initializes a new channel service
func NewChannelService() *ChannelService {
	return &ChannelService{
		DB:  database.DB,
		Log: logger.NewLogger("channel-service"),
	}
}

// CreateChannel handles the creation of a new channel
func (cs *ChannelService) CreateChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		cs.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		cs.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse and validate request body
	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		cs.Log.Error("Failed to decode request body", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Verify user is a member of the team
	var isMember bool
	memberQuery := `SELECT 1 FROM user_teams_mapper WHERE team_id = ? AND user_id = ?`
	err = cs.DB.QueryRowContext(ctx, memberQuery, req.TeamID, userID).Scan(&isMember)
	if err != nil {
		cs.Log.Error("Failed to check team membership", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify team membership")
		return
	}

	if !isMember {
		cs.Log.Warn("Unauthorized channel creation attempt", "team_id", req.TeamID, "user_id", userID)
		respondWithError(w, http.StatusForbidden, "You don't have access to this team")
		return
	}

	// Begin transaction
	tx, err := cs.DB.BeginTx(ctx, nil)
	if err != nil {
		cs.Log.Error("Failed to begin transaction", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback() // Will be ignored if transaction is committed

	// Insert channel into database
	currentTime := time.Now().UTC().Unix()
	query := `
		INSERT INTO channels (team_id, channel_name, description, is_private, created_by, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.ExecContext(ctx, query, req.TeamID, req.Name, req.Description, req.IsPrivate, userID, currentTime, currentTime)
	if err != nil {
		cs.Log.Error("Failed to create channel", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create channel")
		return
	}

	// Get the ID of the newly created channel
	channelID, err := result.LastInsertId()
	if err != nil {
		cs.Log.Error("Failed to get channel ID", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get channel ID")
		return
	}

	// Create channel-user relationship (add creator as channel admin)
	query = `
		INSERT INTO channel_members (channel_id, user_id, role, joined_at, invited_by) 
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, query, channelID, userID, 1, currentTime, userID)
	if err != nil {
		cs.Log.Error("Failed to add user as channel admin", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to add user to channel")
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		cs.Log.Error("Failed to commit transaction", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Return the created channel
	newChannel := models.Channel{
		ChannelID:   channelID,
		TeamID:      req.TeamID,
		Name:        req.Name,
		Description: req.Description,
		IsPrivate:   req.IsPrivate,
		CreatedBy:   userID,
		CreatedAt:   currentTime,
		UpdatedAt:   currentTime,
	}

	// Audit log
	cs.Log.Info("Channel created", "channel_id", channelID, "team_id", req.TeamID, "user_id", userID)

	respondWithJSON(w, http.StatusCreated, newChannel)
}

// GetTeamChannels retrieves all channels in a team accessible to the current user
func (cs *ChannelService) GetTeamChannels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		cs.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		cs.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get team ID from URL parameters
	vars := mux.Vars(r)
	teamID, err := strconv.ParseInt(vars["team_id"], 10, 64)
	if err != nil {
		cs.Log.Error("Invalid team ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	// Verify user is a member of the team
	var isMember bool
	memberQuery := `SELECT EXISTS(SELECT 1 FROM user_teams_mapper WHERE team_id = ? AND user_id = ?)`
	err = cs.DB.QueryRowContext(ctx, memberQuery, teamID, userID).Scan(&isMember)
	if err != nil {
		cs.Log.Error("Failed to check team membership", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify team membership")
		return
	}

	if !isMember {
		cs.Log.Warn("Unauthorized channel access attempt", "team_id", teamID, "user_id", userID)
		respondWithError(w, http.StatusForbidden, "You don't have access to this team")
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

	// Count total channels for pagination
	var totalCount int
	countQuery := `
		SELECT COUNT(*) 
		FROM channels c
		WHERE c.team_id = ? AND (
			c.is_private = 0 OR
			EXISTS (SELECT 1 FROM channel_members cm WHERE cm.channel_id = c.id AND cm.user_id = ?)
		)
	`
	err = cs.DB.QueryRowContext(ctx, countQuery, teamID, userID).Scan(&totalCount)
	if err != nil {
		cs.Log.Error("Failed to count channels", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get channels")
		return
	}

	// Query to get channels with pagination
	query := `
		SELECT c.id, c.team_id, c.name, c.description, c.is_private, c.created_by, c.created_at, c.updated_at
		FROM channels c
		WHERE c.team_id = ? AND (
			c.is_private = 0 OR
			EXISTS (SELECT 1 FROM channel_members cm WHERE cm.channel_id = c.id AND cm.user_id = ?)
		)
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := cs.DB.QueryContext(ctx, query, teamID, userID, perPage, offset)
	if err != nil {
		cs.Log.Error("Failed to query channels", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get channels")
		return
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		if err := rows.Scan(&c.ChannelID, &c.TeamID, &c.Name, &c.Description, &c.IsPrivate, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			cs.Log.Error("Failed to scan channel row", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to process channels data")
			return
		}
		channels = append(channels, c)
	}

	if err := rows.Err(); err != nil {
		cs.Log.Error("Error iterating channels rows", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Error processing channels data")
		return
	}

	// Build response
	response := PaginationResponse{
		Channels:   channels,
		TotalCount: totalCount,
		Page:       page,
		PerPage:    perPage,
	}

	cs.Log.Info("Channels fetched from database", "team_id", teamID, "user_id", userID, "count", len(channels))
	respondWithJSON(w, http.StatusOK, response)
}

// GetChannel retrieves a specific channel by ID
func (cs *ChannelService) GetChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		cs.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		cs.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get channel ID from URL parameters
	vars := mux.Vars(r)
	channelID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		cs.Log.Error("Invalid channel ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	// Check if channel exists and user has access
	var channel models.Channel
	var userRole string

	query := `
		SELECT c.channel_id, c.team_id, c.name, c.description, c.is_private, c.created_by, c.created_at, c.updated_at , CM.role
		FROM channels c
		INNER JOIN channel_members CM ON c.channel_id = CM.channel_id
		WHERE c.channel_id = ? AND CM.user_id = ?
	`
	err = cs.DB.QueryRowContext(ctx, query, channelID, userID).Scan(
		&channel.ChannelID, &channel.TeamID, &channel.Name, &channel.Description,
		&channel.IsPrivate, &channel.CreatedBy, &channel.CreatedAt, &channel.UpdatedAt, &userRole,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cs.Log.Warn("Channel not found or access denied", "channel_id", channelID, "user_id", userID)
			respondWithError(w, http.StatusNotFound, "Channel not found or you don't have access")
		} else {
			cs.Log.Error("Failed to get channel details", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve channel details")
		}
		return
	}

	// If no role found and channel is public, user is a viewer
	if errors.Is(err, sql.ErrNoRows) && !channel.IsPrivate {
		userRole = "viewer"
	}

	// Return channel with user's role
	response := struct {
		Channel  models.Channel `json:"channel"`
		UserRole string         `json:"user_role"`
	}{
		Channel:  channel,
		UserRole: userRole,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// UpdateChannel updates a channel's details
func (cs *ChannelService) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		cs.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		cs.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get channel ID from URL parameters
	vars := mux.Vars(r)
	channelID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		cs.Log.Error("Invalid channel ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	// Parse and validate request body
	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		cs.Log.Error("Failed to decode request body", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Check if user has admin role in the channel
	var role int
	roleQuery := `SELECT role FROM channel_members WHERE channel_id = ? AND user_id = ?`
	err = cs.DB.QueryRowContext(ctx, roleQuery, channelID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cs.Log.Warn("Unauthorized channel update attempt", "channel_id", channelID, "user_id", userID)
			respondWithError(w, http.StatusForbidden, "You don't have permission to update this channel")
		} else {
			cs.Log.Error("Failed to check channel permissions", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to check permissions")
		}
		return
	}

	// Only admins can update channel details
	if role != 1 {
		cs.Log.Warn("Insufficient permissions for channel update", "channel_id", channelID, "user_id", userID, "role", role)
		respondWithError(w, http.StatusForbidden, "You don't have permission to update this channel")
		return
	}

	// Update channel details
	currentTime := time.Now().UTC().Unix()
	updateQuery := `UPDATE channels SET name = ?, description = ?, updated_at = ? WHERE channl_id = ?`
	result, err := cs.DB.ExecContext(ctx, updateQuery, req.Name, req.Description, currentTime, channelID)
	if err != nil {
		cs.Log.Error("Failed to update channel", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update channel")
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		cs.Log.Error("Failed to get rows affected", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify update")
		return
	}

	if rowsAffected == 0 {
		cs.Log.Warn("Channel not found for update", "channel_id", channelID)
		respondWithError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Get the updated channel
	var updatedChannel models.Channel
	query := `
		SELECT channel_id, team_id, name, description, is_private, created_by, created_at, updated_at
		FROM channels WHERE id = ?
	`
	err = cs.DB.QueryRowContext(ctx, query, channelID).Scan(
		&updatedChannel.ChannelID, &updatedChannel.TeamID, &updatedChannel.Name, &updatedChannel.Description,
		&updatedChannel.IsPrivate, &updatedChannel.CreatedBy, &updatedChannel.CreatedAt, &updatedChannel.UpdatedAt,
	)
	if err != nil {
		cs.Log.Error("Failed to get updated channel", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve updated channel")
		return
	}

	// Log the update
	cs.Log.Info("Channel updated", "channel_id", channelID, "updated_by", userID)

	respondWithJSON(w, http.StatusOK, updatedChannel)
}

func (cs *ChannelService) SubscribeChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract user details from context
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		cs.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		cs.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get channel ID from URL parameters
	vars := mux.Vars(r)
	channelID, err := strconv.ParseInt(vars["channel_id"], 10, 64)
	if err != nil {
		cs.Log.Error("Invalid channel ID in URL", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	var memeberID int64
	fmt.Println("userID", userID)
	fmt.Println("channelID", channelID)
	sqlQuery := `Select channel_member_id from channel_members where  user_id=? and channel_id = ?`
	err = cs.DB.QueryRowContext(ctx, sqlQuery, userID, channelID).Scan(&memeberID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User is not in the channel, which is fine - we'll add them
		} else {
			// Some other database error occurred
			cs.Log.Error("Database error checking channel membership", "error", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to check channel membership")
			return
		}
	} else {
		// User already exists in the channel
		respondWithError(w, http.StatusConflict, "User is already a member of this channel")
		return
	}
	//check if user already exists in the team , if yes then add or else throw error that user is not present in team
	var channelUserData models.ChannelUserDataStruct
	memberQuery := `SELECT CM.channel_id , UTM.user_id , T.team_id , U.first_name , U.last_name  , CM.channel_name
					FROM channels CM
					INNER JOIN teams T on CM.team_id = T.team_id
					INNER JOIN user_teams_mapper UTM on  UTM.team_id = CM.team_id
					INNER JOIN users U on U.user_id = UTM.user_id
					WHERE CM.channel_id = ? and UTM.user_id = ?`
	err = cs.DB.QueryRowContext(ctx, memberQuery, channelID, userID).Scan(&channelUserData.ChannelID, &channelUserData.UserID, &channelUserData.TeamID, &channelUserData.FirstName, &channelUserData.LastName, &channelUserData.ChannelName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cs.Log.Warn("Unauthorized channel join attempt", "channel_id", channelID, "user_id", userID)
			respondWithError(w, http.StatusForbidden, "You don't have permission to join this channel")
			return
		}
		cs.Log.Error("Failed to check channel membership", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to check membership")
		return
	}
	currentTime := time.Now().UTC().Unix()

	// Subscribe user to channel
	subscribeQuery := `INSERT INTO channel_members (channel_id, user_id ,role, joined_at) VALUES (?,?,?,?)`
	_, err = cs.DB.ExecContext(ctx, subscribeQuery, channelID, userID, 2, currentTime)
	if err != nil {
		cs.Log.Error("Failed to subscribe user to channel", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to subscribe user")
		return
	}

	ms := messageService.NewMessageService()
	msg := models.MessageBody{
		ChannelID:   channelID,
		UserID:      userID,
		Content:     channelUserData.FirstName + " has joined " + channelUserData.ChannelName,
		MessageTime: currentTime,
	}

	_, err = ms.SaveMessage(ctx, msg)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to insert message")
		return
	}

	// Return success response with channel details
	response := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Channel struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"channel"`
	}{
		Status:  "success",
		Message: "User successfully subscribed to channel",
		Channel: struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{
			ID:   channelID,
			Name: channelUserData.ChannelName,
		},
	}

	respondWithJSON(w, http.StatusOK, response)
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
