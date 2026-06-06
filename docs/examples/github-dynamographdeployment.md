# DynamoGraphDeployment

| Field | Value |
| --- | --- |
| Kind | `DynamoGraphDeployment` |
| Versions | `nvidia.com/v1beta1`, `nvidia.com/v1alpha1` |
| Resource | `dynamographdeployments` |

## nvidia.com/v1beta1

<details open>
<summary>YAML: nvidia.com/v1beta1</summary>

```yaml
# DynamoGraphDeployment is the Schema for the dynamographdeployments API.
#
# v1beta1 is a served version: the API server accepts reads and writes against it, and transparently
# converts to/from v1alpha1 (still the storage version until a later MR flips it). Conversion goes
# through the operator's conversion webhook; see api/v1alpha1/*_conversion.go.
apiVersion: nvidia.com/v1beta1
kind: DynamoGraphDeployment
metadata:
  name: "<name>"
  namespace: "<namespace>"
# spec defines the desired state for this graph deployment.
spec: # optional
  # annotations to propagate to all child resources (PCS, DCD, Deployments, and pod templates).
  # Component-level (`podTemplate`) values take precedence on conflict.
  # annotations:
    # <key>: "<string>"

  # backendFramework specifies the backend framework (e.g. "sglang", "vllm", "trtllm").
  # backendFramework: "sglang" # enum: "vllm" | "trtllm"

  # components are the components deployed as part of this graph. Each entry carries its own stable
  # logical `name`, and names must be unique within the list. Component types are generally
  # repeatable, except `type: epp` which may appear at most once.
  components: # optional, listType: map, listMapKeys: name
    - # name is the stable logical identifier for this component within its DynamoGraphDeployment.
      # It must be unique within the parent's `spec.components` list.
      #
      # For standalone DynamoComponentDeployment objects, the defaulting webhook populates `name`
      # from `metadata.name` on admission, so users typically do not need to set it explicitly.
      #
      # `name` is decoupled from the underlying Kubernetes resource name so that the operator can
      # rename child workloads (e.g. suffixing worker DCDs with a hash during rolling updates)
      # without losing the stable identity that downstream consumers (labels, status maps, DGDSA
      # references, planner RBAC, EPP filters) depend on.
      name: "<string>" # required, minLength: 1, maxLength: 63

      # compilationCache configures a PVC-backed compilation cache. The operator handles
      # backend-specific mount paths and environment variables, so users do not need to hand-wire
      # them into `podTemplate`. Extracted from v1alpha1's `volumeMount.useAsCompilationCache` flag.
      compilationCache: # optional
        # pvcName references a user-created PVC by name. The PVC must exist in the same namespace as
        # the DynamoGraphDeployment.
        pvcName: "<string>" # required, minLength: 1

        # mountPath overrides the backend-specific default mount path. When empty, the operator
        # selects a default appropriate for the backend framework.
        # mountPath: "<string>"

      # eppConfig holds EPP-specific configuration for Endpoint Picker Plugin components. Only
      # meaningful when `type` is `epp`.
      eppConfig: # optional
        # config allows specifying EPP `EndpointPickerConfig` directly as a structured object. The
        # operator marshals this to YAML and creates a ConfigMap automatically. Mutually exclusive
        # with `configMapRef`. One of `configMapRef` or `config` must be specified.
        config: {} # optional, preserveUnknownFields, show with --expand-depth 5

        # configMapRef references a user-provided ConfigMap containing EPP configuration. Mutually
        # exclusive with `config`.
        configMapRef: {} # optional, mapType: atomic, show with --expand-depth 5

      # experimental groups opt-in preview features whose API shape and behavior may change in
      # breaking ways between v1beta1 releases, including disappearing without a name-preserving
      # graduation path. In v1beta1 this block holds `gpuMemoryService` and `failover` (which remain
      # tightly coupled -- failover requires GMS -- and are expected to evolve together as the
      # DRA-based GPU sharing story matures), and `checkpoint` (whose interaction with the
      # standalone DynamoCheckpoint resource and identity-hash computation is still settling).
      # Fields here are explicitly NOT covered by the normal v1beta1 deprecation policy; do not
      # depend on them for production workloads.
      experimental: # optional
        # checkpoint configures container-image snapshotting and restore for this component. When
        # set, the DGD controller can produce a DGD-scoped DynamoCheckpoint CR and later restore
        # pods in the same DGD generation from that checkpoint for faster cold start. The
        # user-facing shape of this field is still settling, which is why it lives under
        # `experimental` in v1beta1 instead of at the top level.
        checkpoint: {} # optional, show with --expand-depth 5

        # failover configures active-passive GPU failover for this component. Requires
        # `gpuMemoryService` to also be set, and `failover.mode` must match `gpuMemoryService.mode`
        # (enforced by the validation webhook).
        # failover: {} # show with --expand-depth 5

        # gpuMemoryService configures the GPU Memory Service (GMS). When set, GPU access for GMS
        # clients is managed via DRA.
        gpuMemoryService: {} # optional, show with --expand-depth 5

      # frontendSidecar optionally designates a container in `podTemplate.spec.containers` as the
      # frontend sidecar. The value must match the `name` of a container in that list; the operator
      # merges its frontend-sidecar defaults (auto-generated Dynamo env vars, ports, health probes)
      # into that container the same way it merges into `"main"`. The full container definition
      # (image, args, envFrom, env) lives in `podTemplate` -- this eliminates the redundant `image`,
      # `args`, `envFromSecret`, and `envs` fields from v1alpha1's `FrontendSidecarSpec`. The
      # validation webhook rejects values that do not match any container name in
      # `podTemplate.spec.containers`.
      # frontendSidecar: "<string>"

      # globalDynamoNamespace places the component in the global Dynamo namespace rather than the
      # per-deployment namespace derived from the DGD name.
      # globalDynamoNamespace: <boolean>

      # modelRef references a model served by this component. When specified, a headless service is
      # created for endpoint discovery.
      modelRef: # optional
        # name is the base model identifier (e.g. "llama-3-70b-instruct-v1").
        name: "<string>" # required, minLength: 1

        # revision is the model revision/version.
        # revision: "<string>"

      # multinode configures multinode components.
      # multinode:
        # nodeCount is the number of nodes to deploy for the multinode component. Total GPUs used is
        # `nodeCount * container GPU request`.
        # nodeCount: 2 # default, minimum: 2

      # podTemplate is the pod template used to create the component's pods. The operator injects
      # its defaults (image, command, env, ports, probes, resources, volume mounts) into the
      # container named `"main"` inside `podTemplate.spec.containers`, merging user overrides by
      # name. If no container named `"main"` is present, the operator auto-generates it with
      # standard defaults. All other containers in `podTemplate.spec.containers` are treated as
      # user-managed sidecars: the operator does not inject defaults into them, so sidecars must
      # specify required fields (e.g. `image`) themselves. The validation webhook rejects pod
      # templates where a non-`"main"` container is missing a required field such as `image`.
      podTemplate: # optional
        # Standard object's metadata. More info:
        # https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
        # metadata: {} # show with --expand-depth 5

        # Specification of the desired behavior of the pod. More info:
        # https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
        spec: {} # optional, show with --expand-depth 5

      # replicas is the desired number of Pods for this component. When `scalingAdapter` is set on
      # this component, this field is managed by the DynamoGraphDeploymentScalingAdapter and should
      # not be modified directly.
      # replicas: <int32> # minimum: 0

      # scalingAdapter opts this component into using the DynamoGraphDeploymentScalingAdapter. When
      # set (even as an empty object, `scalingAdapter: {}`), a DGDSA is created and owns the
      # `replicas` field so that external autoscalers (HPA/KEDA/Planner) can drive scaling via the
      # Scale subresource. Omit the field to opt out.
      # scalingAdapter: {}

      # sharedMemorySize controls the size of the tmpfs mounted at `/dev/shm`. `nil` selects the
      # operator default (8Gi), a positive quantity sets a custom size, and `"0"` disables the
      # shared-memory volume entirely. Simpler replacement for v1alpha1's `SharedMemorySpec` struct
      # with its `disabled bool` + `size Quantity` pattern.
      # sharedMemorySize: <int-or-string> # intOrString

      # topologyConstraint applies to this component. `topologyConstraint.packDomain` is required.
      # When both this and `spec.topologyConstraint.packDomain` are set, this field's `packDomain`
      # must be narrower than or equal to the spec-level value.
      topologyConstraint: # optional
        # packDomain is the topology domain to pack pods within. Must match a domain defined in the
        # referenced ClusterTopology CR.
        packDomain: "<string>" # required

      # type indicates the role of this component within a Dynamo graph. Drives port mapping,
      # frontend detection, planner RBAC, and the pod label `nvidia.com/dynamo-component-type`.
      # Because `prefill` and `decode` are first-class values, users can set them directly.
      # type: "frontend" # enum: "worker" | "prefill" | "decode" | "planner" | "epp"

  # env is prepended to every component's environment. Component-specific env entries with the same
  # name take precedence and may reference values from this list.
  env: # optional
    - # Name of the environment variable. May consist of any printable ASCII characters except '='.
      name: "<string>" # required

      # Variable references $(VAR_NAME) are expanded using the previously defined environment
      # variables in the container and any service environment variables. If a variable cannot be
      # resolved, the reference in the input string will be unchanged. Double $$ are reduced to a
      # single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will produce
      # the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless of
      # whether the variable exists or not. Defaults to "".
      # value: "<string>"

      # Source for the environment variable's value. Cannot be used if value is not empty.
      valueFrom: # optional
        # Selects a key of a ConfigMap.
        configMapKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a field of the pod: supports metadata.name, metadata.namespace,
        # `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`, spec.nodeName,
        # spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.
        fieldRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # FileKeyRef selects a key of the env file. Requires the EnvFiles feature gate to be
        # enabled.
        fileKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a resource of the container: only resources limits and requests (limits.cpu,
        # limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and
        # requests.ephemeral-storage) are currently supported.
        resourceFieldRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a key of a secret in the pod's namespace
        secretKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

  # experimental groups graph-level preview features whose API shape and behavior may change in
  # breaking ways between v1beta1 releases.
  experimental: # optional
    # kvTransferPolicy configures topology-aware routing for KV-cache transfers between prefill and
    # decode workers.
    kvTransferPolicy: # optional
      # domain is the logical name for the topology level to enforce (e.g. "zone", "rack"). The
      # router uses this to match workers that share the same value for the label identified by
      # `labelKey`.
      domain: "<string>" # required

      # enforcement controls how the selected prefill worker's topology is applied to decode
      # routing. "required" only allows decode workers in the same topology domain as the selected
      # prefill worker. "preferred" keeps all decode workers eligible, but biases selection toward
      # workers in the same topology domain. Defaults to "required".
      # enforcement: "required" # default, enum: "preferred"

      # labelKey is a Kubernetes node label key (e.g. "topology.kubernetes.io/zone") whose value
      # identifies the topology domain for each worker. The operator copies the node label onto
      # worker pods so the runtime can publish it as worker metadata. The label should correspond to
      # the topology level named in `domain`.
      # labelKey: "<string>" # minLength: 1, maxLength: 317

      # preferredWeight is required and used only when enforcement is "preferred". Higher values
      # create a stronger same-domain routing preference, but do not guarantee same-domain
      # selection. The value is not a probability; worker selection still depends on load and other
      # routing inputs. A value of 0 disables the topology preference; 1 is the strongest supported
      # preference.
      # preferredWeight: <number> # minimum: 0, maximum: 1

  # labels to propagate to all child resources. Same precedence rules as `annotations`.
  # labels:
    # <key>: "<string>"

  # priorityClassName is the name of the PriorityClass to use for Grove PodCliqueSets. Requires the
  # Grove pathway.
  # priorityClassName: "<string>"

  # restart specifies the restart policy for the graph deployment.
  restart: # optional
    # id is an arbitrary string that triggers a restart when changed. Any modification to this value
    # initiates a restart of the graph deployment according to the configured strategy.
    id: "<string>" # required, minLength: 1

    # strategy specifies the restart strategy for the graph deployment.
    # strategy:
      # order is the complete ordered set of component names for sequential restarts. Omit or leave
      # empty to use the controller's default order. This field must not be set for parallel
      # restarts.
      # order:
        # - "<string>"

      # type specifies the restart strategy type.
      # type: "Sequential" # default, enum: "Parallel"

  # topologyConstraint is the deployment-level topology constraint. When set,
  # `spec.topologyConstraint.clusterTopologyName` names the ClusterTopology CR to use.
  # `spec.topologyConstraint.packDomain` is optional at this level and can be omitted when only
  # components carry constraints. Components without their own `topologyConstraint` inherit from
  # this value.
  topologyConstraint: # optional
    # clusterTopologyName is the name of the ClusterTopology resource that defines the topology
    # hierarchy for this deployment.
    clusterTopologyName: "<string>" # required, minLength: 1

    # packDomain is the default topology domain to pack pods within. Optional; omit when only
    # components carry constraints.
    # packDomain: "<string>"

# status reflects the current observed state of this graph deployment.
# status: {}
```
</details>


