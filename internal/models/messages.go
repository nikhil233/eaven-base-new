package models

type MessageBody struct {
	ChannelID   int64  `json:"channel_id"`
	UserID      int64  `json:"user_id"`
	Content     string `json:"content"`
	MessageTime int64  `json:"message_created_at"`
}
