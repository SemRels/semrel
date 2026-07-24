# Test suites and shared harness

Reusable test foundations live in `pkg/sdk/sdktest`, next to the public plugin
SDK. Keeping them here gives core and independently versioned plugins one
contract instead of copying HTTP servers and subprocess runners. The
`semrel-plugins` repository remains the plugin catalog; it is not a second SDK.

## Suite conventions

| Suite | Build constraint | Purpose | External requirements |
|---|---|---|---|
| Unit | none | Pure logic and in-process behavior | None |
| Integration | `//go:build integration` | Required hermetic component contracts | Local processes/loopback only |
| Container | `//go:build container` | Real protocol, persistence, or process boundaries | Docker |
| Live | `//go:build live` | Optional smoke against a disposable external account | Explicit credentials |

Run them independently:

```sh
go test ./...
go test -tags=integration ./...
go test -tags=container ./...
go test -tags=live ./...
```

Integration and container tests use bounded contexts, random ports and unique
resource names. They must register cleanup immediately and print captured
process/container logs on failure. Required suites must not turn missing
configuration into a successful skip. Live tests are never the only coverage
for a behavior. Use Testcontainers only when testing a real process, protocol,
or persistence boundary; do not containerize pure logic or SaaS contracts.

## Stateful HTTP contracts

`sdktest.NewContractServer` consumes ordered `ExpectedRequest` values. Each
expectation can check method, exact request URI (including query), a header
subset, and exact body, then return a configured status, headers, body, or
delay. Requests are always recorded, and cleanup checks for mismatches,
unexpected calls, and missing calls.

```go
server := sdktest.NewContractServer(t, sdktest.ExpectedRequest{
    Method: http.MethodPost,
    Path:   "/hooks?wait=true",
    Headers: http.Header{
        "Authorization": {"Bearer test-token"},
        "Content-Type":  {"application/json"},
    },
    Body: []byte(`{"version":"v1.2.3"}`),
    Response: sdktest.Response{Status: http.StatusAccepted},
})
client := NewClient(server.URL)
require.NoError(t, client.Notify(context.Background()))
require.Len(t, server.Requests(), 1)
```

Use multiple expectations for retries, pagination, rate limits, and recovery
from configured errors. `WaitForRequests` synchronizes cancellation tests
without sleeps.

## Plugin subprocess conformance

`sdktest.RunPlugin` removes inherited `SEMREL_*` values, installs the standard
`ReleaseEnvironment` plus canonicalized `SEMREL_PLUGIN_*` configuration, and
captures stdout, stderr, exit status, timeout, and parent-cancellation state.
It always waits for a cancelled child before returning.

```go
result := sdktest.RunPlugin(ctx, binary, sdktest.RunOptions{
    Environment: sdktest.ReleaseEnvironment{
        Version: "v1.2.3", TagName: "v1.2.3", DryRun: true,
    },
    PluginConfig: map[string]string{"token": "test-token"},
    Timeout: 5 * time.Second,
})
if !result.Success() {
    t.Fatalf("plugin: exit=%d timeout=%t stderr=%s",
        result.ExitCode, result.TimedOut, result.Stderr)
}
```

Conformance tests should cover the environment consumed by the plugin,
dry-run behavior, stdout format, diagnostic stderr, success and failure exit
statuses, cancellation, and the absence of secret values in output.
