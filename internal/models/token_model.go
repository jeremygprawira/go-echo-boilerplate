package models

var (
	TYPE_ACCESS_TOKEN  = "accessToken"
	TYPE_REFRESH_TOKEN = "refreshToken"
)

type Token struct {
	Type      string `json:"type" example:"accessToken"`
	Token     string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiredIn int    `json:"expiredIn" example:"300"`
}
