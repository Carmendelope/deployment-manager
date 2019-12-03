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

package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/nalej/deployment-manager/pkg/decorators/network/istio"
	"github.com/nalej/deployment-manager/pkg/decorators/network/zerotier"
	"github.com/nalej/deployment-manager/pkg/executor"
	"github.com/nalej/deployment-manager/pkg/offline-policy"
	"github.com/nalej/grpc-unified-logging-go"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nalej/deployment-manager/internal/collect"
	"github.com/nalej/deployment-manager/internal/structures"
	"github.com/nalej/deployment-manager/internal/structures/monitor"

	"github.com/nalej/deployment-manager/pkg/config"
	"github.com/nalej/deployment-manager/pkg/handler"
	"github.com/nalej/deployment-manager/pkg/kubernetes"
	"github.com/nalej/deployment-manager/pkg/kubernetes/events"
	"github.com/nalej/deployment-manager/pkg/login-helper"
	"github.com/nalej/deployment-manager/pkg/metrics/prometheus"
	monitor2 "github.com/nalej/deployment-manager/pkg/monitor"
	"github.com/nalej/deployment-manager/pkg/network"
	"github.com/nalej/deployment-manager/pkg/proxy"
	"github.com/nalej/deployment-manager/pkg/utils"

	"github.com/nalej/derrors"

	pbDeploymentMgr "github.com/nalej/grpc-deployment-manager-go"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type DeploymentManagerService struct {
	// Manager with the logic for incoming requests
	mgr *handler.Manager
	// Manager for networking services
	net *network.Manager
	// Manager for collecting metrics for metrics endpoint
	collect *collect.Manager
	// Proxy manager for proxy forwarding
	netProxy *proxy.Manager
	// Offline Policy Manager
	offlinePolicy *offline_policy.Manager
	// configuration
	configuration config.Config
}

