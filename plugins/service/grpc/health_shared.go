package grpc

// requiredReadinessResourceName is the shared resource name that indicates
// whether all required upstream services are ready. It is published by the
// gRPC client plugin after its required-check passes, and can be updated later
// if a background monitor is introduced.
const requiredReadinessResourceName = "lynx.grpc.required_upstreams_ready"
