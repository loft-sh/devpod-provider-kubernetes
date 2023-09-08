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


## Testing locally
1. Build the new version in a dev mode with some version tag (e.g. 0.0.1-dev)
```sh
chmod +x ./hack/build.sh
RELEASE_VERSION=0.0.1-dev ./hack/build.sh --dev
```
2. Remove the old provider from your devpod installation (make sure you delete all workspaces using the provider).
```sh
devpod provider delete kubernetes
```
3. Install the new provider from the local build
```sh
devpod provider add ./release/provider.yaml --name kubernetes
```
4. Test your provider, e.g. with `devpod up` command. Make sure you have a valid kubeconfig file in your home directory.
```sh
devpod up <repository-url> --provider kubernetes --debug 
```