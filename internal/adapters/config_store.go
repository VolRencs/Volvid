package adapters

import (
	"os"
	"path/filepath"
	"strings"
)

const maxDotfileBytes = 8 << 10

func loadDotFile(env *Env, name string) string {
	b, err := os.ReadFile(filepath.Join(env.ConfigDir, name))
	if err != nil || len(b) > maxDotfileBytes {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func saveDotFile(env *Env, name, content string) error {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return writeAppConfig(filepath.Join(env.ConfigDir, name), content)
}

func loadValidDirDotFile(env *Env, name string) string {
	dir := cleanAbsPath(loadDotFile(env, name))
	if dir == "" {
		return ""
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
