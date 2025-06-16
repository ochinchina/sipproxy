package main

import (
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

type HostIp struct {
	Name string
	Ip   string
}

type ViaConfig struct {
	Address string
	// must be tcp or udp
	Protocol string
	Port     int
}
type BackendConfig struct {
	// backend address
	Address string `yaml:"address,omitempty"`
	// local bind address to sending sip message to backend
	LocalAddress string `yaml:"localAddress,omitempty"`
}
type ListenConfig struct {
	Address  string
	Via      string `yaml:"via,omitempty"`
	TcpPort  int    `yaml:"tcp-port,omitempty"`
	UdpPort  int    `yaml:"udp-port,omitempty"`
	Backends []BackendConfig
}

type RedisAddress struct {
	// Redis address in format "host:port"
	// For example: "127.0.0.1:6379"
	Address string
	// Redis password
	// If not specified, the default value is empty string
	// If the Redis server does not require a password, leave it empty
	Password string `yaml:"password,omitempty"`
	// Redis database index
	Db int `yaml:"db,omitempty"`
}

type RedisSessionStore struct {
	// Redis addresses in the cluster
	Addresses []RedisAddress
	// Redis Channel for session events updates
	// If not specified, the default value is "sipproxy:session"
	Channel string `yaml:"channel,omitempty"`

	// Redis retry timeout in seconds
	// If not specified, the default value is 5 seconds
	RetryTimeout int `yaml:"retry-timeout,omitempty"`
}

// ProxyConfig is the configuration for a SIP proxy
type ProxyConfig struct {
	Name          string
	DialogTimeout int `yaml:"dialog-timeout,omitempty"`
	// Yes or True: keep the next hop route in the route header
	// No or False: remove the next hop route in the route header
	// If not specified, the default value is "yes"
	KeepNextHopRoute string `yaml:"keep-next-hop-route,omitempty"`
	NoReceived       bool   `yaml:"no-received,omitempty"`
	// True if the route must be recorded in the route header
	// False: no record-route will be added to the header if there is any record-route in the header
	// If not specified, the route must be recorded in the route header
	MustRecordRoute   bool               `yaml:"must-record-route,omitempty"`
	RedisSessionStore *RedisSessionStore `yaml:"redis-session-store,omitempty"`
	// The listens is a list of listen configurations
	Listens []ListenConfig

	// The route is a list of destination and next hop
	// The destination is a regular expression
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
	// Global hosts IPs, used for resolving host names in the SIP messages
	Hosts []HostIp
}

func init() {
}

func (vc *ViaConfig) String() string {
	return fmt.Sprintf("%s://%s:%d", vc.Protocol, vc.Address, vc.Port)
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
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", fileName)
	}
	// Check if the file is a valid YAML file
	p := &ProxiesConfigure{}

	err = yaml.Unmarshal(data, p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration file %s: %w", fileName, err)
	}
	return p, err
	/*f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return loadConfigFromReader(f)*/

}

func toKeepNextHopRoute(s string) bool {

	possibleTrueValues := []string{"true", "yes", "1", "on", "t", "y"}

	if s == "" {
		s = os.Getenv("KEEP_NEXT_HOP_ROUTE")
	}
	return slices.Contains(possibleTrueValues, strings.ToLower(s))
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
	proxy := NewProxy(config.Name,
		int64(dialogTimeout),
		config.Listens,
		toKeepNextHopRoute(config.KeepNextHopRoute),
		preConfigRoute,
		resolver,
		selfLearnRoute,
		!config.NoReceived,
		config.MustRecordRoute,
		config.RedisSessionStore,
	)

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