## nvidia.com/v1alpha1

<details open>
<summary>YAML: nvidia.com/v1alpha1</summary>

```yaml
# DynamoGraphDeployment is the Schema for the dynamographdeployments API.
apiVersion: nvidia.com/v1alpha1
kind: DynamoGraphDeployment
metadata:
  name: "<name>"
  namespace: "<namespace>"
# Spec defines the desired state for this graph deployment.
spec: # optional
  # Annotations to propagate to all child resources (PCS, DCD, Deployments, and pod templates).
  # Service-level annotations take precedence over these values.
  # annotations:
    # <key>: "<string>"

  # BackendFramework specifies the backend framework (e.g., "sglang", "vllm", "trtllm").
  # backendFramework: "sglang" # enum: "vllm" | "trtllm"

  # Envs are environment variables applied to all services in the deployment unless overridden by
  # service-specific configuration.
  envs: # optional
    - # Name of the environment variable. May consist of any printable ASCII characters except '='.
      name: "<string>" # required

      # Variable references $(VAR_NAME) are expanded using the previously defined environment
      # variables in the container and any service environment variables. If a variable cannot be
      # resolved, the reference in the input string will be unchanged. Double $$ are reduced to a
      # single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will produce
      # the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless of
      # whether the variable exists or not. Defaults to "".
      # value: "<string>"

      # Source for the environment variable's value. Cannot be used if value is not empty.
      valueFrom: # optional
        # Selects a key of a ConfigMap.
        configMapKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a field of the pod: supports metadata.name, metadata.namespace,
        # `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`, spec.nodeName,
        # spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.
        fieldRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # FileKeyRef selects a key of the env file. Requires the EnvFiles feature gate to be
        # enabled.
        fileKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a resource of the container: only resources limits and requests (limits.cpu,
        # limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and
        # requests.ephemeral-storage) are currently supported.
        resourceFieldRef: {} # optional, mapType: atomic, show with --expand-depth 5

        # Selects a key of a secret in the pod's namespace
        secretKeyRef: {} # optional, mapType: atomic, show with --expand-depth 5

  # Experimental groups graph-level preview features whose API shape and behavior may change in
  # breaking ways between releases.
  experimental: # optional
    # KvTransferPolicy configures topology-aware routing for KV-cache transfers between prefill and
    # decode workers.
    kvTransferPolicy: # optional
      # Domain is the logical name for the topology level to enforce (e.g. "zone", "rack"). The
      # router uses this to match workers that share the same value for the label identified by
      # `labelKey`.
      domain: "<string>" # required

      # Enforcement controls how the selected prefill worker's topology is applied to decode
      # routing. "required" only allows decode workers in the same topology domain as the selected
      # prefill worker. "preferred" keeps all decode workers eligible, but biases selection toward
      # workers in the same topology domain. Defaults to "required".
      # enforcement: "required" # default, enum: "preferred"

      # LabelKey is a Kubernetes node label key (e.g. "topology.kubernetes.io/zone") whose value
      # identifies the topology domain for each worker. The operator copies the node label onto
      # worker pods so the runtime can publish it as worker metadata. The label should correspond to
      # the topology level named in `domain`.
      # labelKey: "<string>" # minLength: 1, maxLength: 317

      # PreferredWeight is required and used only when enforcement is "preferred". Higher values
      # create a stronger same-domain routing preference, but do not guarantee same-domain
      # selection. The value is not a probability; worker selection still depends on load and other
      # routing inputs. A value of 0 disables the topology preference; 1 is the strongest supported
      # preference.
      # preferredWeight: <number> # minimum: 0, maximum: 1

  # Labels to propagate to all child resources (PCS, DCD, Deployments, and pod templates).
  # Service-level labels take precedence over these values.
  # labels:
    # <key>: "<string>"

  # PriorityClassName is the name of the PriorityClass to use for Grove PodCliqueSets. Requires the
  # Grove pathway.
  # priorityClassName: "<string>"

  # PVCs defines a list of persistent volume claims that can be referenced by components. Each PVC
  # must have a unique name that can be referenced in component specifications.
  pvcs: # optional
    - # Name is the name of the PVC
      name: "<string>" # required

      # Create indicates to create a new PVC
      # create: <boolean>

      # Size of the volume in Gi, used during PVC creation. Required when create is true.
      # size: <int-or-string> # intOrString

      # StorageClass to be used for PVC creation. Required when create is true.
      # storageClass: "<string>"

      # VolumeAccessMode is the volume access mode of the PVC. Required when create is true.
      # volumeAccessMode: "<string>"

  # Restart specifies the restart policy for the graph deployment.
  restart: # optional
    # ID is an arbitrary string that triggers a restart when changed. Any modification to this value
    # will initiate a restart of the graph deployment according to the strategy.
    id: "<string>" # required, minLength: 1

    # Strategy specifies the restart strategy for the graph deployment.
    # strategy:
      # Order specifies the order in which the services should be restarted.
      # order:
        # - "<string>"

      # Type specifies the restart strategy type.
      # type: "Sequential" # default, enum: "Parallel"

  # Services are the services to deploy as part of this deployment.
  services: # optional
    <key>: # optional
      # Annotations to add to generated Kubernetes resources for this component (such as Pod,
      # Service, and Ingress when applicable).
      # annotations:
        # <key>: "<string>"

      # Deprecated: This field is deprecated and ignored. Use DynamoGraphDeploymentScalingAdapter
      # with HPA, KEDA, or Planner for autoscaling instead. See docs/kubernetes/autoscaling.md for
      # migration guidance. This field will be removed in a future API version.
      autoscaling: # optional
        # Deprecated: This field is ignored.
        behavior: {} # optional, show with --expand-depth 5

        # Deprecated: This field is ignored.
        # enabled: <boolean>

        # Deprecated: This field is ignored.
        # maxReplicas: <integer>

        # Deprecated: This field is ignored.
        metrics: # optional
          - {} # show with --expand-depth 6

        # Deprecated: This field is ignored.
        # minReplicas: <integer>

      # Checkpoint configures container checkpointing for this service. When enabled, pods can be
      # restored from a checkpoint files for faster cold start.
      checkpoint: # optional
        # CheckpointRef references an existing DynamoCheckpoint CR by metadata.name. If specified,
        # this service's Identity is ignored and the referenced checkpoint is used directly.
        # checkpointRef: "<string>"

        # Enabled indicates whether checkpointing is enabled for this service
        # enabled: false # default

        # Deprecated: Identity is ignored by DGD-managed automatic checkpoints. Automatic
        # checkpoints are scoped to the owning DGD/component generation and are never reused across
        # DGDs.
        identity: {} # optional, show with --expand-depth 5

        # Job customizes the checkpoint Job that is created in Auto mode.
        # job: {} # show with --expand-depth 5

        # Mode defines how checkpoint creation is handled - Auto: DGD controller creates Checkpoint
        # CR automatically - Manual: User must create Checkpoint CR
        # mode: "Auto" # default, enum: "Manual"

        # TargetContainerName is the workload container to snapshot and restore.
        # targetContainerName: "main" # default, minLength: 1, maxLength: 63

      # ComponentType indicates the role of this component (for example, "main").
      # componentType: "<string>"

      # DynamoNamespace is deprecated and will be removed in a future version. The DGD Kubernetes
      # namespace and DynamoGraphDeployment name are used to construct the Dynamo namespace for each
      # component
      # dynamoNamespace: "<string>"

      # EnvFromSecret references a Secret whose key/value pairs will be exposed as environment
      # variables in the component containers.
      # envFromSecret: "<string>"

      # Envs defines additional environment variables to inject into the component containers.
      envs: # optional
        - # Name of the environment variable. May consist of any printable ASCII characters except
          # '='.
          name: "<string>" # required

          # Variable references $(VAR_NAME) are expanded using the previously defined environment
          # variables in the container and any service environment variables. If a variable cannot
          # be resolved, the reference in the input string will be unchanged. Double $$ are reduced
          # to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)"
          # will produce the string literal "$(VAR_NAME)". Escaped references will never be
          # expanded, regardless of whether the variable exists or not. Defaults to "".
          # value: "<string>"

          # Source for the environment variable's value. Cannot be used if value is not empty.
          valueFrom: {} # optional, show with --expand-depth 6

      # EPPConfig defines EPP-specific configuration options for Endpoint Picker Plugin components.
      # Only applicable when ComponentType is "epp".
      eppConfig: # optional
        # Config allows specifying EPP EndpointPickerConfig directly as a structured object. The
        # operator will marshal this to YAML and create a ConfigMap automatically. Mutually
        # exclusive with ConfigMapRef. One of ConfigMapRef or Config must be specified (no default
        # configuration). Uses the upstream type from
        # github.com/kubernetes-sigs/gateway-api-inference-extension
        config: {} # optional, preserveUnknownFields, show with --expand-depth 5

        # ConfigMapRef references a user-provided ConfigMap containing EPP configuration. The
        # ConfigMap should contain EndpointPickerConfig YAML. Mutually exclusive with Config.
        configMapRef: {} # optional, mapType: atomic, show with --expand-depth 5

      # ExtraPodMetadata adds labels/annotations to the created Pods.
      # extraPodMetadata:
        # annotations:
          # <key>: "<string>"

        # labels:
          # <key>: "<string>"

      # ExtraPodSpec allows to override the main pod spec configuration. It is a k8s standard
      # PodSpec. It also contains a MainContainer (standard k8s Container) field that allows
      # overriding the main container configuration.
      extraPodSpec: # optional
        # Optional duration in seconds the pod may be active on the node relative to StartTime
        # before the system will actively try to mark it failed and kill associated containers.
        # Value must be a positive integer.
        # activeDeadlineSeconds: <int64>

        # If specified, the pod's scheduling constraints
        affinity: {} # optional, show with --expand-depth 5

        # AutomountServiceAccountToken indicates whether a service account token should be
        # automatically mounted.
        # automountServiceAccountToken: <boolean>

        # List of containers belonging to the pod. Containers cannot currently be added or removed.
        # There must be at least one container in a Pod. Cannot be updated.
        containers: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # Specifies the DNS parameters of a pod. Parameters specified here will be merged to the
        # generated DNS configuration based on DNSPolicy.
        # dnsConfig: {} # show with --expand-depth 5

        # Set DNS policy for the pod. Defaults to "ClusterFirst". Valid values are
        # 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'. DNS parameters given in
        # DNSConfig will be merged with the policy selected with DNSPolicy. To have DNS options set
        # along with hostNetwork, you have to specify DNS policy explicitly to
        # 'ClusterFirstWithHostNet'.
        # dnsPolicy: "<string>"

        # EnableServiceLinks indicates whether information about services should be injected into
        # pod's environment variables, matching the syntax of Docker links. Optional: Defaults to
        # true.
        # enableServiceLinks: <boolean>

        # List of ephemeral containers run in this pod. Ephemeral containers may be run in an
        # existing pod to perform user-initiated actions such as debugging. This list cannot be
        # specified when creating a pod, and it cannot be modified by updating the pod spec. In
        # order to add an ephemeral container to an existing pod, use the pod's ephemeralcontainers
        # subresource.
        ephemeralContainers: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # HostAliases is an optional list of hosts and IPs that will be injected into the pod's
        # hosts file if specified.
        hostAliases: # optional, listType: map, listMapKeys: ip
          - {} # show with --expand-depth 6

        # Use the host's ipc namespace. Optional: Default to false.
        # hostIPC: <boolean>

        # Host networking requested for this pod. Use the host's network namespace. When using
        # HostNetwork you should specify ports so the scheduler is aware. When `hostNetwork` is
        # true, specified `hostPort` fields in port definitions must match `containerPort`, and
        # unspecified `hostPort` fields in port definitions are defaulted to match `containerPort`.
        # Default to false.
        # hostNetwork: <boolean>

        # Use the host's pid namespace. Optional: Default to false.
        # hostPID: <boolean>

        # Use the host's user namespace. Optional: Default to true. If set to true or not present,
        # the pod will be run in the host user namespace, useful for when the pod needs a feature
        # only available to the host user namespace, such as loading a kernel module with
        # CAP_SYS_MODULE. When set to false, a new userns is created for the pod. Setting false is
        # useful for mitigating container breakout vulnerabilities even allowing users to run their
        # containers as root without actually having root privileges on the host. This field is
        # alpha-level and is only honored by servers that enable the UserNamespacesSupport feature.
        # hostUsers: <boolean>

        # Specifies the hostname of the Pod If not specified, the pod's hostname will be set to a
        # system-defined value.
        # hostname: "<string>"

        # HostnameOverride specifies an explicit override for the pod's hostname as perceived by the
        # pod. This field only specifies the pod's hostname and does not affect its DNS records.
        # When this field is set to a non-empty string: - It takes precedence over the values set in
        # `hostname` and `subdomain`. - The Pod's hostname will be set to this value. -
        # `setHostnameAsFQDN` must be nil or set to false. - `hostNetwork` must be set to false.
        #
        # This field must be a valid DNS subdomain as defined in RFC 1123 and contain at most 64
        # characters. Requires the HostnameOverride feature gate to be enabled.
        # hostnameOverride: "<string>"

        # ImagePullSecrets is an optional list of references to secrets in the same namespace to use
        # for pulling any of the images used by this PodSpec. If specified, these secrets will be
        # passed to individual puller implementations for them to use. More info:
        # https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
        # imagePullSecrets: # listType: map, listMapKeys: name
          # - {} # mapType: atomic, show with --expand-depth 6

        # List of initialization containers belonging to the pod. Init containers are executed in
        # order prior to containers being started. If any init container fails, the pod is
        # considered to have failed and is handled according to its restartPolicy. The name for an
        # init container or normal container must be unique among all containers. Init containers
        # may not have Lifecycle actions, Readiness probes, Liveness probes, or Startup probes. The
        # resourceRequirements of an init container are taken into account during scheduling by
        # finding the highest request/limit for each resource type, and then using the max of that
        # value or the sum of the normal containers. Limits are applied to init containers in a
        # similar fashion. Init containers cannot currently be added or removed. Cannot be updated.
        # More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
        initContainers: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # A single application container that you want to run within a pod.
        mainContainer: {} # optional, show with --expand-depth 5

        # NodeName indicates in which node this pod is scheduled. If empty, this pod is a candidate
        # for scheduling by the scheduler defined in schedulerName. Once this field is set, the
        # kubelet for this node becomes responsible for the lifecycle of this pod. This field should
        # not be used to express a desire for the pod to be scheduled on a specific node.
        # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodename
        # nodeName: "<string>"

        # NodeSelector is a selector which must be true for the pod to fit on a node. Selector which
        # must match a node's labels for the pod to be scheduled on that node. More info:
        # https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
        # nodeSelector: # mapType: atomic
          # <key>: "<string>"

        # Specifies the OS of the containers in the pod. Some pod and container fields are
        # restricted if this is set.
        #
        # If the OS field is set to linux, the following fields must be unset:
        # -securityContext.windowsOptions
        #
        # If the OS field is set to windows, following fields must be unset: - spec.hostPID -
        # spec.hostIPC - spec.hostUsers - spec.resources - spec.securityContext.appArmorProfile -
        # spec.securityContext.seLinuxOptions - spec.securityContext.seccompProfile -
        # spec.securityContext.fsGroup - spec.securityContext.fsGroupChangePolicy -
        # spec.securityContext.sysctls - spec.shareProcessNamespace - spec.securityContext.runAsUser
        # - spec.securityContext.runAsGroup - spec.securityContext.supplementalGroups -
        # spec.securityContext.supplementalGroupsPolicy -
        # spec.containers[*].securityContext.appArmorProfile -
        # spec.containers[*].securityContext.seLinuxOptions -
        # spec.containers[*].securityContext.seccompProfile -
        # spec.containers[*].securityContext.capabilities -
        # spec.containers[*].securityContext.readOnlyRootFilesystem -
        # spec.containers[*].securityContext.privileged -
        # spec.containers[*].securityContext.allowPrivilegeEscalation -
        # spec.containers[*].securityContext.procMount -
        # spec.containers[*].securityContext.runAsUser -
        # spec.containers[*].securityContext.runAsGroup
        os: {} # optional, show with --expand-depth 5

        # Overhead represents the resource overhead associated with running a pod for a given
        # RuntimeClass. This field will be autopopulated at admission time by the RuntimeClass
        # admission controller. If the RuntimeClass admission controller is enabled, overhead must
        # not be set in Pod create requests. The RuntimeClass admission controller will reject Pod
        # create requests which have the overhead already set. If RuntimeClass is configured and
        # selected in the PodSpec, Overhead will be set to the value defined in the corresponding
        # RuntimeClass, otherwise it will remain unset and treated as zero. More info:
        # https://git.k8s.io/enhancements/keps/sig-node/688-pod-overhead/README.md
        # overhead:
          # <key>: <int-or-string> # intOrString

        # PreemptionPolicy is the Policy for preempting pods with lower priority. One of Never,
        # PreemptLowerPriority. Defaults to PreemptLowerPriority if unset.
        # preemptionPolicy: "<string>"

        # The priority value. Various system components use this field to find the priority of the
        # pod. When Priority Admission Controller is enabled, it prevents users from setting this
        # field. The admission controller populates this field from PriorityClassName. The higher
        # the value, the higher the priority.
        # priority: <int32>

        # If specified, indicates the pod's priority. "system-node-critical" and
        # "system-cluster-critical" are two special keywords which indicate the highest priorities
        # with the former being the highest priority. Any other name must be defined by creating a
        # PriorityClass object with that name. If not specified, the pod priority will be default or
        # zero if there is no default.
        # priorityClassName: "<string>"

        # If specified, all readiness gates will be evaluated for pod readiness. A pod is ready when
        # all its containers are ready AND all conditions specified in the readiness gates have
        # status equal to "True" More info:
        # https://git.k8s.io/enhancements/keps/sig-network/580-pod-readiness-gates
        readinessGates: # optional, listType: atomic
          - {} # show with --expand-depth 6

        # ResourceClaims defines which ResourceClaims must be allocated and reserved before the Pod
        # is allowed to start. The resources will be made available to those containers which
        # consume them by name.
        #
        # This is an alpha field and requires enabling the DynamicResourceAllocation feature gate.
        #
        # This field is immutable.
        resourceClaims: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # Resources is the total amount of CPU and Memory resources required by all containers in
        # the pod. It supports specifying Requests and Limits for "cpu", "memory" and "hugepages-"
        # resource names only. ResourceClaims are not supported.
        #
        # This field enables fine-grained control over resource allocation for the entire pod,
        # allowing resource sharing among containers in a pod.
        #
        # This is an alpha field and requires enabling the PodLevelResources feature gate.
        resources: {} # optional, show with --expand-depth 5

        # Restart policy for all containers within the pod. One of Always, OnFailure, Never. In some
        # contexts, only a subset of those values may be permitted. Default to Always. More info:
        # https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy
        # restartPolicy: "<string>"

        # RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be
        # used to run this pod. If no RuntimeClass resource matches the named class, the pod will
        # not be run. If unset or empty, the "legacy" RuntimeClass will be used, which is an
        # implicit class with an empty definition that uses the default runtime handler. More info:
        # https://git.k8s.io/enhancements/keps/sig-node/585-runtime-class
        # runtimeClassName: "<string>"

        # If specified, the pod will be dispatched by specified scheduler. If not specified, the pod
        # will be dispatched by default scheduler.
        # schedulerName: "<string>"

        # SchedulingGates is an opaque list of values that if specified will block scheduling the
        # pod. If schedulingGates is not empty, the pod will stay in the SchedulingGated state and
        # the scheduler will not attempt to schedule the pod.
        #
        # SchedulingGates can only be set at pod creation time, and be removed only afterwards.
        schedulingGates: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # SecurityContext holds pod-level security attributes and common container settings.
        # Optional: Defaults to empty. See type description for default values of each field.
        securityContext: {} # optional, show with --expand-depth 5

        # DeprecatedServiceAccount is a deprecated alias for ServiceAccountName. Deprecated: Use
        # serviceAccountName instead.
        # serviceAccount: "<string>"

        # ServiceAccountName is the name of the ServiceAccount to use to run this pod. More info:
        # https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
        # serviceAccountName: "<string>"

        # If true the pod's hostname will be configured as the pod's FQDN, rather than the leaf name
        # (the default). In Linux containers, this means setting the FQDN in the hostname field of
        # the kernel (the nodename field of struct utsname). In Windows containers, this means
        # setting the registry value of hostname for the registry key
        # HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters to FQDN. If a
        # pod does not have FQDN, this has no effect. Default to false.
        # setHostnameAsFQDN: <boolean>

        # Share a single process namespace between all of the containers in a pod. When this is set
        # containers will be able to view and signal processes from other containers in the same
        # pod, and the first process in each container will not be assigned PID 1. HostPID and
        # ShareProcessNamespace cannot both be set. Optional: Default to false.
        # shareProcessNamespace: <boolean>

        # If specified, the fully qualified Pod hostname will be "<hostname>.<subdomain>.<pod
        # namespace>.svc.<cluster domain>". If not specified, the pod will not have a domainname at
        # all.
        # subdomain: "<string>"

        # Optional duration in seconds the pod needs to terminate gracefully. May be decreased in
        # delete request. Value must be non-negative integer. The value zero indicates stop
        # immediately via the kill signal (no opportunity to shut down). If this value is nil, the
        # default grace period will be used instead. The grace period is the duration in seconds
        # after the processes running in the pod are sent a termination signal and the time when the
        # processes are forcibly halted with a kill signal. Set this value longer than the expected
        # cleanup time for your process. Defaults to 30 seconds.
        # terminationGracePeriodSeconds: <int64>

        # If specified, the pod's tolerations.
        # tolerations: # listType: atomic
          # - {} # show with --expand-depth 6

        # TopologySpreadConstraints describes how a group of pods ought to spread across topology
        # domains. Scheduler will schedule pods in a way which abides by the constraints. All
        # topologySpreadConstraints are ANDed.
        topologySpreadConstraints: # optional, listType: map, listMapKeys: topologyKey,
                                   # whenUnsatisfiable
          - {} # show with --expand-depth 6

        # List of volumes that can be mounted by containers belonging to the pod. More info:
        # https://kubernetes.io/docs/concepts/storage/volumes
        volumes: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

      # Failover configures GMS (GPU Memory Service) failover for this service. For intraPod mode:
      # the main container is cloned into two engine containers (active + standby). For interPod
      # mode: the operator creates a dedicated GMS weight server pod and multiple engine pods per
      # rank that share GPUs via DRA resource claims.
      failover: # optional
        # Enabled activates failover mode.
        enabled: <boolean> # required

        # Mode selects the failover deployment topology. intraPod: engine containers run within the
        # same pod (requires gpuMemoryService.enabled). interPod: a dedicated GMS weight server pod
        # + engine pods per rank (requires Grove).
        # mode: "intraPod" # default, enum: "interPod"

        # NumShadows is the number of shadow (standby) engine pods per rank. Total engine pods per
        # rank = NumShadows + 1 (1 primary + NumShadows shadows).
        #
        # NumShadows is only meaningful for mode=interPod; intraPod uses a fixed 1 primary + 1
        # shadow sidecar layout and any value other than 1 is rejected at admission time.
        # numShadows: 1 # default, minimum: 1

      # FrontendSidecar configures an auto-generated frontend sidecar container. When specified, the
      # operator injects a fully configured frontend container with all standard Dynamo environment
      # variables, health probes, and ports. This eliminates the need to manually specify these in
      # extraPodSpec.containers. (GAIE)
      frontendSidecar: # optional
        # Image is the container image for the frontend sidecar.
        image: "<string>" # required

        # Args overrides the default frontend arguments. When specified, these replace the default
        # ["-m", "dynamo.frontend"] entirely. For example, ["-m", "dynamo.frontend",
        # "--router-mode", "direct"] for GAIE deployments.
        # args:
          # - "<string>"

        # EnvFromSecret references a Secret whose key/value pairs will be exposed as environment
        # variables in the frontend sidecar container.
        # envFromSecret: "<string>"

        # Envs defines additional environment variables for the frontend sidecar. These are merged
        # with (and can override) the auto-generated Dynamo env vars.
        envs: # optional
          - {} # show with --expand-depth 6

      # GlobalDynamoNamespace indicates that the Component will be placed in the global Dynamo
      # namespace
      # globalDynamoNamespace: <boolean>

      # GPUMemoryService configures the GPU Memory Service (GMS) sidecar. When enabled, a GMS
      # sidecar is injected and GPU access is managed via DRA.
      gpuMemoryService: # optional
        # Enabled activates GMS wiring. GPU resources on client containers are replaced with a DRA
        # ResourceClaim for shared GPU access.
        enabled: <boolean> # required

        # DeviceClassName is the DRA DeviceClass to request GPUs from.
        # deviceClassName: "gpu.nvidia.com" # default

        # ExtraClientContainers lists additional user-declared containers that should be wired as
        # GMS clients in pods rendered from the enclosing spec. DGD/DCD services apply this to
        # service pods. Auto-created checkpoints apply checkpoint job clients before creating the
        # DynamoCheckpoint; manual DynamoCheckpoint users must provide an already-prepared pod
        # template. In each rendered pod, only matching container names are wired; absent names are
        # ignored.
        # extraClientContainers: # listType: set
          # - "<string>"

        # ExtraClientPods declares additional GMS client pods for inter-pod GMS. This field is
        # reserved for future use and is rejected until inter-pod client orchestration is wired.
        extraClientPods: # optional, listType: map, listMapKeys: name
          - {} # show with --expand-depth 6

        # Mode selects the GMS deployment topology.
        # mode: "intraPod" # default, enum: "interPod"

      # Ingress config to expose the component outside the cluster (or through a service mesh).
      # ingress:
        # Annotations to set on the generated Ingress/VirtualService resources.
        # annotations:
          # <key>: "<string>"

        # Enabled exposes the component through an ingress or virtual service when true.
        # enabled: <boolean>

        # Host is the base host name to route external traffic to this component.
        # host: "<string>"

        # HostPrefix is an optional prefix added before the host.
        # hostPrefix: "<string>"

        # HostSuffix is an optional suffix appended after the host.
        # hostSuffix: "<string>"

        # IngressControllerClassName selects the ingress controller class (e.g., "nginx").
        # ingressControllerClassName: "<string>"

        # Labels to set on the generated Ingress/VirtualService resources.
        # labels:
          # <key>: "<string>"

        # TLS holds the TLS configuration used by the Ingress/VirtualService.
        # tls: {} # show with --expand-depth 5

        # UseVirtualService indicates whether to configure a service-mesh VirtualService instead of
        # a standard Ingress.
        # useVirtualService: <boolean>

        # VirtualServiceGateway optionally specifies the gateway name to attach the VirtualService
        # to.
        # virtualServiceGateway: "<string>"

      # Labels to add to generated Kubernetes resources for this component.
      # labels:
        # <key>: "<string>"

      # LivenessProbe to detect and restart unhealthy containers.
      livenessProbe: # optional
        # Exec specifies a command to execute in the container.
        # exec: {} # show with --expand-depth 5

        # Minimum consecutive failures for the probe to be considered failed after having succeeded.
        # Defaults to 3. Minimum value is 1.
        # failureThreshold: <int32>

        # GRPC specifies a GRPC HealthCheckRequest.
        grpc: {} # optional, show with --expand-depth 5

        # HTTPGet specifies an HTTP GET request to perform.
        httpGet: {} # optional, show with --expand-depth 5

        # Number of seconds after the container has started before liveness probes are initiated.
        # More info:
        # https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
        # initialDelaySeconds: <int32>

        # How often (in seconds) to perform the probe. Default to 10 seconds. Minimum value is 1.
        # periodSeconds: <int32>

        # Minimum consecutive successes for the probe to be considered successful after having
        # failed. Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
        # successThreshold: <int32>

        # TCPSocket specifies a connection to a TCP port.
        tcpSocket: {} # optional, show with --expand-depth 5

        # Optional duration in seconds the pod needs to terminate gracefully upon probe failure. The
        # grace period is the duration in seconds after the processes running in the pod are sent a
        # termination signal and the time when the processes are forcibly halted with a kill signal.
        # Set this value longer than the expected cleanup time for your process. If this value is
        # nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this value overrides
        # the value provided by the pod spec. Value must be non-negative integer. The value zero
        # indicates stop immediately via the kill signal (no opportunity to shut down). This is a
        # beta field and requires enabling ProbeTerminationGracePeriod feature gate. Minimum value
        # is 1. spec.terminationGracePeriodSeconds is used if unset.
        # terminationGracePeriodSeconds: <int64>

        # Number of seconds after which the probe times out. Defaults to 1 second. Minimum value is
        # 1. More info:
        # https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
        # timeoutSeconds: <int32>

      # ModelRef references a model that this component serves When specified, a headless service
      # will be created for endpoint discovery
      modelRef: # optional
        # Name is the base model identifier (e.g., "llama-3-70b-instruct-v1")
        name: "<string>" # required

        # Revision is the model revision/version (optional)
        # revision: "<string>"

      # Multinode is the configuration for multinode components.
      multinode: # optional
        # Indicates the number of nodes to deploy for multinode components. Total number of GPUs is
        # NumberOfNodes * GPU limit. Must be greater than 1.
        nodeCount: 2 # default, required, minimum: 2

      # ReadinessProbe to signal when the container is ready to receive traffic.
      readinessProbe: # optional
        # Exec specifies a command to execute in the container.
        # exec: {} # show with --expand-depth 5

        # Minimum consecutive failures for the probe to be considered failed after having succeeded.
        # Defaults to 3. Minimum value is 1.
        # failureThreshold: <int32>

        # GRPC specifies a GRPC HealthCheckRequest.
        grpc: {} # optional, show with --expand-depth 5

        # HTTPGet specifies an HTTP GET request to perform.
        httpGet: {} # optional, show with --expand-depth 5

        # Number of seconds after the container has started before liveness probes are initiated.
        # More info:
        # https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
        # initialDelaySeconds: <int32>

        # How often (in seconds) to perform the probe. Default to 10 seconds. Minimum value is 1.
        # periodSeconds: <int32>

        # Minimum consecutive successes for the probe to be considered successful after having
        # failed. Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
        # successThreshold: <int32>

        # TCPSocket specifies a connection to a TCP port.
        tcpSocket: {} # optional, show with --expand-depth 5

        # Optional duration in seconds the pod needs to terminate gracefully upon probe failure. The
        # grace period is the duration in seconds after the processes running in the pod are sent a
        # termination signal and the time when the processes are forcibly halted with a kill signal.
        # Set this value longer than the expected cleanup time for your process. If this value is
        # nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this value overrides
        # the value provided by the pod spec. Value must be non-negative integer. The value zero
        # indicates stop immediately via the kill signal (no opportunity to shut down). This is a
        # beta field and requires enabling ProbeTerminationGracePeriod feature gate. Minimum value
        # is 1. spec.terminationGracePeriodSeconds is used if unset.
        # terminationGracePeriodSeconds: <int64>

        # Number of seconds after which the probe times out. Defaults to 1 second. Minimum value is
        # 1. More info:
        # https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
        # timeoutSeconds: <int32>

      # Replicas is the desired number of Pods for this component. When scalingAdapter is enabled,
      # this field is managed by the DynamoGraphDeploymentScalingAdapter and should not be modified
      # directly.
      # replicas: <int32> # minimum: 0

      # Resources requested and limits for this component, including CPU, memory, GPUs/devices, and
      # any runtime-specific resources.
      resources: # optional
        # Claims specifies resource claims for dynamic resource allocation
        claims: # optional
          - {} # show with --expand-depth 6

        # Limits specifies the maximum resources allowed for the component
        # limits: {} # show with --expand-depth 5

        # Requests specifies the minimum resources required by the component
        # requests: {} # show with --expand-depth 5

      # ScalingAdapter configures whether this service uses the DynamoGraphDeploymentScalingAdapter.
      # When enabled, replicas are managed via DGDSA and external autoscalers can scale the service
      # using the Scale subresource. When disabled, replicas can be modified directly.
      # scalingAdapter:
        # Enabled indicates whether the ScalingAdapter should be enabled for this service. When
        # true, a DGDSA is created and owns the replicas field. When false (default), no DGDSA is
        # created and replicas can be modified directly in the DGD.
        # enabled: false # default

      # The name of the component
      # serviceName: "<string>"

      # SharedMemory controls the tmpfs mounted at /dev/shm (enable/disable and size).
      # sharedMemory:
        # Disabled, when true, opts out of mounting a shared-memory medium for the component. When
        # false (or unset), shared memory is enabled and Size is required (enforced by the
        # validating webhook). Size is ignored when Disabled is true.
        # disabled: <boolean>

        # size: <int-or-string> # intOrString

      # SubComponentType indicates the sub-role of this component (for example, "prefill").
      # subComponentType: "<string>"

      # TopologyConstraint for this service. packDomain is required. When both this and
      # spec.topologyConstraint.packDomain are set, packDomain must be narrower than or equal to the
      # spec-level packDomain.
      topologyConstraint: # optional
        # PackDomain is the topology domain to pack pods within. Must match a domain defined in the
        # referenced ClusterTopology CR.
        packDomain: "<string>" # required

      # VolumeMounts references PVCs defined at the top level for volumes to be mounted by the
      # component.
      volumeMounts: # optional
        - # Name references a PVC name defined in the top-level PVCs map
          name: "<string>" # required

          # MountPoint specifies where to mount the volume. If useAsCompilationCache is true and
          # mountPoint is not specified, a backend-specific default will be used.
          # mountPoint: "<string>"

          # UseAsCompilationCache indicates this volume should be used as a compilation cache. When
          # true, backend-specific environment variables will be set and default mount points may be
          # used.
          # useAsCompilationCache: false # default

  # TopologyConstraint is the deployment-level topology constraint. When set, topologyProfile is
  # required and names the ClusterTopology CR to use. packDomain is optional here — it can be
  # omitted when only services carry constraints. Services without their own topologyConstraint
  # inherit from this value.
  topologyConstraint: # optional
    # TopologyProfile is the name of the ClusterTopology CR that defines the topology hierarchy for
    # this deployment.
    topologyProfile: "<string>" # required, minLength: 1

    # PackDomain is the default topology domain to pack pods within. Optional — omit when only
    # services carry constraints.
    # packDomain: "<string>"

# Status reflects the current observed state of this graph deployment.
# status: {}
```
</details>
