package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
	"os"
	"runtime"
	"strings"
	"time"
)

type HostIp struct {
	Name string
	Ip   string
}

type ProxyConfig struct {
	Name    string
	Listens []struct {
		Address    string
		UDPPort    int      `yaml:"udp-port,omitempty"`
		TCPPort    int      `yaml:"tcp-port,omitempty"`
		Backends   []string `yaml:",omitempty"`
		Dests      []string `yaml:",omitempty"`
		NoReceived bool     `yaml:"no-received,omitempty"`
		defRoute   bool     `yaml:"def-route,omitempty"`
	}
	Route []struct {
		Dests    []string
		Protocol string
		NextHop  string
	}
	Hosts []HostIp
}

type ProxiesConfigure struct {
	Admin struct {
		Addr string
	}
	Proxies []ProxyConfig
	Hosts   []HostIp
}

func init() {
	log.SetOutput(os.Stdout)
	disableColors := (runtime.GOOS == "windows")

	log.SetFormatter(&log.TextFormatter{DisableColors: disableColors,
		FullTimestamp: true,
		ForceQuote:    false,
		DisableQuote:  isQuoteDisabled()})

	log.SetLevel(log.DebugLevel)
}

func isQuoteDisabled() bool {
	disableQuoteEnv := os.Getenv("DISABLE_QUOTE")

	return strings.EqualFold("true", disableQuoteEnv) || len(disableQuoteEnv) == 0
}

func initLog(logFile string, strLevel string, logSize int, backups int) {
	level, err := log.ParseLevel(strLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
	if len(logFile) <= 0 {
		log.SetOutput(os.Stdout)
	} else {
		log.SetFormatter(&log.TextFormatter{DisableColors: true,
			FullTimestamp: true,
			ForceQuote:    false,
			DisableQuote:  isQuoteDisabled()})
		log.SetOutput(&lumberjack.Logger{Filename: logFile,
			LocalTime:  true,
			MaxSize:    logSize,
			MaxBackups: backups})
	}
}

func loadConfig(fileName string) (*ProxiesConfigure, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	r := &ProxiesConfigure{}

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(r)

	if err != nil {
		return nil, err
	}

	return r, nil

}

func startProxies(c *cli.Context) error {
	config, err := loadConfig(c.String("config"))
	if err != nil {
		return err
	}
	strLevel := c.String("log-level")
	fileName := c.String("log-file")
	logSize := c.Int("log-size")
	backups := c.Int("log-backups")
	initLog(fileName, strLevel, logSize, backups)
	b, _ := yaml.Marshal(config)
	log.Debug("Success load configuration file:\n", string(b))
	for _, proxy := range config.Proxies {
		preConfigRoute := createPreConfigRoute(proxy)
		resolver := createPreConfigHostResolver(config.Hosts, proxy)
		log.Info("start sip proxy:", proxy.Name)
		err = startProxy(proxy, preConfigRoute, resolver)
		if err != nil {
			return err
		}
	}
	for {
		time.Sleep(time.Duration(5 * time.Second))
	}
}

func startProxy(config ProxyConfig, preConfigRoute *PreConfigRoute, resolver *PreConfigHostResolver) error {
	selfLearnRoute := NewSelfLearnRoute()
	proxy := NewProxy(config.Name, preConfigRoute, resolver, selfLearnRoute)
	for _, listen := range config.Listens {
		item, err := NewProxyItem(listen.Address,
			listen.UDPPort,
			listen.TCPPort,
			listen.Backends,
			listen.Dests,
			listen.defRoute,
			!listen.NoReceived,
			proxy,
			selfLearnRoute)
		if err != nil {
			log.Error("Fail to start proxy with error:", err)
			return err
		}
		proxy.AddItem(item)
	}
	err := proxy.Start()
	if err == nil {
		log.WithFields(log.Fields{"name": config.Name}).Info("Succeed to start proxy")
	} else {
		log.WithFields(log.Fields{"name": config.Name}).Error("Fail to start proxy")
	}
	return err
}

func createPreConfigRoute(config ProxyConfig) *PreConfigRoute {
	preConfigRoute := NewPreConfigRoute()
	for _, routeItem := range config.Route {
		for _, dest := range routeItem.Dests {
			preConfigRoute.AddRouteItem(routeItem.Protocol, dest, routeItem.NextHop)
		}

	}
	return preConfigRoute
}

func createPreConfigHostResolver(globalHostIPs []HostIp, config ProxyConfig) *PreConfigHostResolver {
	resolver := NewPreConfigHostResolver()
	for _, hostInfo := range globalHostIPs {
		resolver.AddHostIP(hostInfo.Name, hostInfo.Ip)
	}
	for _, hostInfo := range config.Hosts {
		resolver.AddHostIP(hostInfo.Name, hostInfo.Ip)
	}
	return resolver
}
func main() {
	app := &cli.App{
		Name:  "sipproxy",
		Usage: "a sip proxy in golang",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "Load configuration from `FILE`",
			},
			&cli.StringFlag{
				Name:  "log-file",
				Usage: "log file name",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "one of following level: Trace, Debug, Info, Warn, Error, Fatal, Panic",
			},
			&cli.IntFlag{
				Name:  "log-size",
				Usage: "size of log file in Megabytes",
				Value: 50,
			},
			&cli.IntFlag{
				Name:  "log-backups",
				Usage: "number of log rotate files",
				Value: 10,
			},
		},
		Action: startProxies,
	}
	err := app.Run(os.Args)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Fail to start application")
	}
}
