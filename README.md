# Kubernetes Provider for DevPod

[![Join us on Slack!](docs/static/media/slack.svg)](https://slack.loft.sh/) [![Open in DevPod!](https://devpod.sh/assets/open-in-devpod.svg)](https://devpod.sh/open#https://github.com/loft-sh/devpod-provider-kubernetes)

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

### Configuration Options

#### Target Architecture

You can specify the target architecture directly to avoid architecture detection pods and potential scheduling conflicts:

```sh
# Set target architecture via environment variable
export TARGET_ARCHITECTURE=amd64  # or arm64
devpod up .

# Or via provider configuration
devpod provider set-options kubernetes -o targetArchitecture=amd64
```

This is especially useful when:
- You know the required architecture in advance
- You want to avoid potential scheduling conflicts between architecture detection pods and devpods
- You need to ensure pods are scheduled on nodes with specific architectures


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
devpod provider add --name kubernetes --use ./release/provider.yaml 
```
4. Test your provider, e.g. with `devpod up` command. Make sure you have a valid kubeconfig file in your home directory.
```sh
devpod up <repository-url> --provider kubernetes --debug 
```
