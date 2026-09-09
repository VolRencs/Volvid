package adapters

import "volvid/internal/core"

const localeFileName = ".volvid_locale"

func LoadLocale(env *Env) core.Locale {
	return core.ParseLocale(loadDotFile(env, localeFileName))
}

func SaveLocale(env *Env, l core.Locale) error {
	return saveDotFile(env, localeFileName, l.String())
}
