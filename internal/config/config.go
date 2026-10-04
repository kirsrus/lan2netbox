// Package config определяет структуру конфигурации приложения,
// значения по умолчанию, загрузку из YAML и проверку корректности.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DefaultPath — путь к файлу конфигурации по умолчанию.
const DefaultPath = "configs/config.yaml"

// Config — корневая структура конфигурации приложения.
type Config struct {
	Store   StoreConfig   `mapstructure:"store"`
	Sniffer SnifferConfig `mapstructure:"sniffer"`
	Probe   ProbeConfig   `mapstructure:"probe"`
	NetBox  NetBoxConfig  `mapstructure:"netbox"`
	Web     WebConfig     `mapstructure:"web"`
	Zabbix  ZabbixConfig  `mapstructure:"zabbix"`
	Logging LoggingConfig `mapstructure:"logging"`
}

// StoreConfig — локальная база данных (bbolt).
type StoreConfig struct {
	// Path — путь к файлу БД.
	Path string `mapstructure:"path"`
}

// SnifferConfig — обнаружение устройств (горутина sniffer).
type SnifferConfig struct {
	// Interface — сетевой интерфейс для захвата пакетов; пусто — автоопределение.
	Interface string `mapstructure:"interface"`
	// Subnets — подсети (CIDR) для активного ARP-скана; пусто — подсети интерфейса.
	Subnets []string `mapstructure:"subnets"`
	// ScanInterval — периодичность активного ARP-скана.
	ScanInterval time.Duration `mapstructure:"scan_interval"`
	// OfflineTimeout — время без активности, после которого устройство считается пропавшим.
	OfflineTimeout time.Duration `mapstructure:"offline_timeout"`
	// Capture — протоколы пассивного захвата: arp, dhcp, mdns.
	Capture []string `mapstructure:"capture"`
}

// ProbeConfig — опрос устройств (горутина probe).
type ProbeConfig struct {
	// Interval — период опроса доступных устройств.
	Interval time.Duration `mapstructure:"interval"`
	// RetryInitial — начальная задержка повторного опроса недоступного устройства.
	RetryInitial time.Duration `mapstructure:"retry_initial"`
	// RetryMax — максимальная задержка повторного опроса (backoff).
	RetryMax time.Duration `mapstructure:"retry_max"`
	// Plugins — плагины-опросчики.
	Plugins PluginsConfig `mapstructure:"plugins"`
}

// PluginsConfig — набор плагинов опроса.
type PluginsConfig struct {
	SNMP    SNMPConfig    `mapstructure:"snmp"`
	HTTP    HTTPConfig    `mapstructure:"http"`
	NETCONF NETCONFConfig `mapstructure:"netconf"`
}

// SNMPConfig — плагин SNMP (gosnmp).
type SNMPConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Community — community-строка для SNMPv1/2c.
	Community string `mapstructure:"community"`
	// Version — версия SNMP: 1, 2c или 3.
	Version string `mapstructure:"version"`
	// Username/Password — параметры аутентификации SNMPv3.
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	// Port — UDP-порт агента.
	Port int `mapstructure:"port"`
	// Timeout — таймаут запроса.
	Timeout time.Duration `mapstructure:"timeout"`
	// Retries — число повторов запроса.
	Retries int `mapstructure:"retries"`
}

// HTTPConfig — плагин опроса WEB-интерфейсов.
type HTTPConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Ports — порты для GET-запросов.
	Ports []int `mapstructure:"ports"`
	// Timeout — таймаут HTTP-запроса.
	Timeout time.Duration `mapstructure:"timeout"`
}

// NETCONFConfig — плагин NETCONF (заглушка).
type NETCONFConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Port — TCP-порт NETCONF.
	Port int `mapstructure:"port"`
}

// NetBoxConfig — синхронизация с NetBox (горутина netbox).
type NetBoxConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// URL — адрес NetBox API (например, https://netbox.example.com).
	URL string `mapstructure:"url"`
	// Token — API-токен NetBox.
	Token string `mapstructure:"token"`
	// SyncInterval — период принудительной синхронизации; 0 — только по событиям.
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	// AutoCreate — автоматическое создание отсутствующих устройств в NetBox.
	AutoCreate bool `mapstructure:"auto_create"`
	// InsecureTLS — отключить проверку TLS-сертификата.
	InsecureTLS bool `mapstructure:"insecure_tls"`
}

// WebConfig — web-интерфейс (горутина web).
type WebConfig struct {
	// Listen — адрес прослушивания, например 0.0.0.0:8080.
	Listen string `mapstructure:"listen"`
	// TemplatesDir — каталог go-шаблонов.
	TemplatesDir string `mapstructure:"templates_dir"`
	// StaticDir — каталог статических файлов.
	StaticDir string `mapstructure:"static_dir"`
}

// ZabbixConfig — синхронизация с Zabbix (горутина zabbix, заглушка).
type ZabbixConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// URL — адрес Zabbix API (заполняется при реализации интеграции).
	URL string `mapstructure:"url"`
}

// LoggingConfig — настройки логирования.
type LoggingConfig struct {
	// Level — debug | info | warn | error.
	Level string `mapstructure:"level"`
	// Format — text | json | auto (auto — автодетект терминала/systemd).
	Format string `mapstructure:"format"`
	// File — путь к файлу лога; пусто — stderr.
	File string `mapstructure:"file"`
}

