package login_helper

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/nalej/derrors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
)

type Connection struct {
	Hostname string
	Port int
	UseTLS bool
	CACertPath string
	ClientCertPath string
	SkipCAValidation bool
}

// TODO Add caCertPath and skipCAValidation to the configuration.
func NewConnection(hostname string, port int, useTLS bool) *Connection {
	return &Connection{hostname, port, useTLS, "", "", true}
}

func (c *Connection) GetInsecureConnection() (*grpc.ClientConn, derrors.Error) {
	targetAddress := fmt.Sprintf("%s:%d", c.Hostname, c.Port)
	log.Debug().Str("address", targetAddress).Msg("creating connection")
	conn, err := grpc.Dial(targetAddress, grpc.WithInsecure())
	if err != nil {
		return nil, derrors.AsError(err, "cannot create connection with the public api")
	}
	return conn, nil
}

func (c* Connection) GetSecureConnection() (*grpc.ClientConn, derrors.Error){
	rootCAs := x509.NewCertPool()
	tlsConfig := &tls.Config{
		ServerName:   c.Hostname,
	}

	if c.CACertPath != "" {
		log.Debug().Str("caCertPath", c.CACertPath).Msg("loading CA cert")
		caCert, err := ioutil.ReadFile(c.CACertPath)
		if err != nil {
			return nil, derrors.NewInternalError("Error loading CA certificate")
		}
		added := rootCAs.AppendCertsFromPEM(caCert)
		if !added {
			return nil, derrors.NewInternalError("cannot add CA certificate to the pool")
		}
		tlsConfig.RootCAs = rootCAs
	}

	targetAddress := fmt.Sprintf("%s:%d", c.Hostname, c.Port)
	log.Debug().Str("address", targetAddress).Msg("creating connection")

	if c.ClientCertPath != "" {
		log.Debug().Str("clientCertPath", c.ClientCertPath).Msg("loading client certificate")
		clientCert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/tls.crt", c.ClientCertPath),fmt.Sprintf("%s/tls.key", c.ClientCertPath))
		if err != nil {
			log.Error().Str("error", err.Error()).Msg("Error loading client certificate")
			return nil, derrors.NewInternalError("Error loading client certificate")
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
		tlsConfig.BuildNameToCertificate()
	}

	if c.SkipCAValidation {
		tlsConfig.InsecureSkipVerify = true
	}

	creds := credentials.NewTLS(tlsConfig)

	log.Debug().Interface("creds", creds.Info()).Msg("Secure credentials")
	sConn, dErr := grpc.Dial(targetAddress, grpc.WithTransportCredentials(creds))
	if dErr != nil {
		return nil, derrors.AsError(dErr, "cannot create connection with the signup service")
	}
	return sConn, nil
}

func (c *Connection) GetConnection() (*grpc.ClientConn, derrors.Error) {
	if c.UseTLS {
		return c.GetSecureConnection()
	}
	return c.GetInsecureConnection()
}
