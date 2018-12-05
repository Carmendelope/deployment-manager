package login_helper

import (
	"fmt"
	"github.com/nalej/derrors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type Connection struct {
	Address string
}

func NewConnection(address string) *Connection {
	return &Connection{address}
}

func (c *Connection) GetConnection() (*grpc.ClientConn, derrors.Error) {
	targetAddress := fmt.Sprintf("%s", c.Address)
	log.Debug().Str("address", targetAddress).Msg("creating connection")
	conn, err := grpc.Dial(targetAddress, grpc.WithInsecure())
	if err != nil {
		return nil, derrors.AsError(err, "cannot create connection with the public api")
	}
	return conn, nil
}