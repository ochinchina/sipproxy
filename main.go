package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"
)

type HostIp struct {
	Name string
	Ip   string
}

type ProxyConfig struct {
	Name          string
	DialogTimeout int `yaml:"dialogTimeout,omitempty"`
	Listens       []struct {
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
}

func initLog(logFile string, logLevel string, logFormat string, logSize int, backups int) {
	var logEncoder zapcore.Encoder
	if strings.ToLower(logFormat) == "json" {
		logEncoder = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	} else {
		logEncoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	}
	level := zapcore.DebugLevel
	level.Set(logLevel)
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level
	})

	var out io.Writer = os.Stdout
	if len(logFile) > 0 {
		out = &lumberjack.Logger{Filename: logFile,
			LocalTime:  true,
			MaxSize:    logSize,
			MaxBackups: backups}
	}

	core := zapcore.NewCore(logEncoder, zapcore.AddSync(out), highPriority)
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)

}

func startProfiling(port int) {
	if port > 0 {
		go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	}
}

func loadConfigFromReader(reader io.Reader) (*ProxiesConfigure, error) {
	r := &ProxiesConfigure{}

	decoder := yaml.NewDecoder(reader)
	err := decoder.Decode(r)

	if err != nil {
		return nil, err
	}

	return r, nil

}

func loadConfig(fileName string) (*ProxiesConfigure, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return loadConfigFromReader(f)

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
	logFormat := c.String("log-format")
	profilingPort := c.Int("profiling-port")
	initLog(fileName, strLevel, logFormat, logSize, backups)
	startProfiling(profilingPort)

	b, _ := yaml.Marshal(config)
	zap.L().Debug("Success load configuration file", zap.String("config", string(b)))
	for _, proxy := range config.Proxies {
		preConfigRoute := createPreConfigRoute(proxy)
		resolver := createPreConfigHostResolver(config.Hosts, proxy)
		zap.L().Info("start sip proxy", zap.String("name", proxy.Name))
		err = startProxy(proxy, preConfigRoute, resolver)
		if err != nil {
			return err
		}
	}
	for {
		time.Sleep(time.Duration(5 * time.Second))
	}
}

func getDefaultDialogTimeout() int {
	expire, ok := os.LookupEnv("DEFAULT_DIALOG_TIMEOUT")
	if !ok {
		return 1200
	}
	if val, err := strconv.Atoi(expire); err == nil {
		return val
	}
	return 1200
}

func startProxy(config ProxyConfig, preConfigRoute *PreConfigRoute, resolver *PreConfigHostResolver) error {
	selfLearnRoute := NewSelfLearnRoute()
	dialogTimeout := config.DialogTimeout
	if dialogTimeout <= 0 {
		dialogTimeout = getDefaultDialogTimeout()
	}
	proxy := NewProxy(config.Name, int64(dialogTimeout), preConfigRoute, resolver, selfLearnRoute)
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
			zap.L().Error("Fail to start proxy with error", zap.String("error", err.Error()))
			return err
		}
		proxy.AddItem(item)
	}
	err := proxy.Start()
	if err == nil {
		zap.L().Info("Succeed to start proxy", zap.String("name", config.Name))
	} else {
		zap.L().Error("Fail to start proxy", zap.String("name", config.Name))
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
			&cli.StringFlag{
				Name:  "log-format",
				Usage: "must be one of: json, text",
				Value: "text",
			},
			&cli.IntFlag{
				Name:  "profiling-port",
				Usage: "the profiling port number",
				Value: 0,
			},
		},
		Action: startProxies,
	}
	err := app.Run(os.Args)
	if err != nil {
		zap.L().Error("Fail to start application", zap.String("error", err.Error()))
	}
}
