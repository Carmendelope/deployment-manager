package service

import (
	"github.com/nalej/deployment-manager/version"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-application-go"
	"github.com/rs/zerolog/log"
	"strings"
)

// Configuration structure
type Config struct {
	// listening port
	Port uint32
	// ClusterAPIAddress address
	// Deprecated: Use clusterAPIHostname and clusterAPIPort instead.
	ClusterAPIAddress string
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
	// cluster runtime environment such as aws/google/azure/...
	ClusterEnvironment string
	// nalej-public credentials
	PublicCredentials grpc_application_go.ImageCredentials
}

func (conf *Config) Validate() derrors.Error {

	if conf.Port <= 0 {
		return derrors.NewInvalidArgumentError("ports must be valid")
	}

	if conf.ClusterAPIHostname == "" || conf.ClusterAPIPort <= 0 {
		return derrors.NewInvalidArgumentError("clusterAPIHostname and clusterAPIPort must me set")
	}

	if conf.LoginHostname == "" || conf.LoginPort <= 0{
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

	if conf.DNS == "" {
		return derrors.NewInvalidArgumentError("dns list must be set")
	}

	return nil
}

func (conf *Config) Print() {
	log.Info().Str("app", version.AppVersion).Str("commit", version.Commit).Msg("Version")
	log.Info().Uint32("port", conf.Port).Msg("gRPC port")
	log.Info().Str("URL", conf.DeploymentMgrAddress).Msg("Deployment manager")
	log.Info().Bool("local", conf.Local).Msg("Kubernetes is local")
	log.Info().Str("URL", conf.ClusterAPIHostname).Uint32("port", conf.ClusterAPIPort).Bool("TLS", conf.UseTLSForClusterAPI).Msg("Cluster API on management cluster")
	log.Info().Str("URL", conf.LoginHostname).Uint32("port", conf.LoginPort).Bool("TLS", conf.UseTLSForLogin).Msg("Login API on management cluster")
	log.Info().Str("URL", conf.ClusterPublicHostname).Msg("Cluster public hostname")
	if conf.ClusterAPIAddress != "" {
		log.Warn().Str("address", conf.ClusterAPIAddress).Msg("Deprecated Cluster API address is set")
	}
	log.Info().Str("Email", conf.Email).Str("password", strings.Repeat("*", len(conf.Password))).Msg("Application cluster credentials")
	log.Info().Str("DNS", conf.DNS).Msg("List of DNS ips")
}
