package teammodels

// Team represents a team entity
type Team struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// TeamMember represents a team membership with role
type TeamMember struct {
	ID        int64  `json:"id"`
	TeamID    int64  `json:"team_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"`
	JoinedAt  int64  `json:"joined_at"`
	InvitedBy int64  `json:"invited_by,omitempty"`
}
