# kubectl-doc

`kubectl-doc` renders Kubernetes OpenAPI v3 schemas as YAML-shaped
documentation for terminal, Markdown, and interactive documentation surfaces.

## Command Reference

```shell
kubectl doc [resource] [flags]
```

Examples:

```shell
kubectl doc
kubectl doc deployments
kubectl doc deployments -o kro
kubectl doc deployments -o markdown
kubectl doc deployments -o markdown --all-versions
kubectl doc deployments -o html > deployment.html
kubectl doc -w
kubectl doc -f ./crd.yaml
kubectl doc -f ./crd.yaml --version v1
kubectl doc -f ./crd.yaml -o kro --all-versions
```

Resource selectors in cluster mode follow Kubernetes resource lookup syntax,
including plural names, singular names, kinds, short names, and qualified forms
such as `deployments.apps` or `deployments.v1.apps`.

Flags:

| Flag | Description |
| --- | --- |
| `-f, --filename <path>` | Read a local CRD manifest instead of cluster discovery/OpenAPI. |
| `-o, --output <format>` | Select output format. Default: `yaml`. |
| `--nocolor` | Disable color in YAML overview/schema output. |
| `--version <version>` | Select a served CRD version when using `-f`. |
| `--all-versions` | Render all served versions for output formats that support it. |
| `--expand-depth <n>` | Initial static expansion depth for YAML-shaped examples. Default: `2`. |
| `--descriptions=false|required|true` | Control description comments/markers. Default: `true`. |
| `--columns <n>` | Target width for Markdown description wrapping. Default: terminal width, otherwise `80`. |
| `--field-details` | Include Markdown field detail sections. Default: disabled. |
| `-i, --interactive` | Shortcut for `-o tui`. |
| `-w, --web` | Shortcut for `-o browser`; opens the localhost URL on macOS when possible. |

Output formats:

| Format | Status | Description |
| --- | --- | --- |
| `yaml` | implemented | Manifest-shaped, syntactically valid YAML documentation. |
| `kro` | implemented | Kro SimpleSchema-style YAML schema view. |
| `html` | implemented | Self-contained interactive HTML for a selected resource or CRD. |
| `markdown`, `markdown-github` | implemented | GitHub Markdown page with fenced YAML examples. |
| `markdown-fern` | implemented | Fern-compatible Markdown/MDX page. |
| `browser` | implemented | Localhost browser server with discovery navigation and lazy schema loading. |
| `tui` | planned | Interactive terminal view. |

## Generated Examples

- GitHub Pages:
  [`sttts.github.io/kubectl-doc`](https://sttts.github.io/kubectl-doc/)
- GitHub Markdown:
  [`docs/examples/github-dynamographdeployment.md`](docs/examples/github-dynamographdeployment.md)
- Interactive HTML:
  [`docs/examples/html-dynamographdeployment.html`](docs/examples/html-dynamographdeployment.html)
  ([served](https://sttts.github.io/kubectl-doc/examples/html-dynamographdeployment.html))
- Kro SimpleSchema:
  [`docs/examples/kro-dynamographdeployment.yaml`](docs/examples/kro-dynamographdeployment.yaml)
  ([served](https://sttts.github.io/kubectl-doc/examples/kro-dynamographdeployment.yaml))

## GitHub Markdown Example

The example below is generated from
[`internal/cli/testdata/dynamographdeployment-crd.yaml`](internal/cli/testdata/dynamographdeployment-crd.yaml)
with
`kubectl-doc -o markdown-github --all-versions --descriptions=true --expand-depth=4 --columns=100`.
The same output is also checked in as
[`docs/examples/github-dynamographdeployment.md`](docs/examples/github-dynamographdeployment.md).

<!-- BEGIN GENERATED GITHUB MARKDOWN EXAMPLE -->
# DynamoGraphDeployment

| Field | Value |
| --- | --- |
| API Version | `nvidia.com/v1beta1` |
| Kind | `DynamoGraphDeployment` |
| Resource | `dynamographdeployments` |

## YAML

<details open>
<summary>YAML</summary>

```yaml
apiVersion: nvidia.com/v1beta1
kind: DynamoGraphDeployment
metadata:
  name: "<name>"
  namespace: "<namespace>"
# Spec defines the desired graph topology and pod-level defaults for each component.
spec: # required
  # Components are the named graph nodes reconciled into Kubernetes workloads.
  components: # required, listType: map, listMapKeys: name
    - # Unique component name used for generated workload and service names.
      name: "<string>" # required, minLength: 1, maxLength: 63

      # Pod template fragment merged with operator defaults before creating pods.
      # podTemplate: {} # preserveUnknownFields

      # Desired number of pod replicas for the component.
      # replicas: 1 # default, minimum: 0

      # Container resource requests and limits for the component.
      # resources:
        # Resource limits keyed by Kubernetes resource name.
        # limits:
          # <key>: "<string>"

        # Resource requests keyed by Kubernetes resource name.
        # requests:
          # <key>: "<string>"

      # Service ports exposed by this component.
      services: # optional
        - # Service port name.
          name: "<string>" # required, minLength: 1

          # Service port number.
          port: <int32> # required, minimum: 1, maximum: 65535

          # Network protocol for the service port.
          # protocol: "TCP" # default, enum: "UDP"

      # Shared memory size mounted into the component pod.
      # sharedMemorySize: <int-or-string> # intOrString

  # Annotations propagated to generated workloads unless a component overrides them.
  # annotations:
    # <key>: "<string>"

  # Backend framework used when a component does not override the runtime explicitly.
  # backendFramework: "sglang" # default, enum: "vllm" | "trtllm"

  # Environment variables applied to every component unless a component overrides them.
  envs: # optional
    - # Name of the environment variable.
      name: "<string>" # required, minLength: 1

      # Literal value for the environment variable.
      # value: "<string>"

      # Source for the environment variable value.
      valueFrom: # optional
        # Selects a key from a Secret in the same namespace.
        secretKeyRef: {} # optional, show with --expand-depth 5

# Status summarizes the observed deployment state.
# status: {}
```
</details>
<!-- END GENERATED GITHUB MARKDOWN EXAMPLE -->
