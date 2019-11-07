/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	Hostname         string
	Port             int
	UseTLS           bool
	CACertPath       string
	ClientCertPath   string
	SkipCAValidation bool
}

func NewConnection(hostname string, port int, useTLS bool, caCertPath string, clientCertPath string, skipCAValidation bool) *Connection {
	return &Connection{hostname, port, useTLS, caCertPath, clientCertPath, skipCAValidation}
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

func (c *Connection) GetSecureConnection() (*grpc.ClientConn, derrors.Error) {
	rootCAs := x509.NewCertPool()
	tlsConfig := &tls.Config{
		ServerName: c.Hostname,
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
		clientCert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/tls.crt", c.ClientCertPath), fmt.Sprintf("%s/tls.key", c.ClientCertPath))
		if err != nil {
			log.Error().Str("error", err.Error()).Msg("Error loading client certificate")
			return nil, derrors.NewInternalError("Error loading client certificate")
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
		tlsConfig.BuildNameToCertificate()
	}
	log.Debug().Str("address", targetAddress).Bool("useTLS", c.UseTLS).Str("caCertPath", c.CACertPath).Bool("skipServerCertValidation", c.SkipCAValidation).Msg("creating secure connection")

	if c.SkipCAValidation {
		log.Debug().Msg("skipping server cert validation")
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

// printRelevantTLSConfig prints some relevant information from a TLS Config structure, namely:
// ClientAuth, ServerName. RootCAs, Certificates and InsecureSkipVerify
func printRelevantTLSConfig(c *tls.Config) {
	if int(c.ClientAuth) != 0 {
		log.Debug().Int("ClientAuth", int(c.ClientAuth)).Msg("client auth")
	}
	if c.ServerName != "" {
		log.Debug().Str("ServerName", c.ServerName).Msg("server name")
	}
	if c.RootCAs != nil {
		log.Debug().Interface("RootCAs", c.RootCAs).Msg("root cas")
	}
	if c.Certificates != nil {
		log.Debug().Interface("Certificates", c.Certificates).Msg("certificates")
	}
	log.Debug().Bool("InsecureSkipVerify", c.InsecureSkipVerify).Msg("insecure skip verify")
}
