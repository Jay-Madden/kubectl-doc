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
# Spec defines the desired graph topology and pod-level defaults for each component.
spec:
  # Components are the named graph nodes reconciled into Kubernetes workloads.
  components: # listType: map, listMapKeys: name
    - # Unique component name used for generated workload and service names.
      name: "<string>" # minLength: 1, maxLength: 63

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
          name: "<string>" # minLength: 1

          # Service port number.
          port: <int32> # minimum: 1, maximum: 65535

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
      name: "<string>" # minLength: 1

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
