package domain

import "time"

type User struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Account            string    `gorm:"column:username;not null" json:"account"`
	Username           string    `gorm:"column:display_name" json:"username"`
	Bio                string    `gorm:"not null;default:''" json:"bio"`
	PasswordHash       string    `gorm:"not null" json:"-"`
	Role               string    `gorm:"not null;default:user" json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
	Email              *string   `gorm:"default:null" json:"-"`
	EmailVerified      bool      `json:"email_verified"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type AuthToken struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	UserID       int64  `gorm:"index:idx_auth_tokens_user_purpose"`
	TokenHash    []byte `gorm:"uniqueIndex"`
	Purpose      string `gorm:"index:idx_auth_tokens_user_purpose"`
	PendingEmail *string
	ExpiresAt    time.Time
	UsedAt       *time.Time
	CreatedAt    time.Time
	User         User `gorm:"constraint:OnDelete:CASCADE"`
}

func (AuthToken) TableName() string { return "auth_tokens" }

type Follow struct {
	FollowerID int64 `gorm:"primaryKey"`
	FollowedID int64 `gorm:"primaryKey"`
	CreatedAt  time.Time
	Follower   User `gorm:"foreignKey:FollowerID;constraint:OnDelete:CASCADE"`
	Followed   User `gorm:"foreignKey:FollowedID;constraint:OnDelete:CASCADE"`
}

func (Follow) TableName() string { return "follows" }

type SystemSettings struct {
	Singleton                 bool `gorm:"primaryKey;default:true"`
	RegistrationEnabled       bool
	EmailConfirmationRequired bool
	PasswordRecoveryEnabled   bool
	PublicURL                 string
	SMTPEnabled               bool
	SMTPHost                  string
	SMTPPort                  int
	SMTPUsername              string
	SMTPPassword              string `json:"-"`
	SMTPFromName              string
	SMTPFromEmail             string
	SMTPEncryption            string
	UpdatedAt                 time.Time
}

func (SystemSettings) TableName() string { return "system_settings" }

type Memo struct {
	ID          int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64        `gorm:"index" json:"-"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Attachments []Attachment `gorm:"foreignKey:MemoID" json:"attachments"`
	User        User         `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (Memo) TableName() string { return "memos" }

type Attachment struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MemoID       int64     `gorm:"index" json:"-"`
	OriginalName string    `json:"original_name"`
	StoredName   string    `gorm:"uniqueIndex" json:"-"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	Memo         *Memo     `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (Attachment) TableName() string { return "attachments" }

type SocialUser struct {
	UID            int64     `json:"uid"`
	Username       string    `json:"username"`
	Bio            string    `json:"bio"`
	CreatedAt      time.Time `json:"created_at"`
	Following      bool      `json:"following"`
	FollowedBy     bool      `json:"followed_by"`
	Friend         bool      `json:"friend"`
	IsSelf         bool      `json:"is_self"`
	FollowingCount int64     `json:"following_count"`
	FollowerCount  int64     `json:"follower_count"`
	FriendCount    int64     `json:"friend_count"`
}

type UserView struct {
	ID                 int64     `json:"id"`
	UID                int64     `json:"uid"`
	Account            string    `json:"account"`
	Username           string    `json:"username"`
	Bio                string    `json:"bio"`
	Role               string    `json:"role"`
	MustChangePassword bool      `json:"must_change_password"`
	Email              string    `json:"email,omitempty"`
	EmailVerified      bool      `json:"email_verified"`
	CreatedAt          time.Time `json:"created_at"`
}

func NewUserView(user User) UserView {
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	return UserView{
		ID: user.ID, UID: user.ID, Account: user.Account, Username: user.Username,
		Bio: user.Bio, Role: user.Role, MustChangePassword: user.MustChangePassword,
		Email: email, EmailVerified: user.EmailVerified, CreatedAt: user.CreatedAt,
	}
}
