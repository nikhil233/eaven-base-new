package models

// Channel represents a channel entity
type Channel struct {
	ChannelID   int64  `json:"channl_id"`
	TeamID      int64  `json:"team_id"`
	Name        string `json:"channel_name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
	CreatedBy   int64  `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ChannelMember represents a channel membership with role
type ChannelMember struct {
	ID        int64  `json:"id"`
	ChannelID int64  `json:"channel_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"` // admin, member
	JoinedAt  int64  `json:"joined_at"`
	InvitedBy int64  `json:"invited_by,omitempty"`
}

type PaginationResponse struct {
	Channels   []Channel `json:"channels"`
	TotalCount int       `json:"total_count"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
}
