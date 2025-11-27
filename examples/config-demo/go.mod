module github.com/cecil-the-coder/ai-provider-kit/examples/config-demo

go 1.24.0

replace github.com/cecil-the-coder/ai-provider-kit => ../..

replace github.com/cecil-the-coder/ai-provider-kit/examples/config => ../config

require (
	github.com/cecil-the-coder/ai-provider-kit v0.0.0-00010101000000-000000000000
	github.com/cecil-the-coder/ai-provider-kit/examples/config v0.0.0-00010101000000-000000000000
)

require gopkg.in/yaml.v3 v3.0.1 // indirect
