package messageService

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikhil/eaven/internal/database.go"
	"github.com/nikhil/eaven/internal/logger"
	"github.com/nikhil/eaven/internal/middleware"
	"github.com/nikhil/eaven/internal/models"
)

type MessageService struct {
	DB  *sql.DB
	Log *logger.Logger
}

func NewMessageService() *MessageService {
	return &MessageService{
		DB:  database.DB,
		Log: logger.NewLogger("message-service"),
	}
}

type sendMessageRequest struct {
	ChannelID int64  `json:"channel_id"`
	Content   string `json:"content"`
}

func (ms *MessageService) SendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userDetails, ok := ctx.Value(middleware.UserContextKey).(jwt.MapClaims)
	if !ok {
		ms.Log.Error("Failed to extract user details from context")
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Extract user ID from token
	userID, err := strconv.ParseInt(fmt.Sprintf("%v", userDetails["user_id"]), 10, 64)
	if err != nil {
		ms.Log.Error("Invalid user ID in token", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var messageBody sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
		ms.Log.Error("Failed to decode request body", "error", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	// ms.Log.Info("User : ", userID, messageBody.ChannelID)
	// fmt.Println(userID, messageBody.ChannelID)
	var channelUserData models.ChannelUserDataStruct
	memberQuery := `SELECT C.channel_id , CM.user_id , T.team_id , U.first_name , U.last_name  , C.channel_name
					FROM channel_members CM
					INNER JOIN channels C on C.channel_id = CM.channel_id
					INNER JOIN teams T on C.team_id = T.team_id
					INNER JOIN users U on U.user_id = CM.user_id
					INNER JOIN user_teams_mapper UTM on UTM.user_id = CM.user_id and UTM.team_id = C.team_id
					WHERE CM.channel_id = ?  and CM.user_id = ?`
	err = ms.DB.QueryRowContext(ctx, memberQuery, messageBody.ChannelID, userID).Scan(&channelUserData.ChannelID, &channelUserData.UserID, &channelUserData.TeamID, &channelUserData.FirstName, &channelUserData.LastName, &channelUserData.ChannelName)
	if err != nil {
		ms.Log.Error("Failed to check channel subscription", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to verify channel subscription")
		return
	}

	if channelUserData.ChannelID == 0 {
		ms.Log.Error("User is not a member of the channel", "error", err)
		respondWithError(w, http.StatusUnauthorized, "User is not a member of the channel")
		return
	}

	currentTime := time.Now().UTC().Unix()

	msg := models.MessageBody{
		ChannelID:   messageBody.ChannelID,
		UserID:      userID,
		Content:     messageBody.Content,
		MessageTime: currentTime,
		TeamID:      channelUserData.TeamID,
	}

	_, err = ms.SaveMessage(ctx, msg)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to insert message")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Message sent successfully"})
}

func (ms *MessageService) SaveMessage(ctx context.Context, messageBody models.MessageBody) (bool, error) {
	// Insert the message into the database
	query := `INSERT INTO messages (channel_id, user_id, content, message_created_at) VALUES (?, ?, ? , ?)`
	_, err := ms.DB.ExecContext(ctx, query, messageBody.ChannelID, messageBody.UserID, messageBody.Content, messageBody.MessageTime)
	if err != nil {
		ms.Log.Error("Failed to insert message", "error", err)
		return false, fmt.Errorf("failed to insert message: %v", err)
	}

	// trigger messages to channel users
	ms.TriggerMessageToChannelUsers(ctx, messageBody)
	return true, nil
}

func (ms *MessageService) TriggerMessageToChannelUsers(ctx context.Context, messageBody models.MessageBody) {
	query := `SELECT CM.user_id FROM channel_members CM WHERE CM.channel_id = ?`
	rows, err := ms.DB.QueryContext(ctx, query, messageBody.ChannelID)
	if err != nil {
		ms.Log.Error("Failed to trigger message to channel users", "error", err)
		return
	}

	var userIDs []int64
	for rows.Next() {
		var userID int64
		err = rows.Scan(&userID)
		if err != nil {
			ms.Log.Error("Failed to scan user ID", "error", err)
			continue
		}
		userIDs = append(userIDs, userID)
	}
	if err = rows.Err(); err != nil {
		ms.Log.Error("Error iterating over rows", "error", err)
		return
	}
	defer rows.Close()

	// Get the global hub instance
	hub := models.GetHub()

	// Create message payload
	messagePayload := models.Message{
		Type:    "message",
		Content: messageBody.Content,
		UserID:  fmt.Sprintf("%d", messageBody.UserID),
		TeamID:  fmt.Sprintf("%d", messageBody.TeamID),
	}

	messageBytes, err := json.Marshal(messagePayload)
	if err != nil {
		ms.Log.Error("Failed to marshal message", "error", err)
		return
	}

	for _, userID := range userIDs {
		// Skip sending to the sender
		// if userID == messageBody.UserID {
		// 	continue
		// }

		userIDStr := fmt.Sprintf("%d", userID)
		teamIDStr := fmt.Sprintf("%d", messageBody.TeamID)

		// Check if user has an active WebSocket connection and send message
		if hub.IsUserConnected(teamIDStr, userIDStr) {
			if !hub.SendMessageToUser(teamIDStr, userIDStr, messageBytes) {
				ms.Log.Error("Failed to send message to connected user", "user_id", userID)
			}
		} else {
			// TODO: Implement push notification logic here
			ms.Log.Info("User is offline, would send push notification", "user_id", userID)
		}
	}
}

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
