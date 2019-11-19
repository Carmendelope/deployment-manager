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

package config

import (
	"github.com/nalej/deployment-manager/version"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
	"sync"
)

const EnvClusterId = "CLUSTER_ID"

type NetworkType string

const (
	NetworkTypeError = ""
	NetworkTypeZt = "zt"
	NetworkTypeIstio = "istio"
)

func NetworkTypeFromString(net string) (NetworkType, error) {
	switch net {
	case NetworkTypeZt:
		return NetworkTypeZt, nil
	case NetworkTypeIstio:
		return NetworkTypeIstio, nil
	default:
		return NetworkTypeError, derrors.NewInvalidArgumentError("unknown network type")
	}
}

// Configuration structure
type Config struct {
	// Debug is enabled
	Debug bool
	// listening port
	Port uint32
	// port for HTTP metrics endpoint
	MetricsPort uint32
	// ClusterAPIHostname with the hostname of the cluster API on the management cluster
	ClusterAPIHostname string
	// ClusterAPIPort with the port where the cluster API is listening.
	ClusterAPIPort uint32
	// UseTLSForClusterAPI defines if TLS should be used to connect to the cluster API.
	UseTLSForClusterAPI bool
	// LoginHostname with the hostname of the login API on the management cluster.
	LoginHostname string
	// LoginPort with the port where the login API is listening
	LoginPort uint32
	// UseTLSForLogin defines if TLS should be used to connect to the Login API.
	UseTLSForLogin bool
	// ClusterPublicHostname contains the public host where the application cluster can be reached from the outside. Required for the ingresses.
	ClusterPublicHostname string
	// ManagementHostname contains the public host of the root domain of the management cluster.
	ManagementHostname string
	// DeploymentManager address
	DeploymentMgrAddress string
	// is kubernetes locally available
	Local bool
	// Email to log into the management cluster.
	Email string
	// Password to log into the managment cluster.
	Password string
	// List of DNS entries separated by commas
	DNS string
	// TargetPlatformName with the name of the targetPlatform
	TargetPlatformName string
	// TargetPlatform with the target platform enum
	TargetPlatform grpc_installer_go.Platform
	// ClusterId with the cluster identifier.
	ClusterId string
	// nalej-public credentials
	PublicCredentials grpc_application_go.ImageCredentials
	// ZTSidecarPort with the ZT sidecar port listening for route updates
	// TODO change this to 1576
	ZTSidecarPort uint32
	// Path for the certificate of the CA
	CACertPath string
	// Client Cert Path
	ClientCertPath string
	// Skip Server validation
	SkipServerCertValidation bool
	// Network type
	NetworkType NetworkType
}

func (conf *Config) envOrElse(envName string, paramValue string) string {
	if paramValue != "" {
		return paramValue
	}
	fromEnv := os.Getenv(envName)
	if fromEnv != "" {
		return fromEnv
	}
	return ""
}

func (conf *Config) Resolve() derrors.Error {
	conf.ClusterId = conf.envOrElse(EnvClusterId, conf.ClusterId)
	return nil
}

func (conf *Config) Validate() derrors.Error {

	if conf.Port <= 0 {
		return derrors.NewInvalidArgumentError("port must be valid")
	}

	if conf.MetricsPort <= 0 {
		return derrors.NewInvalidArgumentError("metricsPort must be valid")
	}


	if conf.ClusterAPIHostname == "" || conf.ClusterAPIPort <= 0 {
		return derrors.NewInvalidArgumentError("clusterAPIHostname and clusterAPIPort must me set")
	}

	if conf.LoginHostname == "" || conf.LoginPort <= 0 {
		return derrors.NewInvalidArgumentError("loginHostname and loginPort must be set")
	}

	if conf.DeploymentMgrAddress == "" {
		return derrors.NewInvalidArgumentError("depMgrAddress must be set")
	}

	if conf.Email == "" || conf.Password == "" {
		return derrors.NewInvalidArgumentError("email and password must be set")
	}

	if conf.ClusterPublicHostname == "" {
		return derrors.NewInvalidArgumentError("clusterPublicHostname must be set")
	}

	if conf.ManagementHostname == "" {
		return derrors.NewInvalidArgumentError("managementHostname must be set")
	}

	if conf.DNS == "" {
		return derrors.NewInvalidArgumentError("dns list must be set")
	}

	if conf.TargetPlatformName == "" {
		return derrors.NewInvalidArgumentError("targetPlatform must be set")
	}
	conf.TargetPlatform = grpc_installer_go.Platform(grpc_installer_go.Platform_value[conf.TargetPlatformName])

	return nil
}

func (conf *Config) Print() {
	log.Info().Bool("debug", conf.Debug).Msg("Debug")
	log.Info().Str("app", version.AppVersion).Str("commit", version.Commit).Msg("Version")
	log.Info().Uint32("port", conf.Port).Msg("gRPC port")
	log.Info().Uint32("port", conf.MetricsPort).Msg("metrics port")
	log.Info().Str("Id", conf.ClusterId).Msg("Cluster info")
	log.Info().Str("URL", conf.DeploymentMgrAddress).Msg("Deployment manager")
	log.Info().Bool("local", conf.Local).Msg("Kubernetes is local")
	log.Info().Str("URL", conf.ClusterAPIHostname).Uint32("port", conf.ClusterAPIPort).Bool("TLS", conf.UseTLSForClusterAPI).Msg("Cluster API on management cluster")
	log.Info().Str("URL", conf.LoginHostname).Uint32("port", conf.LoginPort).Bool("TLS", conf.UseTLSForLogin).Msg("Login API on management cluster")
	log.Info().Str("URL", conf.ManagementHostname).Msg("Management hostname")
	log.Info().Str("URL", conf.ClusterPublicHostname).Msg("Cluster public hostname")
	log.Info().Str("Email", conf.Email).Str("password", strings.Repeat("*", len(conf.Password))).Msg("Application cluster credentials")
	log.Info().Str("DNS", conf.DNS).Msg("List of DNS ips")
	log.Info().Str("type", conf.TargetPlatform.String()).Msg("Target platform")
	log.Info().Uint32("port", conf.ZTSidecarPort).Msg("ZT sidecar config")
	log.Info().Interface("networkType", conf.NetworkType).Msg("Network type")

}

// appConfig defines the configuration that will be set.
var appConfig *Config

// instance of the configuration to be reused throughout the application.
var instance *Config
var once sync.Once

func SetGlobalConfig(cfg *Config) {
	appConfig = cfg
}

func GetConfig() *Config {
	once.Do(func() {
		instance = appConfig
	})
	return instance
}
