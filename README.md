# deployment-manager
Deployment manager in charge of controlling application deployments at a cluster level


## Integration tests

The following set of variables have go be set in order to proceed with integration tests.

| Variable  | Example Value | Description |
| ------------- | ------------- |------------- |
| MANAGER_CLUSTER_IP | 127.0.0.1 | IP of the manager cluster in charge of this deployment manager |
| RUN_INTEGRATION_TEST  | true | Run integration tests |
| IT_CONDUCTOR_ADDRESS | localhost:5000 | Address of an available conductor for monitoring |
