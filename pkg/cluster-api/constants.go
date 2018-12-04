package cluster_api

import "time"

const (
	DefaultTimeout = time.Second * 5
	DefaultPath = "~/.nalej/"
	// TokenFileName with the name of the file we use to store the token.
	TokenFileName = "token"
	// RefreshTokenFileName with the name of the file that contains the refresh token
	RefreshTokenFileName = "refresh_token"
	AuthHeader = "Authorization"
)