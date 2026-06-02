# kubectl-doc

`kubectl-doc` renders Kubernetes OpenAPI v3 schemas as YAML-shaped
documentation for terminal, Markdown, and interactive documentation surfaces.

## GitHub Markdown Example

The example below is generated from
[`internal/cli/testdata/crontab-crd.yaml`](internal/cli/testdata/crontab-crd.yaml)
with `kubectl-doc -o markdown-github --all-versions`. The same output is also
checked in as [`docs/examples/github-crontab.md`](docs/examples/github-crontab.md).

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
spec:
  cronSpec: "<string>" # minLength: 1

  image: "<string>"

  # concurrencyPolicy: "Allow" # default, enum: "Forbid" | "Replace"

  # labels:
    # <key>: "<string>"

  ports: # optional
    - containerPort: <int32>

      name: "<string>"

      # protocol: "TCP" # default, enum: "UDP"

  # replicas: 1 # default, minimum: 0

# status: {}
```
</details>

### Field details: stable.example.com/v1

<a id="field-stable-example-com-v1-spec"></a>

#### `spec`

- Type: `object`
- Required: `yes`
- Description: CronTabSpec describes the desired cron job.

<a id="field-stable-example-com-v1-spec-concurrencypolicy"></a>

#### `spec.concurrencyPolicy`

- Type: `string`
- Required: `no`
- Metadata: `default: "Allow"`, `enum: "Allow" | "Forbid" | "Replace"`

<a id="field-stable-example-com-v1-spec-cronspec"></a>

#### `spec.cronSpec`

- Type: `string`
- Required: `yes`
- Description: Cron expression for running the job.
- Metadata: `minLength: 1`

<a id="field-stable-example-com-v1-spec-image"></a>

#### `spec.image`

- Type: `string`
- Required: `yes`
- Description: Container image used by the job.

<a id="field-stable-example-com-v1-spec-labels"></a>

#### `spec.labels`

- Type: `object`
- Required: `no`

<a id="field-stable-example-com-v1-spec-labels-key"></a>

#### `spec.labels.<key>`

- Type: `string`
- Required: `no`

<a id="field-stable-example-com-v1-spec-ports"></a>

#### `spec.ports`

- Type: `array<object>`
- Required: `no`

<a id="field-stable-example-com-v1-spec-ports-containerport"></a>

#### `spec.ports[].containerPort`

- Type: `integer/int32`
- Required: `yes`
- Description: Port exposed by the container.

<a id="field-stable-example-com-v1-spec-ports-name"></a>

#### `spec.ports[].name`

- Type: `string`
- Required: `yes`
- Description: Port name.

<a id="field-stable-example-com-v1-spec-ports-protocol"></a>

#### `spec.ports[].protocol`

- Type: `string`
- Required: `no`
- Metadata: `default: "TCP"`, `enum: "TCP" | "UDP"`

<a id="field-stable-example-com-v1-spec-replicas"></a>

#### `spec.replicas`

- Type: `integer/int32`
- Required: `no`
- Metadata: `default: 1`, `minimum: 0`

<a id="field-stable-example-com-v1-status"></a>

#### `status`

- Type: `object`
- Required: `no`

<a id="field-stable-example-com-v1-status-lastscheduletime"></a>

#### `status.lastScheduleTime`

- Type: `string/date-time`
- Required: `no`



## stable.example.com/v1alpha1

<details open>
<summary>YAML: stable.example.com/v1alpha1</summary>

```yaml
apiVersion: stable.example.com/v1alpha1
kind: CronTab
metadata:
  name: "<name>"
spec:
  cronSpec: "<string>" # minLength: 1

  # image: "<string>"
```
</details>

### Field details: stable.example.com/v1alpha1

<a id="field-stable-example-com-v1alpha1-spec"></a>

#### `spec`

- Type: `object`
- Required: `yes`

<a id="field-stable-example-com-v1alpha1-spec-cronspec"></a>

#### `spec.cronSpec`

- Type: `string`
- Required: `yes`
- Description: Cron expression for running the job.
- Metadata: `minLength: 1`

<a id="field-stable-example-com-v1alpha1-spec-image"></a>

#### `spec.image`

- Type: `string`
- Required: `no`
- Description: Container image used by the job.
<!-- END GENERATED GITHUB MARKDOWN EXAMPLE -->
