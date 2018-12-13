package login_helper

import (
	"context"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-authx-go"
	"github.com/nalej/grpc-login-api-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
)

type LoginHelper struct {
	Connection
	useTLS bool
	email      string
	password   string
	Credentials *Credentials
}

// NewLogin creates a new LoginHelper structure.
func NewLogin(hostname string, port int, useTLS bool, email string, password string) *LoginHelper {
	return &LoginHelper{
		Connection: *NewConnection(hostname, port, useTLS),
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
	// log.Debug().Str("token", response.Token).Msg("LoginHelper success")
	l.Credentials = NewCredentials(DefaultPath, response.Token, response.RefreshToken)
	sErr := l.Credentials.Store()
	if sErr != nil {
		return sErr
	}

	return nil
}

func (l *LoginHelper) GetContext() (context.Context, context.CancelFunc) {
	return l.Credentials.GetContext()
}

