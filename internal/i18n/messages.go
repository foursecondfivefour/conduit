package i18n

import (
	"fmt"
	"strings"
)

// Lang is a supported UI language code.
type Lang string

const (
	LangRU Lang = "ru"
	LangEN Lang = "en"
)

func ParseLang(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "en", "english":
		return LangEN
	default:
		return LangRU
	}
}

func (l Lang) String() string {
	return string(l)
}

var messages = map[Lang]map[string]string{
	LangRU: {
		"tray.status":           "Прокси %s (:%d) | %s | %s DoH | %s",
		"tray.status.stopped":   "остановлен",
		"tray.status.running":   "работает",
		"tray.status.health.ok": "OK",
		"tray.status.health.fail": "FAIL",
		"tray.status.health.unknown": "—",
		"tray.strategy":         "Стратегия DPI",
		"tray.dns":              "DNS",
		"tray.domains":          "Домены",
		"tray.domains.youtube":  "Только YouTube",
		"tray.domains.google":   "Google media",
		"tray.domains.custom":   "Custom",
		"tray.domains.reset":    "Сбросить custom",
		"tray.options":          "Параметры",
		"tray.autostart":        "Запускать с Windows",
		"tray.system_proxy":     "Системный прокси",
		"tray.system_proxy.warning.title": "Системный прокси",
		"tray.system_proxy.warning": "Локальный прокси Conduit без аутентификации будет использоваться другими приложениями на этом компьютере в пределах allowlist. Включайте только если понимаете риск.",
		"tray.minimize_tray":    "Сворачивать в трей при закрытии",
		"tray.dns_flush":        "Сброс DNS кэша",
		"tray.connection_test":  "Проверить соединение",
		"tray.connection.ok":    "Соединение OK",
		"tray.connection.fail":  "Соединение не удалось: %s",
		"tray.updates":          "Обновления",
		"tray.updates.current":  "v%s — актуально",
		"tray.updates.available": "Доступно v%s",
		"tray.updates.downloading": "Загрузка %d%%",
		"tray.updates.error":    "Ошибка обновления",
		"tray.updates.check":    "Проверить сейчас",
		"tray.updates.install":  "Установить обновление",
		"tray.updates.ready.title": "Обновление готово",
		"tray.updates.ready.prompt": "Доступна версия %s. Установить сейчас?",
		"tray.updates.later":    "Напомнить позже",
		"tray.updates.auto":     "Автообновление",
		"tray.updates.browser":  "Открыть в браузере",
		"tray.language":         "Язык",
		"tray.language.ru":      "Русский",
		"tray.language.en":      "English",
		"tray.help":             "Обучение",
		"tray.devtools":       "Инструменты разработчика",
		"tray.reload_youtube": "Перезагрузить YouTube",
		"tray.open_settings":    "Открыть папку настроек",
		"tray.open_log":         "Открыть лог",
		"tray.quit":             "Выход",
		"strategy.none":         "Без фрагментации",
		"strategy.tcp_seg":      "TCP сегментация (2 байта)",
		"strategy.tcp_1":        "TCP сегментация (1 байт)",
		"strategy.tcp_8":        "TCP сегментация (8 байт)",
		"strategy.tls_frag":     "TLS record frag",
		"strategy.tls_frag_2":   "TLS record frag (2 байта)",
		"splash.tagline":        "Локальный прокси и просмотр YouTube",
		"splash.status":         "Запуск…",
		"loading.status":        "Загрузка YouTube…",
		"onboarding.title":      "Обучение — Conduit",
		"onboarding.welcome":    "Добро пожаловать",
	},
	LangEN: {
		"tray.status":           "Proxy %s (:%d) | %s | %s DoH | %s",
		"tray.status.stopped":   "stopped",
		"tray.status.running":   "running",
		"tray.status.health.ok": "OK",
		"tray.status.health.fail": "FAIL",
		"tray.status.health.unknown": "—",
		"tray.strategy":         "DPI strategy",
		"tray.dns":              "DNS",
		"tray.domains":          "Domains",
		"tray.domains.youtube":  "YouTube only",
		"tray.domains.google":   "Google media",
		"tray.domains.custom":   "Custom",
		"tray.domains.reset":    "Reset custom",
		"tray.options":          "Options",
		"tray.autostart":        "Start with Windows",
		"tray.system_proxy":     "System proxy",
		"tray.system_proxy.warning.title": "System proxy",
		"tray.system_proxy.warning": "Conduit runs an unauthenticated local proxy. Other apps on this PC may use it within the allowlist. Enable only if you understand the risk.",
		"tray.minimize_tray":    "Minimize to tray on close",
		"tray.dns_flush":        "Flush DNS cache",
		"tray.connection_test":  "Test connection",
		"tray.connection.ok":    "Connection OK",
		"tray.connection.fail":  "Connection failed: %s",
		"tray.updates":          "Updates",
		"tray.updates.current":  "v%s — up to date",
		"tray.updates.available": "v%s available",
		"tray.updates.downloading": "Downloading %d%%",
		"tray.updates.error":    "Update error",
		"tray.updates.check":    "Check now",
		"tray.updates.install":  "Install update",
		"tray.updates.ready.title": "Update ready",
		"tray.updates.ready.prompt": "Version %s is ready. Install now?",
		"tray.updates.later":    "Remind later",
		"tray.updates.auto":     "Auto-update",
		"tray.updates.browser":  "Open in browser",
		"tray.language":         "Language",
		"tray.language.ru":      "Русский",
		"tray.language.en":      "English",
		"tray.help":             "Tutorial",
		"tray.devtools":       "Developer tools",
		"tray.reload_youtube": "Reload YouTube",
		"tray.open_settings":    "Open settings folder",
		"tray.open_log":         "Open log",
		"tray.quit":             "Quit",
		"strategy.none":         "No fragmentation",
		"strategy.tcp_seg":      "TCP segmentation (2 bytes)",
		"strategy.tcp_1":        "TCP segmentation (1 byte)",
		"strategy.tcp_8":        "TCP segmentation (8 bytes)",
		"strategy.tls_frag":     "TLS record frag",
		"strategy.tls_frag_2":   "TLS record frag (2 bytes)",
		"splash.tagline":        "Local proxy and YouTube viewer",
		"splash.status":         "Starting…",
		"loading.status":        "Loading YouTube…",
		"onboarding.title":      "Tutorial — Conduit",
		"onboarding.welcome":    "Welcome",
	},
}

// T returns a localized string for the given language and key.
func T(lang Lang, key string) string {
	if table, ok := messages[lang]; ok {
		if s, ok := table[key]; ok {
			return s
		}
	}
	if table, ok := messages[LangEN]; ok {
		if s, ok := table[key]; ok {
			return s
		}
	}
	return key
}

// Tf returns a formatted localized string.
func Tf(lang Lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}
