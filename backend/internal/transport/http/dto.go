package http

import (
	"github.com/teamcutter/go-poker/internal/domain/user"
)

type UserDTO struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

func toUserDTO(u *user.User, publicID string) UserDTO {
	return UserDTO{
		ID:        publicID,
		Username:  u.Username,
		FirstName: u.FirstName,
	}
}
