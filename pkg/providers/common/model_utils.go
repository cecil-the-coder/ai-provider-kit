package common

// ResolveModel returns the model to use, with fallback priority:
// 1. Model specified in options
// 2. Default model from provider config
// 3. Provider's hardcoded default model
func ResolveModel(optionsModel, configDefaultModel, providerDefaultModel string) string {
	if optionsModel != "" {
		return optionsModel
	}
	if configDefaultModel != "" {
		return configDefaultModel
	}
	return providerDefaultModel
}
