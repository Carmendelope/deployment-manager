package cluster_api

import (
	"context"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-authx-go"
	"github.com/nalej/grpc-login-api-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/rs/zerolog/log"
)

type ClusterAPIClient struct {
	ClusterManagerAddress	string
	ClusterAPIPort			int
	LoginAPIPort			int
	Token					string
}

type Login struct {
	Connection
}

func (l *Login) Login(email string, password string) (*Credentials, derrors.Error) {
	c, err := l.GetConnection()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	loginClient := grpc_login_api_go.NewLoginClient(c)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	loginRequest := &grpc_authx_go.LoginWithBasicCredentialsRequest{
		Username: email,
		Password: password,
	}
	response, lErr := loginClient.LoginWithBasicCredentials(ctx, loginRequest)
	if lErr != nil {
		return nil, conversions.ToDerror(lErr)
	}
	log.Debug().Str("token", response.Token).Msg("Login success")
	credentials := NewCredentials(DefaultPath, response.Token, response.RefreshToken)
	sErr := credentials.Store()
	if sErr != nil {
		return nil, sErr
	}
	return credentials, nil
}