func getClusterAPIConnection(hostname string, port int, caCertPath string, clientCertPath string, skipCAValidation bool) (*grpc.ClientConn, derrors.Error) {
	// Build connection with cluster API
	rootCAs := x509.NewCertPool()
	tlsConfig := &tls.Config{
		ServerName: hostname,
	}

	if caCertPath != "" {
		log.Debug().Str("caCertPath", caCertPath).Msg("loading CA cert")
		caCert, err := ioutil.ReadFile(caCertPath)
		if err != nil {
			return nil, derrors.NewInternalError("Error loading CA certificate")
		}
		added := rootCAs.AppendCertsFromPEM(caCert)
		if !added {
			return nil, derrors.NewInternalError("cannot add CA certificate to the pool")
		}
		tlsConfig.RootCAs = rootCAs
	}

	targetAddress := fmt.Sprintf("%s:%d", hostname, port)
	log.Debug().Str("address", targetAddress).Msg("creating cluster API connection")

	if clientCertPath != "" {
		log.Debug().Str("clientCertPath", clientCertPath).Msg("loading client certificate")
		clientCert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/tls.crt", clientCertPath), fmt.Sprintf("%s/tls.key", clientCertPath))
		if err != nil {
			log.Error().Str("error", err.Error()).Msg("Error loading client certificate")
			return nil, derrors.NewInternalError("Error loading client certificate")
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
		tlsConfig.BuildNameToCertificate()
	}

	if skipCAValidation {
		tlsConfig.InsecureSkipVerify = true
	}

	creds := credentials.NewTLS(tlsConfig)

	log.Debug().Interface("creds", creds.Info()).Msg("Secure credentials")
	sConn, dErr := grpc.Dial(targetAddress, grpc.WithTransportCredentials(creds))
	if dErr != nil {
		return nil, derrors.AsError(dErr, "cannot create connection with the cluster API service")
	}
	return sConn, nil
}

func NewDeploymentManagerService(cfg *config.Config) (*DeploymentManagerService, error) {

	rErr := cfg.Resolve()
	if rErr != nil {
		log.Fatal().Str("trace", rErr.DebugReport()).Msg("cannot resolve variables")
	}

	vErr := cfg.Validate()
	if vErr != nil {
		log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
	}

	cfg.Print()
	config.SetGlobalConfig(cfg)

	// login
	clusterAPILoginHelper := login_helper.NewLogin(cfg.LoginHostname, int(cfg.LoginPort), cfg.UseTLSForLogin, cfg.Email, cfg.Password, cfg.CACertPath, cfg.ClientCertPath, cfg.SkipServerCertValidation)
	err := clusterAPILoginHelper.Login()
	if err != nil {
		log.Panic().Err(err).Msg("there was an error requesting cluster-api login")
		panic(err.Error())
		return nil, err
	}

	// Build connection with conductor
	log.Debug().Str("hostname", cfg.ClusterAPIHostname).Msg("connecting with cluster api")
	clusterAPIConn, errCond := getClusterAPIConnection(cfg.ClusterAPIHostname, int(cfg.ClusterAPIPort), cfg.CACertPath, cfg.ClientCertPath, cfg.SkipServerCertValidation)
	if errCond != nil {
		log.Panic().Err(err).Str("hostname", cfg.ClusterAPIHostname).Msg("impossible to connect with cluster api")
		panic(err.Error())
		return nil, errCond
	}

	log.Info().Msg("instantiate memory based instances monitor structure...")
	instanceMonitor := monitor.NewMemoryMonitoredInstances()
	log.Info().Msg("done")

	log.Info().Msg("start monitor helper service...")
	monitorService := monitor2.NewMonitorHelper(clusterAPIConn, clusterAPILoginHelper, instanceMonitor)
	go monitorService.Run()
	log.Info().Msg("done")

	// Create Kubernetes Event provider
	// Only get events relevant for user applications
	labelSelector := utils.NALEJ_ANNOTATION_ORGANIZATION_ID
	kubernetesEvents, derr := events.NewEventsProvider(kubernetes.KubeConfigPath(), cfg.Local, labelSelector)
	if derr != nil {
		return nil, derr
	}

	// Create the Kubernetes event handler
	controller := kubernetes.NewKubernetesController(instanceMonitor)
	deploymentDispatcher, derr := events.NewDispatcher(controller)
	if derr != nil {
		return nil, derr
	}

	// Add dispatcher to provider
	derr = kubernetesEvents.AddDispatcher(deploymentDispatcher)
	if derr != nil {
		return nil, derr
	}

	// Create metrics endpoint provider
	promMetrics, derr := prometheus.NewMetricsProvider()
	if derr != nil {
		return nil, derr
	}
	collector := promMetrics.GetCollector()

	// Create the dispatcher to handle metrics
	metricsTranslator := kubernetes.NewMetricsTranslator(collector)
	metricsDispatcher, derr := events.NewDispatcher(metricsTranslator)
	if derr != nil {
		return nil, derr
	}

	// Add dispatcher to provider
	derr = kubernetesEvents.AddDispatcher(metricsDispatcher)
	if derr != nil {
		return nil, derr
	}

	// Start collecting events
	derr = kubernetesEvents.Start()
	if derr != nil {
		return nil, derr
	}

	collectManager, derr := collect.NewManager(promMetrics, collector)
	if derr != nil {
		return nil, derr
	}

	// Create the Kubernetes executor
	exec, kubErr := kubernetes.NewKubernetesExecutor(cfg.Local, controller)
	if kubErr != nil {
		log.Panic().Err(err).Msg("there was an error creating kubernetes client")
		panic(err.Error())
		return nil, kubErr
	}

	nalejDNSForPods := strings.Split(cfg.DNS, ",")
	nalejDNSForPods = append(nalejDNSForPods, "8.8.8.8")

	// Instantiate a memory queue for requests
	requestsQueue := structures.NewMemoryRequestQueue()

	// Build the network decorator according to config info
	networkDecorator, errNetworkDecorator := getNetworkDecorator(cfg)
	if errNetworkDecorator != nil {
		log.Panic().Err(errNetworkDecorator).Msg("impossible to build network decorator")
		return nil, errNetworkDecorator
	}

	// Instantiate deployment manager service
	log.Info().Msg("star deployment requests manager")

	ulConn, ulErr := grpc.Dial(cfg.UnifiedLoggingAddress, grpc.WithInsecure())
	if ulErr != nil {
		return nil, derrors.AsError(ulErr, "cannot create connection with unified slave coordinator")
	}
	ulClient := grpc_unified_logging_go.NewSlaveClient(ulConn)

	// Instantiate network manager service
	k8sClient, err := kubernetes.GetKubernetesClient(cfg.Local)
	if err != nil {
		return nil, err
	}

	mgr := handler.NewManager(&exec, cfg.ClusterPublicHostname, requestsQueue, nalejDNSForPods, instanceMonitor,
		cfg.PublicCredentials, networkDecorator, ulClient, k8sClient)
	go mgr.Run()
	log.Info().Msg("done")

	net := network.NewManager(clusterAPIConn, clusterAPILoginHelper, k8sClient)

	// Instantiate app network manager service
	netProxy := proxy.NewManager(clusterAPIConn, clusterAPILoginHelper)

	// Instantiate offline policy service
	offlinePolicy := offline_policy.NewManager()

	instance := &DeploymentManagerService{
		mgr:           mgr,
		net:           net,
		collect:       collectManager,
		netProxy:      netProxy,
		offlinePolicy: offlinePolicy,
		configuration: *cfg,
	}

	return instance, nil
}

func (d *DeploymentManagerService) Run() {
	// Channel to signal errors from starting the servers
	errChan := make(chan error, 1)

	// Listen on metrics port
	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", d.configuration.MetricsPort))
	if err != nil {
		log.Fatal().Err(err).Uint32("port", d.configuration.MetricsPort).Msg("failed to listen on port")
	}

	// Start listening on API port
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", d.configuration.Port))
	if err != nil {
		log.Fatal().Err(err).Uint32("port", d.configuration.Port).Msg("failed to listen on port")
	}

	httpServer, derr := d.startMetrics(httpListener, errChan)
	if derr != nil {
		log.Fatal().Err(derr).Str("err", derr.DebugReport()).Msg("failed to start metrics server")
	}
	defer httpServer.Shutdown(context.TODO()) // Add timeout in context

	grpcServer, derr := d.startGRPC(grpcListener, errChan)
	if derr != nil {
		log.Fatal().Err(derr).Str("err", derr.DebugReport()).Msg("failed to start gRPC server")
	}
	defer grpcServer.GracefulStop()

	// Wait for termination signal
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)

	select {
	case sig := <-sigterm:
		log.Info().Str("signal", sig.String()).Msg("Gracefully shutting down")
	case err := <-errChan:
		if err != nil {
			log.Fatal().Err(err).Msg("error running server")
		}
	}
}

