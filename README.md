# deployment-manager

Deployment manager is in charge of controlling application deployments at a cluster level.

## Getting Started

This component will be deployed on the app cluster and will receive requests related to deploying or undeploying of user applications (mainly from `conductor`). It will also send requests to the `network-manager` regarding network creation, member authorization to join a network, etc.
Every communication with the management cluster will be done, as usual, via `cluster-api`.

### Prerequisites

* [login-api](https://github.com/nalej/login-api)
* [cluster-api](https://github.com/nalej/cluster-api)

### Build and compile

In order to build and compile this repository use the provided Makefile:

```
make all
```

This operation generates the binaries for this repo, downloads the required dependencies, runs existing tests and generates ready-to-deploy Kubernetes files.


### Run tests

Tests are executed using Ginkgo. To run all the available tests:

```
make test
```

### Update dependencies

Dependencies are managed using Godep. For an automatic dependencies download use:

```
make dep
```

In order to have all dependencies up-to-date run:

```
dep ensure -update -v
```


## User client interface

For testing reasons, it is possible to manually undeploy an application using the `deployment-manager-cli`, with the command:

```
deployment-manager-cli undeploy --orgId <organization id> --appId <app instance id>
```

### Optional flag:
`--server`: address where the component is deployed (`localhost:5200` by default.)


## Contributing

Please read [contributing.md](contributing.md) for details on our code of conduct, and the process for submitting pull requests to us.


## Versioning

We use [SemVer](http://semver.org/) for versioning. For the available versions, see the [tags on this repository](https://github.com/nalej/deployment-manager/tags). 

## Authors

See also the list of [contributors](https://github.com/nalej/deployment-manager/contributors) who participated in this project.

## License
This project is licensed under the Apache 2.0 License - see the [LICENSE-2.0.txt](LICENSE-2.0.txt) file for details.
