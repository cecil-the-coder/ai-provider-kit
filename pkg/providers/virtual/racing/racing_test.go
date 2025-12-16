package racing

// This file previously contained all racing provider tests (~2930 lines).
// Tests have been refactored and split into multiple focused test files:
//
// - racing_helpers_test.go      - Shared mock implementations and helper functions
// - racing_unit_test.go          - Core racing provider unit tests and performance tracker tests
// - racing_strategy_test.go      - Strategy-specific tests (FirstWins, Weighted, Quality)
// - racing_streaming_test.go     - Racing stream tests
// - racing_virtualmodel_test.go  - Virtual model configuration and behavior tests
// - racing_config_test.go        - Configuration validation tests
// - racing_integration_test.go   - Integration tests (metrics, health checks, concurrent requests)
// - racing_compatibility_test.go - Backward compatibility tests
//
// This refactoring improves test organization and maintainability.
