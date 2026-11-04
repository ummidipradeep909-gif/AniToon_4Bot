package secrets

import (
	"fmt"
	"os"
	"strings"
)

// Load resolves NAME_FILE before NAME, preserving legacy environment deployments.
// Values are trimmed once; errors never include secret contents or file paths.
func Load(name string, required bool) (string, error) {
	direct := strings.TrimSpace(os.Getenv(name))
	path := strings.TrimSpace(os.Getenv(name + "_FILE"))
	if path != "" && direct != "" {
		return "", fmt.Errorf("configure either %s or %s_FILE, not both", name, name)
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%s could not be read from its configured secret file", name)
		}
		direct = strings.TrimSpace(string(data))
	}
	if required && direct == "" {
		return "", fmt.Errorf("%s is required (set %s or %s_FILE)", name, name, name)
	}
	return direct, nil
}
