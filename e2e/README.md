### Run the e2e tests

Make sure you have ginkgo installed on your local machine:
```sh
go get github.com/onsi/ginkgo/ginkgo
```
Make sure you have docker installed and running on your local machine.
As well as you have access to kubernetes cluster via `kubectl` command.

#### Run all e2e tests
```sh
# Install ginkgo and run in this folder
ginkgo
```

#### Run pull secrets tests
To run this test, you need to provide docker credentials in environment variables:
```sh
DOCKER_USERNAME=<username> DOCKER_PASSWORD=<password> ginkgo -focus="should create pull secret and make pod use it"
```

If you want to use a different registry, you can set the `CONTAINER_REGISTRY` environment variable.
```sh
DOCKER_USERNAME=<username> DOCKER_PASSWORD=<password> CONTAINER_REGISTRY=<registry> \
ginkgo -focus="should create pull secret and make pod use it"
```

#### Run a specific e2e test
```sh
# Install ginkgo and run in this folder
ginkgo -focus="should load profile cached and uncached"
```

