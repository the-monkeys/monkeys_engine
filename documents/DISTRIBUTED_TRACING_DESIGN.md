# Distributed Tracing Design Document

**Project:** The Monkeys Engine  
**Author:** Engineering Team  
**Status:** DRAFT — Pending Review  
**Created:** 2026-04-24  
**Last Updated:** 2026-04-24

---

## Table of Contents

1. [What Problem Are We Solving?](#1-what-problem-are-we-solving)
2. [What Is Distributed Tracing?](#2-what-is-distributed-tracing)
3. [Where We Stand Today](#3-where-we-stand-today)
4. [Industry Standard Approaches](#4-industry-standard-approaches)
5. [Our Recommended Approach](#5-our-recommended-approach)
6. [Low-Level Design](#6-low-level-design)
7. [High-Level Design](#7-high-level-design)
8. [Implementation Plan](#8-implementation-plan)
9. [Backward Compatibility](#9-backward-compatibility)
10. [Risk & Mitigation](#10-risk--mitigation)
11. [Success Criteria](#11-success-criteria)
12. [Open Questions](#12-open-questions)

---

## 1. What Problem Are We Solving?

Today, when something goes wrong in The Monkeys Engine, finding the root cause is painful.

**Example scenario:** A user publishes a blog post. The request flows like this:

```
User Browser
  → Gateway (HTTP)
    → Authz Service (gRPC) — checks permissions
    → Blog Service (gRPC) — saves the blog
      → RabbitMQ message → Activity Service — logs the event
      → RabbitMQ message → Notification Service — sends notification
      → RabbitMQ message → Storage Service — stores files
```

If the user sees an error, we currently have to:
1. SSH into the Gateway container, search the logs.
2. SSH into the Blog container, search the logs.
3. SSH into every downstream consumer, search more logs.
4. Try to match timestamps across containers (often unreliable because of clock drift).
5. Hope we find the right log line among thousands.

**This can take hours.** There is no way to say *"show me everything that happened for this one user request."*

### What We Want Instead

We want the ability to:
- Assign a **unique ID** to every incoming request at the Gateway.
- **Pass that ID** through every gRPC call and every RabbitMQ message.
- **Include that ID** in every log line across every service.
- **Search by that ID** to see the full journey of a request across all services.
- Optionally, **visualize** the request flow with timing information.

---

## 2. What Is Distributed Tracing?

Distributed tracing is a technique to track a single request as it travels through multiple services. Think of it like a package tracking number — every time the package changes hands, the tracking number stays the same, so you can see the full journey.

### Key Terminology

| Term | What It Means | Example |
|------|---------------|---------|
| **Trace** | The complete journey of one request from start to finish | A user publishing a blog — from the Gateway all the way to Storage |
| **Trace ID** | A unique identifier for the entire journey | `abc123-def456-ghi789` |
| **Span** | One step in the journey | "Gateway received HTTP request" or "Blog Service saved to Postgres" |
| **Span ID** | A unique identifier for one step | `span-001` |
| **Parent Span** | The step that triggered the current step | The Gateway span is the parent of the Authz span |
| **Context Propagation** | Passing the Trace ID from one service to the next | Adding Trace ID to gRPC metadata or RabbitMQ message headers |
| **Baggage** | Extra key-value data that travels with the trace | `user_id=abc`, `account_id=xyz` |

### Visual: What A Trace Looks Like

```
Trace ID: abc123-def456

Gateway (HTTP POST /api/v1/blog/publish/xyz)     |----- 250ms ------|
  ├─ Authz Service (gRPC CheckAccessLevel)          |-- 15ms --|
  ├─ Blog Service (gRPC PublishBlog)                    |--- 80ms ---|
  │    ├─ Postgres INSERT                                  |- 12ms -|
  │    └─ Publish to RabbitMQ                              |- 3ms -|
  ├─ Activity Service (RabbitMQ Consumer)                        |-- 20ms --|
  ├─ Notification Service (RabbitMQ Consumer)                    |--- 45ms ---|
  └─ Storage Service (RabbitMQ Consumer)                         |-- 30ms --|
```

By looking at one trace, you instantly see:
- Which service is slow.
- Which service failed.
- The exact order of operations.
- How long each step took.

---

## 3. Where We Stand Today

### What We Already Have (Good News)

| What | Status | Details |
|------|--------|---------|
| Structured logging (Zap) | ✅ Done | All services use `logger.ZapForService("service-name")` with structured fields |
| CORS headers allow trace headers | ✅ Done | `X-Request-ID`, `X-Correlation-ID`, `X-Session-ID`, `X-Client-ID` are allowed in CORS |
| Protobuf has correlation field | ✅ Partial | `ClientInfo` in `gw_activity.proto` has `x_correlation_id` field (field 34) |
| Publisher confirms on RabbitMQ | ✅ Done | Reliable message delivery with broker acknowledgment |
| Health checks on all services | ✅ Done | gRPC health probes + HTTP `/healthz` on Gateway |
| `google/uuid` in go.mod | ✅ Done | Already using UUID generation library |
| OpenTelemetry SDK in go.mod | ✅ Partial | `go.opentelemetry.io/auto/sdk` is an indirect dependency |

### What Is Missing (The Gaps)

| Gap | Impact |
|-----|--------|
| No Trace ID generated at Gateway | Cannot link requests across services |
| gRPC calls use `context.Background()` | No deadline, no trace propagation, no cancellation |
| RabbitMQ messages carry no trace headers | Async operations are completely invisible |
| Logs do not include any request/trace ID | Cannot search by request across services |
| No tracing backend (Jaeger/Tempo/etc.) | No visualization or trace storage |
| No correlation between HTTP → gRPC → RabbitMQ | Three separate worlds with no connection |

---

## 4. Industry Standard Approaches

Below are the four major approaches used in the industry today. We compare them honestly.

### Approach A: Manual Correlation ID (DIY)

**How it works:** Generate a UUID at the Gateway. Pass it through gRPC metadata and RabbitMQ headers manually. Add it to every log line.

| Pros | Cons |
|------|------|
| Zero new dependencies | Manual work in every service |
| Full control over implementation | No visualization (Jaeger-like UI) |
| Lightweight — no sidecar, no collector | No automatic span timing |
| Easy to understand | Need to build search tooling yourself |
| Works with any infrastructure | Error-prone: easy to forget propagation in new code |

**Who uses this:** Small startups, teams that only need log correlation.

**Best for:** Teams that want "just searchable logs" without investing in tracing infrastructure.

---

### Approach B: OpenTelemetry (OTel) — Industry Standard

**How it works:** Use the OpenTelemetry SDK in each service. It automatically generates Trace IDs and Spans. Traces are exported to a backend (Jaeger, Tempo, Zipkin). The SDK has built-in support for gRPC and HTTP instrumentation.

| Pros | Cons |
|------|------|
| Industry standard (CNCF graduated project) | New dependency in every service |
| Auto-instrumentation for gRPC, HTTP, SQL | Requires a collector + backend (Jaeger/Tempo) |
| Rich visualization (flame graphs, dependency maps) | Adds memory + CPU overhead (small but real) |
| Vendor-neutral — switch backends without code changes | Learning curve for the team |
| Huge community, well-documented | Adds container(s) to docker-compose |
| Built-in context propagation | |

**Who uses this:** Google, Microsoft, Uber, Stripe, Shopify, most mid-to-large companies.

**Best for:** Teams that want proper tracing with visualization and are okay adding infrastructure.

---

### Approach C: Service Mesh (Istio, Linkerd)

**How it works:** A sidecar proxy is deployed alongside each service. The proxy automatically injects trace headers into all network traffic. No code changes needed.

| Pros | Cons |
|------|------|
| Zero code changes for basic tracing | Massive infrastructure complexity |
| Handles mTLS, retries, circuit breaking too | Requires Kubernetes (not Docker Compose) |
| Automatic for all HTTP/gRPC traffic | Heavy resource overhead (sidecar per service) |
| | Cannot trace RabbitMQ messages |
| | Overkill for our scale |

**Who uses this:** Large enterprises on Kubernetes (100+ services).

**Not suitable for us:** We run on Docker Compose, not Kubernetes. This approach is overkill.

---

### Approach D: Hybrid — Manual Correlation + OpenTelemetry (Phased)

**How it works:** Start with manual Correlation IDs (Approach A) to get immediate value. Then layer on OpenTelemetry (Approach B) incrementally, service by service.

| Pros | Cons |
|------|------|
| Immediate value from Phase 1 (days, not weeks) | Two phases of work |
| No new infrastructure in Phase 1 | Phase 2 still needs Jaeger/Tempo |
| Phase 2 is optional and incremental | Slightly more total effort than doing B from start |
| Backward compatible at every step | |
| Team learns gradually | |

**Who uses this:** Teams migrating from monolith to microservices, teams that want to ship fast.

---

### Comparison Matrix

| Criteria | A: Manual | B: OTel | C: Mesh | D: Hybrid |
|----------|-----------|---------|---------|-----------|
| Time to first value | Days | 1-2 weeks | Weeks+ | Days |
| New infrastructure needed | None | Jaeger + Collector | Kubernetes + Istio | None → Jaeger later |
| Code changes | Medium | Medium | None | Medium (phased) |
| RabbitMQ tracing | Manual | Manual + helpers | Not supported | Manual + OTel later |
| Visualization | None (logs only) | Full (Jaeger UI) | Full | Logs → Full later |
| Docker Compose compatible | ✅ Yes | ✅ Yes | ❌ No | ✅ Yes |
| Backward compatible | ✅ Yes | ✅ Yes | ❌ No | ✅ Yes |
| Team learning curve | Low | Medium | High | Low → Medium |
| Long-term scalability | Limited | Excellent | Excellent | Excellent |

---

## 5. Our Recommended Approach

### Decision: Approach D — Hybrid (Phased)

We recommend the **Hybrid approach** for the following reasons:

1. **Immediate value:** Phase 1 gives us searchable, correlated logs within days — no new containers, no new infrastructure.
2. **Backward compatible:** Existing services continue working. We add tracing incrementally.
3. **Docker Compose friendly:** No Kubernetes required. Jaeger runs as a single container when we need it.
4. **Team-friendly:** The team learns tracing concepts with simple Correlation IDs first, then graduates to OpenTelemetry.
5. **Production-safe:** Phase 1 is pure Go code — no external dependencies, no performance risk.

### What Each Phase Delivers

| Phase | What You Get | New Infra | Timeline |
|-------|-------------|-----------|----------|
| **Phase 1: Correlation IDs** | Every log line across every service includes a Trace ID. You can grep for one ID and see the entire request flow. | None | ~1 week |
| **Phase 2: OpenTelemetry + Jaeger** | Visual trace timeline (like the diagram above). Automatic span timing. Jaeger UI for searching traces. | Jaeger container + OTel Collector | ~2-3 weeks |

---

## 6. Low-Level Design

This section describes the exact technical changes for Phase 1 (Correlation IDs).

### 6.1 The Trace Context Package

Create a new shared package: `tracing/`

```
the_monkeys_engine/
  tracing/
    context.go      — Trace ID generation + context helpers
    grpc.go         — gRPC interceptors for propagation
    rabbitmq.go     — RabbitMQ header helpers
    middleware.go   — Gin middleware for the Gateway
```

#### 6.1.1 `tracing/context.go`

```go
package tracing

import (
    "context"
    "github.com/google/uuid"
)

// Context keys (unexported to prevent collisions)
type ctxKey string

const (
    traceIDKey  ctxKey = "trace_id"
    spanIDKey   ctxKey = "span_id"
    serviceKey  ctxKey = "service_name"
)

// Header names for HTTP and gRPC metadata
const (
    HeaderTraceID = "X-Trace-ID"
    HeaderSpanID  = "X-Span-ID"
    HeaderParent  = "X-Parent-Span-ID"
)

// NewTraceID generates a new unique trace ID.
func NewTraceID() string {
    return uuid.New().String()
}

// NewSpanID generates a new unique span ID.
func NewSpanID() string {
    return uuid.New().String()
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext extracts the trace ID from context. Returns empty string if not set.
func TraceIDFromContext(ctx context.Context) string {
    if v, ok := ctx.Value(traceIDKey).(string); ok {
        return v
    }
    return ""
}

// WithSpanID adds a span ID to the context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
    return context.WithValue(ctx, spanIDKey, spanID)
}

// SpanIDFromContext extracts the span ID from context. Returns empty string if not set.
func SpanIDFromContext(ctx context.Context) string {
    if v, ok := ctx.Value(spanIDKey).(string); ok {
        return v
    }
    return ""
}
```

#### 6.1.2 `tracing/middleware.go` — Gateway Gin Middleware

```go
package tracing

import (
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"
)

// GinMiddleware is the entry point for all tracing.
// It reads an existing X-Trace-ID from the request header, or generates a new one.
// It adds the Trace ID to the Gin context so all downstream handlers can access it.
// It also sets the X-Trace-ID response header so the client can reference it.
func GinMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Respect an incoming Trace ID (e.g., from a load balancer or frontend).
        // If none exists, generate a new one.
        traceID := c.GetHeader(HeaderTraceID)
        if traceID == "" {
            traceID = NewTraceID()
        }

        spanID := NewSpanID()

        // Store in Gin context (accessible by handlers via c.GetString)
        c.Set(string(traceIDKey), traceID)
        c.Set(string(spanIDKey), spanID)

        // Store in Go context (accessible by gRPC clients that read context)
        ctx := WithTraceID(c.Request.Context(), traceID)
        ctx = WithSpanID(ctx, spanID)
        c.Request = c.Request.WithContext(ctx)

        // Return the Trace ID to the client in the response header.
        // This lets the client include it in bug reports.
        c.Header(HeaderTraceID, traceID)

        // Add trace fields to the request logger
        // This makes every log in this request include the trace ID automatically.
        zap.L().Debug("request traced",
            zap.String("trace_id", traceID),
            zap.String("span_id", spanID),
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL.Path),
        )

        c.Next()
    }
}
```

#### 6.1.3 `tracing/grpc.go` — gRPC Interceptors

```go
package tracing

import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
    "go.uber.org/zap"
)

// UnaryClientInterceptor automatically propagates trace context from the
// Go context into gRPC outgoing metadata. Install this on every gRPC client.
//
// Usage:
//   grpc.NewClient(addr, grpc.WithUnaryInterceptor(tracing.UnaryClientInterceptor()))
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
    return func(
        ctx context.Context,
        method string,
        req, reply interface{},
        cc *grpc.ClientConn,
        invoker grpc.UnaryInvoker,
        opts ...grpc.CallOption,
    ) error {
        // Extract trace context and inject into gRPC metadata
        traceID := TraceIDFromContext(ctx)
        spanID := TraceIDFromContext(ctx)
        if traceID != "" {
            ctx = metadata.AppendToOutgoingContext(ctx,
                HeaderTraceID, traceID,
                HeaderSpanID, spanID,
            )
        }
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// UnaryServerInterceptor extracts trace context from incoming gRPC metadata
// and places it into the Go context. Install this on every gRPC server.
//
// Usage:
//   grpc.NewServer(grpc.UnaryInterceptor(tracing.UnaryServerInterceptor("service-name")))
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        // Extract from incoming metadata
        md, ok := metadata.FromIncomingContext(ctx)
        if ok {
            if vals := md.Get(HeaderTraceID); len(vals) > 0 {
                ctx = WithTraceID(ctx, vals[0])
            }
            if vals := md.Get(HeaderSpanID); len(vals) > 0 {
                ctx = WithSpanID(ctx, vals[0])
            }
        }

        traceID := TraceIDFromContext(ctx)
        zap.L().Debug("gRPC request received",
            zap.String("trace_id", traceID),
            zap.String("service", serviceName),
            zap.String("method", info.FullMethod),
        )

        return handler(ctx, req)
    }
}
```

#### 6.1.4 `tracing/rabbitmq.go` — RabbitMQ Header Propagation

```go
package tracing

import "github.com/streadway/amqp"

// InjectToAMQPHeaders writes trace context into RabbitMQ message headers.
// Call this before publishing a message.
//
// Usage:
//   headers := tracing.InjectToAMQPHeaders(ctx, nil)
//   ch.Publish(exchange, key, false, false, amqp.Publishing{
//       Headers: headers,
//       Body:    messageBytes,
//   })
func InjectToAMQPHeaders(traceID, spanID string, existing amqp.Table) amqp.Table {
    if existing == nil {
        existing = amqp.Table{}
    }
    if traceID != "" {
        existing[HeaderTraceID] = traceID
    }
    if spanID != "" {
        existing[HeaderSpanID] = spanID
    }
    return existing
}

// ExtractFromAMQPHeaders reads trace context from RabbitMQ message headers.
// Call this in your consumer when you receive a message.
//
// Usage:
//   traceID, spanID := tracing.ExtractFromAMQPHeaders(delivery.Headers)
func ExtractFromAMQPHeaders(headers amqp.Table) (traceID string, spanID string) {
    if headers == nil {
        return "", ""
    }
    if v, ok := headers[HeaderTraceID]; ok {
        if s, ok := v.(string); ok {
            traceID = s
        }
    }
    if v, ok := headers[HeaderSpanID]; ok {
        if s, ok := v.(string); ok {
            spanID = s
        }
    }
    return traceID, spanID
}
```

### 6.2 Logger Integration

Modify the existing Zap logger to support trace-aware logging.

#### New helper in `logger/zaplogger.go`:

```go
// WithTraceContext returns a sugared logger with trace_id and span_id fields.
// Use this in any handler or consumer to get automatic trace fields in every log line.
//
// Usage:
//   log := logger.WithTraceContext(traceID, spanID, "blog-service")
//   log.Infow("blog published", "blog_id", id)
//
// Output:
//   {"ts":"2026-04-24T10:00:00Z","level":"info","msg":"blog published","trace_id":"abc-123","span_id":"def-456","service":"blog-service","blog_id":"xyz"}
func WithTraceContext(traceID, spanID, service string) *zap.SugaredLogger {
    return zap.S().With(
        "trace_id", traceID,
        "span_id", spanID,
        "service", service,
    )
}
```

### 6.3 What Changes in Each Service

#### Gateway (`the_monkeys_gateway`)

| File | Change |
|------|--------|
| `main.go` | Add `tracing.GinMiddleware()` to the middleware chain (before routes) |
| `internal/auth/routes.go` | Replace `context.Background()` with `c.Request.Context()` in gRPC calls |
| `internal/blog/routes.go` | Replace `context.Background()` with `c.Request.Context()` in gRPC calls |
| `internal/user_service/routes.go` | Replace `context.Background()` with `c.Request.Context()` in gRPC calls |
| `internal/activity/routes.go` | Replace `context.Background()` with `c.Request.Context()` in gRPC calls |
| All gRPC client creation | Add `grpc.WithUnaryInterceptor(tracing.UnaryClientInterceptor())` |

#### Backend Services (Authz, Blog, Users, Activity, Notification, Storage)

| File | Change |
|------|--------|
| `main.go` | Add `grpc.UnaryInterceptor(tracing.UnaryServerInterceptor("service-name"))` to gRPC server |
| `internal/consumer/*.go` | Extract trace headers from RabbitMQ message: `traceID, spanID := tracing.ExtractFromAMQPHeaders(delivery.Headers)` |
| All service methods | Use `logger.WithTraceContext(traceID, spanID, "service-name")` for logging |

#### RabbitMQ Publisher (Users Service & Others)

| File | Change |
|------|--------|
| `internal/services/service.go` | Pass `amqp.Table` headers with trace context when publishing messages |
| `rabbitmq/rabbitmq.go` | Update `PublishMessage` and `PublishReliable` to accept optional `amqp.Table` headers |

---

## 7. High-Level Design

### 7.1 Architecture Diagram — Phase 1 (Correlation IDs)

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLIENT (Browser)                           │
│                                                                     │
│   Sends request → receives X-Trace-ID in response header            │
│   Can include X-Trace-ID in bug reports                             │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     GATEWAY (Gin HTTP Server)                       │
│                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │  tracing.GinMiddleware()                                     │   │
│   │  • Generate/extract X-Trace-ID                               │   │
│   │  • Generate X-Span-ID                                        │   │
│   │  • Store in context                                          │   │
│   │  • Set response header                                       │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│   Every log line now includes: trace_id=abc-123                     │
│                                                                     │
│   gRPC calls use c.Request.Context() instead of context.Background()│
│   Interceptor auto-injects X-Trace-ID into gRPC metadata           │
└──────┬───────────────┬───────────────┬──────────────────────────────┘
       │               │               │
       ▼               ▼               ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│  Authz Svc  │ │  Blog Svc   │ │  User Svc   │
│  (gRPC)     │ │  (gRPC)     │ │  (gRPC)     │
│             │ │             │ │             │
│ Interceptor │ │ Interceptor │ │ Interceptor │
│ extracts    │ │ extracts    │ │ extracts    │
│ trace_id    │ │ trace_id    │ │ trace_id    │
│ from meta   │ │ from meta   │ │ from meta   │
│             │ │             │ │             │
│ All logs    │ │ All logs    │ │ All logs    │
│ include     │ │ include     │ │ include     │
│ trace_id    │ │ trace_id    │ │ trace_id    │
└─────────────┘ └──────┬──────┘ └──────┬──────┘
                       │               │
              Publishes to RabbitMQ    Publishes to RabbitMQ
              with X-Trace-ID in      with X-Trace-ID in
              message headers         message headers
                       │               │
                       ▼               ▼
              ┌─────────────────────────────┐
              │        RabbitMQ             │
              │  Messages carry trace       │
              │  headers in amqp.Table      │
              └──────┬──────┬──────┬───────┘
                     │      │      │
                     ▼      ▼      ▼
              ┌──────┐ ┌───────┐ ┌────────┐
              │Activ.│ │Notif. │ │Storage │
              │ Svc  │ │ Svc   │ │ Svc    │
              │      │ │       │ │        │
              │Extract│ │Extract│ │Extract │
              │trace  │ │trace  │ │trace   │
              │from   │ │from   │ │from    │
              │headers│ │headers│ │headers │
              │      │ │       │ │        │
              │Logs  │ │Logs   │ │Logs    │
              │with  │ │with   │ │with    │
              │trace │ │trace  │ │trace   │
              │_id   │ │_id    │ │_id     │
              └──────┘ └───────┘ └────────┘
```

### 7.2 Architecture Diagram — Phase 2 (OpenTelemetry + Jaeger)

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLIENT (Browser)                           │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     GATEWAY (Gin + OTel SDK)                        │
│  OTel HTTP middleware auto-creates spans                            │
└──────┬───────────────┬───────────────┬──────────────────────────────┘
       │               │               │
       ▼               ▼               ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│  Authz Svc  │ │  Blog Svc   │ │  User Svc   │
│ (OTel gRPC) │ │ (OTel gRPC) │ │ (OTel gRPC) │
└─────────────┘ └──────┬──────┘ └──────┬──────┘
                       │               │
                       ▼               ▼
              ┌─────────────────────────────┐
              │    RabbitMQ (trace headers)  │
              └──────┬──────┬──────┬───────┘
                     │      │      │
                     ▼      ▼      ▼
              ┌──────┐ ┌───────┐ ┌────────┐
              │Activ.│ │Notif. │ │Storage │
              └──┬───┘ └──┬────┘ └──┬─────┘
                 │        │         │
    All services export spans via OTLP protocol
                 │        │         │
                 ▼        ▼         ▼
     ┌──────────────────────────────────────┐
     │       OpenTelemetry Collector        │
     │   (Receives, processes, exports)     │
     └───────────────┬──────────────────────┘
                     │
                     ▼
     ┌──────────────────────────────────────┐
     │            Jaeger (All-in-One)       │
     │   • Stores traces                    │
     │   • Web UI at :16686                 │
     │   • Search by Trace ID               │
     │   • Flame graph visualization        │
     └──────────────────────────────────────┘
```

### 7.3 Docker Compose Changes (Phase 2 Only)

Phase 1 requires **zero changes** to docker-compose.yml.

Phase 2 adds two containers:

```yaml
# Only added in Phase 2
otel-collector:
  image: otel/opentelemetry-collector-contrib:latest
  container_name: "otel-collector"
  command: ["--config=/etc/otel-collector-config.yaml"]
  volumes:
    - ./config/otel-collector-config.yaml:/etc/otel-collector-config.yaml
  ports:
    - "4317:4317"   # OTLP gRPC receiver
    - "4318:4318"   # OTLP HTTP receiver
  networks:
    - monkeys-network
  restart: unless-stopped
  deploy:
    resources:
      limits:
        memory: 128M
        cpus: '0.3'

jaeger:
  image: jaegertracing/all-in-one:latest
  container_name: "jaeger"
  ports:
    - "16686:16686"  # Jaeger UI
    - "14268:14268"  # Jaeger collector HTTP
  networks:
    - monkeys-network
  environment:
    - COLLECTOR_OTLP_ENABLED=true
  restart: unless-stopped
  deploy:
    resources:
      limits:
        memory: 256M
        cpus: '0.5'
```

---

## 8. Implementation Plan

### Phase 1: Correlation IDs (Target: ~1 week)

#### Step 1.1 — Create the `tracing/` package (Day 1)

| Task | Details |
|------|---------|
| Create `tracing/context.go` | Trace ID generation, context helpers |
| Create `tracing/middleware.go` | Gin middleware |
| Create `tracing/grpc.go` | Client + server interceptors |
| Create `tracing/rabbitmq.go` | AMQP header inject/extract helpers |
| Write unit tests | Test generation, extraction, context round-trip |

**Acceptance criteria:** All tests pass. No external dependencies added (only `google/uuid` which already exists).

#### Step 1.2 — Update the Gateway (Day 2)

| Task | Details |
|------|---------|
| Add `tracing.GinMiddleware()` to `main.go` | Place it before route registration, after Recovery |
| Update all gRPC client constructors | Add `tracing.UnaryClientInterceptor()` |
| Replace `context.Background()` with `c.Request.Context()` | In all gateway handler files that make gRPC calls |

**Acceptance criteria:** Gateway generates Trace ID. Response headers include `X-Trace-ID`. Gateway logs include `trace_id`.

#### Step 1.3 — Update gRPC Services (Day 3-4)

| Service | Task |
|---------|------|
| `the_monkeys_authz` | Add server interceptor to `main.go`. Update service methods to log with trace ID. |
| `the_monkeys_blog` | Add server interceptor. Update service methods + consumer to extract trace from RabbitMQ. |
| `the_monkeys_users` | Add server interceptor. Update publisher to inject trace headers into RabbitMQ messages. |
| `the_monkeys_activity` | Add server interceptor. Update consumer to extract trace from RabbitMQ. |
| `the_monkeys_notification` | Add server interceptor. Update consumer to extract trace from RabbitMQ. |
| `the_monkeys_storage` | Add server interceptor. Update consumer to extract trace from RabbitMQ. |

**Acceptance criteria:** All gRPC services extract trace ID from metadata. All consumers extract trace ID from RabbitMQ headers. All logs include `trace_id`.

#### Step 1.4 — Update RabbitMQ Publisher (Day 4)

| Task | Details |
|------|---------|
| Update `rabbitmq/rabbitmq.go` | Add optional `amqp.Table` parameter to `PublishMessage` and `PublishReliable` |
| Update all publisher call sites | Pass trace headers when publishing |

**Backward compatibility:** The `amqp.Table` parameter is optional (nil means no headers). Existing callers continue to work without changes until updated.

#### Step 1.5 — Add logger helper (Day 1)

| Task | Details |
|------|---------|
| Add `WithTraceContext()` to `logger/zaplogger.go` | Returns a Zap sugared logger with trace_id and span_id fields |

#### Step 1.6 — Testing & Validation (Day 5)

| Task | Details |
|------|---------|
| End-to-end test | Publish a blog → verify Trace ID appears in Gateway, Blog, Activity, and Storage logs |
| Load test | Verify no measurable performance regression |
| Document | Update this doc with any changes discovered during implementation |

---

### Phase 2: OpenTelemetry + Jaeger (Target: ~2-3 weeks, starts after Phase 1 is stable)

#### Step 2.1 — Add Jaeger + OTel Collector to docker-compose (Day 1)

Add the two new service definitions shown in Section 7.3.

#### Step 2.2 — Add OTel SDK to Go services (Day 2-3)

| Task | Details |
|------|---------|
| Add `go.opentelemetry.io/otel` to go.mod | OTel SDK |
| Add `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` | Auto-instrumentation for Gin |
| Add `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | Auto-instrumentation for gRPC |
| Create `tracing/otel.go` | OTel SDK initialization (tracer provider, exporter, resource) |

#### Step 2.3 — Instrument services one by one (Day 4-10)

Start with the Gateway, then one backend at a time. Each service gets:
1. OTel SDK initialization in `main.go`.
2. Gin/gRPC auto-instrumentation middleware.
3. Custom spans for RabbitMQ publish/consume.
4. Custom spans for database calls (optional, high value).

#### Step 2.4 — Validate in Jaeger UI (Day 11-12)

- Verify traces appear in Jaeger at `http://localhost:16686`.
- Verify parent-child relationships are correct.
- Verify RabbitMQ async spans appear as linked traces.

#### Step 2.5 — Production deploy (Day 13-15)

- Deploy Jaeger + Collector in production docker-compose.
- Configure retention policies (e.g., keep traces for 7 days).
- Set sampling rate (start with 100%, reduce if needed).

---

## 9. Backward Compatibility

This is a critical requirement. Here is how each change is backward compatible:

| Change | Backward Compatible? | Why |
|--------|---------------------|-----|
| New `tracing/` package | ✅ Yes | New package — no existing code breaks |
| `GinMiddleware()` added to chain | ✅ Yes | Additive — adds headers, doesn't remove any |
| gRPC client interceptor | ✅ Yes | Adds metadata — servers that don't read it are unaffected |
| gRPC server interceptor | ✅ Yes | Reads metadata if present — works fine without it |
| RabbitMQ headers | ✅ Yes | AMQP headers are optional — consumers that don't read them are unaffected |
| `context.Background()` → `c.Request.Context()` | ✅ Yes | Strictly better — adds deadline and trace, no behavior change |
| Logger `WithTraceContext` | ✅ Yes | New function — existing logging untouched |
| Jaeger container (Phase 2) | ✅ Yes | New container — no existing container affected |

### Rollback Plan

- **Phase 1 rollback:** Remove the `tracing.GinMiddleware()` from the middleware chain. All other changes are passive (interceptors that read empty metadata are no-ops).
- **Phase 2 rollback:** Stop the Jaeger and Collector containers. Remove OTel SDK initialization. Services fall back to Phase 1 correlation IDs.

---

## 10. Risk & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| UUID generation overhead per request | Low | Low | UUID generation takes ~200ns. Negligible at our scale. |
| gRPC metadata adds latency | Low | Low | A few extra bytes per call. Measured in microseconds. |
| Trace ID missing in some logs | Medium | Medium | Add CI test that greps service logs for `trace_id` field. |
| Team unfamiliar with OTel (Phase 2) | Medium | Medium | Phase 1 builds familiarity. Pair programming for Phase 2. |
| Jaeger uses too much memory | Low | Medium | Set resource limits in docker-compose. Configure sampling rate and retention. |
| `context.Background()` replacement introduces bug | Low | High | Change one service at a time. Test thoroughly. Context propagation is strictly additive. |

---

## 11. Success Criteria

### Phase 1 — Done When:

- [ ] Every HTTP request to the Gateway generates/extracts a `X-Trace-ID`.
- [ ] The `X-Trace-ID` is present in the response header.
- [ ] All gRPC calls between services carry the Trace ID in metadata.
- [ ] All RabbitMQ messages carry the Trace ID in headers.
- [ ] Every log line from every service includes a `trace_id` field.
- [ ] Given a Trace ID, we can grep across all container logs and see the full request flow.
- [ ] No performance regression (measured by existing benchmarks or a simple load test).
- [ ] Zero breaking changes to existing APIs.

### Phase 2 — Done When:

- [ ] Jaeger UI shows complete traces for HTTP → gRPC → RabbitMQ flows.
- [ ] Parent-child span relationships are correct.
- [ ] Average trace latency overhead is < 1ms per request.
- [ ] Jaeger is accessible in production.
- [ ] Retention policy is configured (traces older than 7 days are deleted).

---

## 12. Open Questions

| # | Question | Who Decides | Status |
|---|----------|-------------|--------|
| 1 | Should the frontend include `X-Trace-ID` in requests (for end-to-end tracing from browser)? | Frontend Team | Open |
| 2 | Should we log the Trace ID in Elasticsearch activity events? (We already have `x_correlation_id` in the proto.) | Engineering | Open |
| 3 | What is the Trace ID retention period in Jaeger? 7 days? 30 days? | Ops/Engineering | Open |
| 4 | Should we expose the Jaeger UI publicly (behind auth) or keep it internal only? | Ops/Security | Open |
| 5 | For Phase 2, should we sample 100% of traces or use probabilistic sampling? | Engineering | Open |
| 6 | Should the `x_correlation_id` field in `gw_activity.proto` be repurposed as the Trace ID, or kept separate? | Engineering | Open |

---

## Appendix A: How To Search Logs After Phase 1

Once Phase 1 is deployed, debugging looks like this:

### Step 1: Get the Trace ID

The client (browser, curl, etc.) receives the Trace ID in the response header:

```
HTTP/1.1 500 Internal Server Error
X-Trace-ID: 8f14e45f-ceea-467f-a83a-0e7d5c26d5b4
```

### Step 2: Search all container logs

```bash
# Search across ALL containers for this trace
docker compose logs --no-color | grep "8f14e45f-ceea-467f-a83a-0e7d5c26d5b4"
```

**Output:**
```
the-monkeys-gateway  | {"ts":"2026-04-24T10:00:00Z","level":"info","msg":"request received","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4","method":"POST","path":"/api/v1/blog/publish/xyz"}
the-monkeys-auth     | {"ts":"2026-04-24T10:00:00Z","level":"info","msg":"access check","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4","account_id":"abc","access":"granted"}
the-monkeys-blog     | {"ts":"2026-04-24T10:00:01Z","level":"info","msg":"blog published","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4","blog_id":"xyz"}
the-monkeys-blog     | {"ts":"2026-04-24T10:00:01Z","level":"info","msg":"event published to rabbitmq","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4"}
the_monkeys_activity | {"ts":"2026-04-24T10:00:02Z","level":"info","msg":"activity logged","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4","event":"blog_publish"}
the_monkeys_storage  | {"ts":"2026-04-24T10:00:02Z","level":"info","msg":"files stored","trace_id":"8f14e45f-ceea-467f-a83a-0e7d5c26d5b4","blog_id":"xyz"}
```

You now see the full journey of the request in one command.

### Step 3: Filter by service (optional)

```bash
# Only show Blog service logs for this trace
docker compose logs the_monkeys_blog --no-color | grep "8f14e45f"
```

---

## Appendix B: Glossary

| Term | Definition |
|------|-----------|
| **AMQP** | Advanced Message Queuing Protocol — the protocol RabbitMQ uses |
| **CNCF** | Cloud Native Computing Foundation — the organization behind Kubernetes, OpenTelemetry, etc. |
| **DLQ** | Dead Letter Queue — where failed messages go for later inspection |
| **gRPC** | Google Remote Procedure Call — a fast, binary protocol for service-to-service communication |
| **Jaeger** | An open-source distributed tracing backend created by Uber |
| **OTel** | Short for OpenTelemetry |
| **OTLP** | OpenTelemetry Protocol — the standard protocol for exporting trace data |
| **Protobuf** | Protocol Buffers — the serialization format used by gRPC |
| **Span** | A single unit of work in a trace (e.g., one gRPC call, one DB query) |
| **Trace** | The complete journey of a request across all services |
| **Zap** | Uber's high-performance structured logging library for Go |

---

*End of document. Please review and provide feedback.*
