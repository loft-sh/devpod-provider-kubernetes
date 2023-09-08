# Kubernetes Provider for DevPod

## Getting started

The provider is available for auto-installation using 

```sh
devpod provider add kubernetes
devpod provider use kubernetes
```

Follow the on-screen instructions to complete the setup.

### Creating your first devpod env with kubernetes

After the initial setup, just use:

```sh
devpod up .
```

You'll need to wait for the pod and environment setup.

## Building new version
To build a new version with specific version, just run a command in the root of the repository:

```sh
RELEASE_VERSION=x.y.z ./hack/build.sh
```
