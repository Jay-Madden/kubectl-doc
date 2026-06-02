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
| `-w, --web` | Shortcut for `-o browser`. |

Output formats:

| Format | Status | Description |
| --- | --- | --- |
| `yaml` | implemented | Manifest-shaped, syntactically valid YAML documentation. |
| `kro` | implemented | Kro SimpleSchema-style YAML schema view. |
| `markdown`, `markdown-github` | implemented | GitHub Markdown page with fenced YAML examples. |
| `markdown-fern` | implemented | Fern-compatible Markdown/MDX page. |
| `tui` | planned | Interactive terminal view. |
| `browser` | planned | Localhost browser view. |

## GitHub Markdown Example

The example below is generated from
[`internal/cli/testdata/crontab-crd.yaml`](internal/cli/testdata/crontab-crd.yaml)
with
`kubectl-doc -o markdown-github --all-versions --descriptions=true --columns=100`.
The same output is also checked in as
[`docs/examples/github-crontab.md`](docs/examples/github-crontab.md).

<!-- BEGIN GENERATED GITHUB MARKDOWN EXAMPLE -->
# CronTab

| Field | Value |
| --- | --- |
| Kind | `CronTab` |
| Versions | `stable.example.com/v1`, `stable.example.com/v1alpha1` |
| Resource | `crontabs` |

## stable.example.com/v1

<details open>
<summary>YAML: stable.example.com/v1</summary>

```yaml
apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: "<name>"
# CronTabSpec describes the desired cron job.
spec:
  # Cron expression for running the job.
  cronSpec: "<string>" # minLength: 1

  # Container image used by the job.
  image: "<string>"

  # concurrencyPolicy: "Allow" # default, enum: "Forbid" | "Replace"

  # labels:
    # <key>: "<string>"

  ports: # optional
    - # Port exposed by the container.
      containerPort: <int32>

      # Port name.
      name: "<string>"

      # protocol: "TCP" # default, enum: "UDP"

  # replicas: 1 # default, minimum: 0

# status: {}
```
</details>


## stable.example.com/v1alpha1

<details open>
<summary>YAML: stable.example.com/v1alpha1</summary>

```yaml
apiVersion: stable.example.com/v1alpha1
kind: CronTab
metadata:
  name: "<name>"
spec:
  # Cron expression for running the job.
  cronSpec: "<string>" # minLength: 1

  # Container image used by the job.
  # image: "<string>"
```
</details>
<!-- END GENERATED GITHUB MARKDOWN EXAMPLE -->
