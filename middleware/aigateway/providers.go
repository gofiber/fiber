package aigateway

import (
	"strings"
)

// OpenAI returns an Upstream preset for the OpenAI API. Mount example:
//
//	app.Use("/openai", aigateway.New(aigateway.Config{
//	    PathPrefix: "/openai",
//	    Upstreams:  []aigateway.Upstream{aigateway.OpenAI(key)},
//	}))
//
// Clients then use base URL "https://gateway.example.com/openai/v1".
func OpenAI(key string) Upstream {
	return Upstream{
		Name: "openai",
		URL:  "https://api.openai.com",
		Auth: AuthBearer(),
		Key:  key,
	}
}

// Anthropic returns an Upstream preset for the Anthropic API. The key is
// injected via the x-api-key header. The anthropic-version header is not
// forced: pass-through clients (native SDKs) already send it. Set
// Upstream.Headers to pin one at the gateway.
func Anthropic(key string) Upstream {
	return Upstream{
		Name: "anthropic",
		URL:  "https://api.anthropic.com",
		Auth: AuthHeader("x-api-key"),
		Key:  key,
	}
}

// OpenRouter returns an Upstream preset for the OpenRouter API.
func OpenRouter(key string) Upstream {
	return Upstream{
		Name: "openrouter",
		URL:  "https://openrouter.ai/api",
		Auth: AuthBearer(),
		Key:  key,
	}
}

// AzureOpenAI returns an Upstream preset for an Azure OpenAI resource. The
// endpoint is the resource base URL, e.g. "https://my-resource.openai.azure.com".
// The api-version query parameter stays under client control (pass-through).
func AzureOpenAI(endpoint, key string) Upstream {
	return Upstream{
		Name: "azure-openai",
		URL:  strings.TrimRight(endpoint, "/"),
		Auth: AuthHeader("api-key"),
		Key:  key,
	}
}
