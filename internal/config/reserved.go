// internal/config/reserved.go
package config

// ReservedEnvNames lists environment variables that airlock itself
// injects into the agent container. User-registered env secrets MUST
// NOT use these names; collisions are rejected at config load time.
//
// Keep in sync with the env block in internal/container/manager.go
// (BuildClaudeConfig).
var ReservedEnvNames = map[string]bool{
	"HTTP_PROXY":  true,
	"HTTPS_PROXY": true,
	"http_proxy":  true,
	"https_proxy": true,
	"NO_PROXY":    true,
	"LANG":        true,
}
