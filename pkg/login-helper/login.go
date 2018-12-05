package login_helper

import (
	"context"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-authx-go"
	"github.com/nalej/grpc-login-api-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/rs/zerolog/log"
)

type LoginHelper struct {
	Connection
	email      string
	password   string
	Ctx        context.Context
	CancelFunc context.CancelFunc
}

// NewLogin creates a new LoginHelper structure.
func NewLogin(address string, email string, password string) *LoginHelper {
	return &LoginHelper{
		Connection: *NewConnection(address),
		email: email,
		password: password,
	}
}

func (l *LoginHelper) Login() derrors.Error {
	c, err := l.GetConnection()
	if err != nil {
		return err
	}
	defer c.Close()
	loginClient := grpc_login_api_go.NewLoginClient(c)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	loginRequest := &grpc_authx_go.LoginWithBasicCredentialsRequest{
		Username: l.email,
		Password: l.password,
	}
	response, lErr := loginClient.LoginWithBasicCredentials(ctx, loginRequest)
	if lErr != nil {
		return conversions.ToDerror(lErr)
	}
	log.Debug().Str("token", response.Token).Msg("LoginHelper success")
	credentials := NewCredentials(DefaultPath, response.Token, response.RefreshToken)
	sErr := credentials.Store()
	if sErr != nil {
		return sErr
	}

	// create context
	ctx, cFunc :=  credentials.GetContext()
	l.Ctx = ctx
	l.CancelFunc = cFunc
	return nil
}

