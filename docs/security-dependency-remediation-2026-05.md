# Dependency Security Remediation - May 2026

## Scope

This change resolves the dependency alerts reported against `go.mod` and
`requirements.txt`. It changes dependency versions only, except for the
required Go import-path migration from the unmaintained JWT v3 module to its
patched v4 module.

| Dependency | Previous version | Remediated version | Alert addressed |
| --- | ---: | ---: | --- |
| `github.com/jackc/pgx/v5` | `v5.7.6` | `v5.9.2` | Memory-safety issue; SQL injection from placeholder confusion in dollar-quoted literals |
| `google.golang.org/grpc` | `v1.78.0` | `v1.81.1` | Authorization bypass for a malformed `:path` without a leading slash |
| `github.com/golang-jwt/jwt` | `v3.2.2+incompatible` | `github.com/golang-jwt/jwt/v4 v4.5.2` | Excessive allocation during JWT header parsing |
| `go.opentelemetry.io/otel` | `v1.38.0` | `v1.43.0` | Excessive allocations while extracting multi-value `baggage` headers |
| `golang.org/x/image` | `v0.18.0` | `v0.38.0` | Out-of-memory denial of service while decoding crafted TIFF data |
| `github.com/redis/go-redis/v9` | `v9.7.1` | `v9.7.3` | Possible out-of-order responses when `CLIENT SETINFO` times out during connection establishment |
| Python `protobuf` | `5.29.5` | `5.29.6` | JSON recursion-depth limit bypass |
| Python `python-dotenv` | `1.1.1` | `1.2.2` | Symlink-following arbitrary file overwrite in `set_key` fallback handling |

## What The Issues Are

### Go runtime paths

- `pgx` is the PostgreSQL driver imported by the storage, users, authz, and
  notification services. The memory-safety flaw could be triggered while
  processing hostile PostgreSQL protocol input. The SQL parsing flaw affected
  SQL construction involving placeholder replacement and dollar-quoted string
  literals. Upgrading the driver applies both upstream fixes.
- gRPC is used for internal RPC servers and gateway clients. An HTTP/2 request
  whose `:path` does not begin with `/` could bypass method-based authorization
  decisions in affected gRPC-Go versions.
- JWT parsing is used by the authorization service. A large malicious token
  header can make the old parser allocate excessive memory before rejecting the
  token. The maintained v4 module includes a parser bound for this case.
- OpenTelemetry propagation may parse request baggage when instrumentation is
  enabled. Multiple `baggage` header values can cause amplification through
  excessive allocations in the affected releases.
- The gateway uses `image.Decode` while validating uploaded storage images and
  imports `golang.org/x/image/webp`. The reported issue is in TIFF decoding,
  and no `golang.org/x/image/tiff` import is present in this repository, so a
  reachable TIFF attack path was not identified. The directly required module
  is still upgraded so vulnerable decoder code is not available to later
  imports or build variants.
- Redis is used in authentication OTP and gateway/search cache code. The
  affected client could misassociate responses after a timeout during its
  connection setup command, producing incorrect application results.

### Python runtime paths

- The AI service installs Python `protobuf`. A recursively crafted JSON
  protobuf payload can bypass the intended depth accounting and cause resource
  exhaustion if JSON protobuf parsing is introduced or exercised by gRPC
  handling.
- The AI service imports `load_dotenv`, not `set_key`; therefore the reported
  file-overwrite primitive is not currently called in repository code.
  Updating still prevents exposure if environment-file writing is added later
  or used in administrative scripts.

## What Could Break

| Change | Compatibility risk | Checks required |
| --- | --- | --- |
| `pgx` upgrade | Database wire behavior, parameter encoding, or error details could differ in edge cases. The SQL-injection fix may reject or handle unusual dollar-quoted query text differently. | Run database integration tests and exercise login, notification, storage, and user CRUD operations. |
| gRPC upgrade | More malformed RPC paths are rejected; any proxy producing invalid gRPC `:path` values will fail instead of reaching a handler. Transitive `x/net`, protobuf support, and genproto modules also move forward. | Verify service-to-service RPC calls and any ingress/proxy gRPC routing. |
| JWT v3 to v4 module path | This is a major-module import change. Existing HS256 token claim encoding remains compatible because the code retains `StandardClaims`, but parser validation behavior and errors may be stricter. | Verify existing access/refresh tokens, login, token refresh, OTP flows, and password reset tokens. |
| OpenTelemetry upgrade | Propagation and SDK handling may change emitted telemetry or header processing; this should not alter business responses. | Inspect tracing export and baggage propagation in a traced request. |
| `x/image` upgrade | Decoder hardening can reject files previously accepted if another binary or future path imports TIFF; the existing WebP path also receives intervening module fixes. | Upload and retrieve supported WebP images; test malformed input rejection. |
| Redis upgrade | Connection initialization behavior around Redis servers or proxies that do not support `CLIENT SETINFO` changes; this is necessary to avoid mismatched responses. | Test OTP operations and search/cache reads against the deployment Redis version. |
| Python `protobuf` upgrade | Generated Python stubs require a compatible runtime; this patch stays in the `5.29.x` line and is low risk. | Start the AI service and execute its recommendation gRPC request. |
| `python-dotenv` upgrade | Parsing or file-writing edge cases may differ. The present code only reads `.env` files. | Start the AI service with its configured environment file. |

## Validation Notes

Dependency scanning should be rerun after merging because GitHub alerts are
closed against the committed dependency graph. Locally, `go mod verify` and
`go test ./...` passed after the upgrades. The AI service dependency
installation and service/database integration checks remain appropriate in CI
or a deployed test environment.

## Advisory References

- Go vulnerability database: [GO-2026-4602 (`pgx` memory-safety)](https://pkg.go.dev/vuln/GO-2026-4602)
- Go vulnerability database: [GO-2026-4588 (`pgx` SQL injection)](https://pkg.go.dev/vuln/GO-2026-4588)
- GitHub advisory: [GHSA-mh55-gqvf-xfwm (gRPC-Go)](https://github.com/advisories/GHSA-mh55-gqvf-xfwm)
- GitHub advisory: [GHSA-mh63-6h87-95cp (`jwt-go`)](https://github.com/advisories/GHSA-mh63-6h87-95cp)
- GitHub advisory: [GHSA-9h8m-3fm2-qjrq (OpenTelemetry-Go)](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq)
- Go vulnerability database: [GO-2026-4603 (`x/image` TIFF)](https://pkg.go.dev/vuln/GO-2026-4603)
- GitHub advisory: [GHSA-7xcr-6qr3-2hmm (`go-redis`)](https://github.com/advisories/GHSA-7xcr-6qr3-2hmm)
- GitHub advisory: [GHSA-8qvm-5x2c-j2w7 (Python `protobuf`)](https://github.com/advisories/GHSA-8qvm-5x2c-j2w7)
- GitHub advisory: [GHSA-h2x3-3mp7-8r28 (`python-dotenv`)](https://github.com/advisories/GHSA-h2x3-3mp7-8r28)
