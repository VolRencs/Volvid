package bot

import (
	"fmt"
	"strings"

	app "YouTubeBuild/internal/app"
)

func progressBar(done, total int) string {
	const width = 10
	if total == 0 {
		return strings.Repeat("░", width)
	}
	filled := done * width / total
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

func depLine(dep app.DependencyInfo) string {
	meta := depMeta(dep)
	if !dep.Available {
		return fmt.Sprintf("• %s — <b>не найден</b> [%s]", escapeHTML(dep.Name), escapeHTML(meta))
	}

	label := dep.Version
	if strings.TrimSpace(label) == "" {
		label = "доступно"
	}
	return fmt.Sprintf("• %s — <code>%s</code> [%s]", escapeHTML(dep.Name), escapeHTML(label), escapeHTML(string(dep.Source)+", "+meta))
}

func accessLine(name, status, detail string) string {
	status = strings.TrimSpace(status)
	detail = strings.TrimSpace(detail)
	if status == "" || status == "browser not found" || status == "not found" {
		return fmt.Sprintf("• %s — <b>не активно</b>", escapeHTML(name))
	}
	if status != "active" {
		return fmt.Sprintf("• %s — <b>%s</b>", escapeHTML(name), escapeHTML(status))
	}
	if detail == "" {
		return fmt.Sprintf("• %s — <b>активно</b>", escapeHTML(name))
	}
	return fmt.Sprintf("• %s — <code>%s</code> [%s]", escapeHTML(name), escapeHTML(detail), escapeHTML(status))
}

func cookiesDetail(info app.BrowserCookiesInfo) string {
	if strings.TrimSpace(info.Browser) == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(info.Browser), "firefox") {
		return info.Browser
	}
	if strings.TrimSpace(info.Profile) == "" {
		return info.Browser
	}
	return info.Browser + ":" + info.Profile
}

func runtimeDetail(info app.JSRuntimeInfo) string {
	return strings.TrimSpace(info.Name)
}

func depMeta(dep app.DependencyInfo) string {
	if dep.Required {
		return "обязательно"
	}
	return "опционально"
}

func verLine(name, ver string) string {
	if ver == "" {
		return fmt.Sprintf("• %s — <b>не найден</b>", escapeHTML(name))
	}
	return fmt.Sprintf("• %s — <code>%s</code>", escapeHTML(name), escapeHTML(ver))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
