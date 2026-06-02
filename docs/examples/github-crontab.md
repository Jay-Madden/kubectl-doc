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