// Default возвращает конфигурацию со значениями по умолчанию.
func Default() *Config {
	return &Config{
		Store: StoreConfig{Path: "data/lan2netbox.db"},
		Sniffer: SnifferConfig{
			Interface:      "",
			Subnets:        nil,
			ScanInterval:   60 * time.Second,
			OfflineTimeout: 10 * time.Minute,
			Capture:        []string{"arp", "dhcp", "mdns"},
		},
		Probe: ProbeConfig{
			Interval:     5 * time.Minute,
			RetryInitial: 30 * time.Second,
			RetryMax:     30 * time.Minute,
			Plugins: PluginsConfig{
				SNMP: SNMPConfig{
					Enabled:   true,
					Community: "public",
					Version:   "2c",
					Port:      161,
					Timeout:   3 * time.Second,
					Retries:   2,
				},
				HTTP: HTTPConfig{
					Enabled: true,
					Ports:   []int{80, 443, 8080, 8443},
					Timeout: 5 * time.Second,
				},
				NETCONF: NETCONFConfig{
					Enabled: false,
					Port:    830,
				},
			},
		},
		NetBox: NetBoxConfig{
			Enabled:      true,
			URL:          "",
			Token:        "",
			SyncInterval: 15 * time.Minute,
			AutoCreate:   true,
			InsecureTLS:  false,
		},
		Web: WebConfig{
			Listen:       "0.0.0.0:8080",
			TemplatesDir: "web/templates",
			StaticDir:    "web/static",
		},
		Zabbix: ZabbixConfig{
			Enabled: false,
			URL:     "",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "",
		},
	}
}

// Load читает YAML-конфигурацию из файла path, применяет значения
// по умолчанию для отсутствующих ключей и проверяет результат.
//
// Ошибка возвращается и при отсутствии файла, и при невалидной конфигурации.
func Load(path string) (*Config, error) {
	cfg := Default()

	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: чтение файла %s: %w", path, err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: разбор файла %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return cfg, nil
}

// Validate проверяет корректность конфигурации и возвращает
// агрегированную ошибку со всеми найденными проблемами.
func (c *Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Store.Path) == "" {
		errs = append(errs, errors.New("store.path: не может быть пустым"))
	}

	if c.Sniffer.ScanInterval <= 0 {
		errs = append(errs, errors.New("sniffer.scan_interval: должно быть больше нуля"))
	}
	if c.Sniffer.OfflineTimeout <= 0 {
		errs = append(errs, errors.New("sniffer.offline_timeout: должно быть больше нуля"))
	}
	for _, s := range c.Sniffer.Subnets {
		if _, _, err := net.ParseCIDR(s); err != nil {
			errs = append(errs, fmt.Errorf("sniffer.subnets: %q: %w", s, err))
		}
	}
	for _, p := range c.Sniffer.Capture {
		switch p {
		case "arp", "dhcp", "mdns":
		default:
			errs = append(errs, fmt.Errorf("sniffer.capture: неизвестный протокол %q (допустимо: arp, dhcp, mdns)", p))
		}
	}

	if c.Probe.Interval <= 0 {
		errs = append(errs, errors.New("probe.interval: должно быть больше нуля"))
	}
	if c.Probe.RetryInitial < 0 {
		errs = append(errs, errors.New("probe.retry_initial: не может быть отрицательным"))
	}
	if c.Probe.RetryMax < c.Probe.RetryInitial {
		errs = append(errs, errors.New("probe.retry_max: не может быть меньше probe.retry_initial"))
	}

	snmp := c.Probe.Plugins.SNMP
	if snmp.Enabled {
		switch snmp.Version {
		case "1", "2c", "3":
		default:
			errs = append(errs, fmt.Errorf("probe.plugins.snmp.version: неизвестная версия %q (допустимо: 1, 2c, 3)", snmp.Version))
		}
		if snmp.Version != "3" && snmp.Community == "" {
			errs = append(errs, errors.New("probe.plugins.snmp.community: не может быть пустой для SNMPv1/2c"))
		}
		if snmp.Port < 1 || snmp.Port > 65535 {
			errs = append(errs, fmt.Errorf("probe.plugins.snmp.port: недопустимый порт %d", snmp.Port))
		}
		if snmp.Timeout <= 0 {
			errs = append(errs, errors.New("probe.plugins.snmp.timeout: должно быть больше нуля"))
		}
	}

	httpPlugin := c.Probe.Plugins.HTTP
	if httpPlugin.Enabled {
		for _, p := range httpPlugin.Ports {
			if p < 1 || p > 65535 {
				errs = append(errs, fmt.Errorf("probe.plugins.http.ports: недопустимый порт %d", p))
			}
		}
		if httpPlugin.Timeout <= 0 {
			errs = append(errs, errors.New("probe.plugins.http.timeout: должно быть больше нуля"))
		}
	}

	if nc := c.Probe.Plugins.NETCONF; nc.Enabled {
		if nc.Port < 1 || nc.Port > 65535 {
			errs = append(errs, fmt.Errorf("probe.plugins.netconf.port: недопустимый порт %d", nc.Port))
		}
	}

	if c.NetBox.Enabled {
		u, err := url.Parse(c.NetBox.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("netbox.url: ожидается http(s)-адрес, получено %q", c.NetBox.URL))
		}
		if strings.TrimSpace(c.NetBox.Token) == "" {
			errs = append(errs, errors.New("netbox.token: не может быть пустым при netbox.enabled=true"))
		}
	}

	if strings.TrimSpace(c.Web.Listen) == "" {
		errs = append(errs, errors.New("web.listen: не может быть пустым"))
	}

	switch strings.ToLower(c.Logging.Level) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("logging.level: неизвестный уровень %q (допустимо: debug, info, warn, error)", c.Logging.Level))
	}

	switch strings.ToLower(c.Logging.Format) {
	case "", "auto", "text", "json":
	default:
		errs = append(errs, fmt.Errorf("logging.format: неизвестный формат %q (допустимо: text, json, auto)", c.Logging.Format))
	}

	return errors.Join(errs...)
}