func (d *DeploymentManagerService) startGRPC(grpcListener net.Listener, errChan chan<- error) (*grpc.Server, derrors.Error) {
	// Create handlers
	deployment := handler.NewHandler(d.mgr)
	network := network.NewHandler(d.net)
	netProxy := proxy.NewHandler(d.netProxy)
	offlinePolicy := offline_policy.NewHandler(d.offlinePolicy)

	// Register handlers with server
	grpcServer := grpc.NewServer()
	pbDeploymentMgr.RegisterDeploymentManagerServer(grpcServer, deployment)
	pbDeploymentMgr.RegisterDeploymentManagerNetworkServer(grpcServer, network)
	pbDeploymentMgr.RegisterApplicationProxyServer(grpcServer, netProxy)
	pbDeploymentMgr.RegisterOfflinePolicyServer(grpcServer, offlinePolicy)

	if d.configuration.Debug {
		reflection.Register(grpcServer)
	}

	// Run
	log.Info().Uint32("port", d.configuration.Port).Msg("Launching gRPC server")
	go func() {
		err := grpcServer.Serve(grpcListener)
		if err != nil {
			log.Error().Err(err).Msg("failed to serve grpc")
		}
		errChan <- err
		log.Info().Msg("closed grpc server")
	}()

	return grpcServer, nil
}

func (d *DeploymentManagerService) startMetrics(httpListener net.Listener, errChan chan<- error) (*http.Server, derrors.Error) {
	// Create handler
	handler, derr := collect.NewHandler(d.collect)
	if derr != nil {
		return nil, derr
	}

	// Create server with metrics handler
	httpServer := &http.Server{
		Handler: handler,
	}

	// Start manager
	derr = d.collect.Start()
	if derr != nil {
		return nil, derr
	}

	// Start HTTP server
	log.Info().Uint32("port", d.configuration.MetricsPort).Msg("Launching HTTP server")
	go func() {
		err := httpServer.Serve(httpListener)
		if err == http.ErrServerClosed {
			log.Info().Err(err).Msg("closed http server")
		} else if err != nil {
			log.Error().Err(err).Msg("failed to serve http")
		}
		errChan <- err
	}()

	return httpServer, nil
}


func getNetworkDecorator(configuration *config.Config) (executor.NetworkDecorator, derrors.Error) {
	switch configuration.NetworkType {
	case config.NetworkTypeZt:
		return zerotier.NewZerotierDecorator(), nil
	case config.NetworkTypeIstio:
		decorator, err := istio.NewIstioDecorator()
		if err != nil {
			return nil, err
		}
		return decorator, nil
	}
	return nil, derrors.NewInvalidArgumentError("unknown decorator type")
}