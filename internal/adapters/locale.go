package adapters

import (
	"os"
	"path/filepath"
	"volvid/internal/core"
)

const localeFileName = ".volvid_locale"

func localePath(env *Env) string {
	return filepath.Join(env.ConfigDir, localeFileName)
}

func LoadLocale(env *Env) core.Locale {
	b, err := os.ReadFile(localePath(env))
	if err != nil {
		return core.LocaleEN
	}
	return core.ParseLocale(string(b))
}

func SaveLocale(env *Env, l core.Locale) error {
	return writeAppConfig(localePath(env), l.String()+"\n")
}
