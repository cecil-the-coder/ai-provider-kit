module github.com/cecil-the-coder/ai-provider-kit/examples/streaming-chat

go 1.24.0

require (
	github.com/cecil-the-coder/ai-provider-kit v0.0.0
	github.com/cecil-the-coder/ai-provider-kit/examples/config v0.0.0-00010101000000-000000000000
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/cecil-the-coder/ai-provider-kit => ../..
	github.com/cecil-the-coder/ai-provider-kit/examples/config => ../config
)
