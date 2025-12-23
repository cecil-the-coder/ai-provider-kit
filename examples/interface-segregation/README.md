# Interface Segregation Examples

This directory demonstrates how to use the segregated provider interfaces for different use cases, following the **Interface Segregation Principle**.

## Overview

The Interface Segregation Principle (ISP) states that clients should not depend on interfaces they don't use. The AI Provider Kit follows this principle by defining smaller, focused interfaces that can be composed as needed:

- `CoreProvider` - Basic provider information
- `ModelProvider` - Model discovery functionality
- `ChatProvider` - Chat completion functionality
- `ToolCallingProvider` - Tool invocation functionality
- `HealthCheckProvider` - Health monitoring functionality
- `Provider` - Complete interface combining all above

## Examples in this File

### 1. ModelDiscoveryService
Demonstrates depending only on `ModelProvider` when you just need to discover available models.

```go
type ModelDiscoveryService struct {
    providers []ModelProvider
}
```

### 2. HealthMonitoringService
Demonstrates depending only on `HealthCheckProvider` when you only need to check health status.

```go
type HealthMonitoringService struct {
    providers []HealthCheckProvider
}
```

### 3. ChatService
Demonstrates depending only on `ChatProvider` when you only need chat completion.

```go
type ChatService struct {
    provider ChatProvider
}
```

### 4. ToolExecutionService
Demonstrates depending only on `ToolCallingProvider` when you only need tool invocation.

```go
type ToolExecutionService struct {
    provider ToolCallingProvider
}
```

### 5. ProviderInfoService
Demonstrates depending only on `CoreProvider` for basic provider information.

```go
type ProviderInfoService struct {
    providers []CoreProvider
}
```

### 6. MultiPurposeService
Demonstrates using the full `Provider` interface when you need comprehensive functionality.

```go
type MultiPurposeService struct {
    provider Provider
}
```

### 7. FlexibleProviderFactory
Demonstrates creating providers with specific interface requirements, showing how to depend only on needed interfaces.

## Benefits of Interface Segregation

1. **Loose Coupling**: Services depend only on the interfaces they actually use
2. **Easy Testing**: Mock implementations only need to implement relevant methods
3. **Clear Intent**: The interface a service depends on immediately reveals its requirements
4. **Flexibility**: Can swap implementations without affecting unrelated functionality

## Note

This file contains educational examples demonstrating interface usage patterns. The `FlexibleMockProvider` is a mock implementation for demonstration purposes only and should not be used in production.
