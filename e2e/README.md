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

Please make sure you have a private and public image in the registry you're using.
For each of the supported container registries (AWS ECR, Docker Hub, Github Registry) 
there is a class that implements the `ContainerRegistry` interface. You can find them in `e2e/pullsecrets/registry.go`.

If you want to add support for a new container registry, you can implement the `ContainerRegistry` interface.
If you want to change the image name used for tests you can modify methods `PublicImageName` and `PrivateImageName`
for the registry you're using.

#### Run a specific e2e test
```sh
# Install ginkgo and run in this folder
ginkgo -focus="should load profile cached and uncached"
```

#### Debugging e2e test
If you need to debug the test, make sure you have `dlv` installed.
Then go to the test folder and run the following command:
```sh
DOCKER_USERNAME=<username> DOCKER_PASSWORD=<password> dlv test . -focus="should create pull secret and make pod use it"
```
Then, when you're inside the debugger, you can set breakpoints:
```sh
break <file>:<line>
# for example:
break main.go:123
# or
break <package>.<function>
# you can also use alias for break:
b <package>.<function>
```
and continue the test:
```sh
continue
# or
c
```
and in the end, exit the debugger:
```sh
q
# or
quit
```
For more information about `dlv` commands, please refer to [dlv documentation](https://github.com/go-delve/delve/tree/master/Documentation/cli)
