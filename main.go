package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/exp/slog"
)

var (
	listenAddress = flag.String("web.listen-address", "",
		`Address to listen on for telemetry "(default: :9092)"`)
	metricsPath = flag.String("web.telemetry-path", "",
		"Path under which to expose metrics (default: /metrics)")
	logLevel            = flag.String("web.log-level", "", "Log level: info, debug, error, warning")
	traceEnabled        = flag.Bool("trace.enabled", false, "Set up Post endpoint for collecting traces")
	tracePath           = flag.String("trace.path", "/trace", "POST path to upload job proc info")
	traceRate           = flag.Uint64("trace.rate", 0, "number of seconds proc info should stay in memory before being marked as stale (default 10)")
	slurmPollLimit      = flag.Float64("slurm.poll-limit", 0, "throttle for slurmctld (default: 10s)")
	slurmSinfoOverride  = flag.String("slurm.sinfo-cli", "sinfo cli override", "")
	slurmSqueueOverride = flag.String("slurm.squeue-cli", "squeue cli override", "")
	logLevelMap         = map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
)

type CliOpts struct {
	sinfo  []string
	squeue []string
}

type TraceConfig struct {
	enabled       bool
	path          string
	rate          uint64
	sharedFetcher SlurmFetcher
}

type Config struct {
	traceConf     *TraceConfig
	pollLimit     float64
	logLevel      slog.Level
	listenAddress string
	metricsPath   string
	cliOpts       *CliOpts
}

func NewConfig() (*Config, error) {
	// defaults
	cliOpts := CliOpts{
		squeue: []string{"squeue", "--json"},
		sinfo:  []string{"sinfo", "--json"},
	}
	config := &Config{
		pollLimit:     10,
		logLevel:      slog.LevelInfo,
		listenAddress: ":9092",
		metricsPath:   "/metrics",
		traceConf: &TraceConfig{
			enabled: *traceEnabled,
			path:    *tracePath,
			rate:    *traceRate,
		},
		cliOpts: &cliOpts,
	}
	if lm, ok := os.LookupEnv("POLL_LIMIT"); ok {
		if limit, err := strconv.ParseFloat(lm, 64); err != nil {
			return nil, err
		} else {
			config.pollLimit = limit
		}
	}
	if *slurmPollLimit > 0 {
		config.pollLimit = *slurmPollLimit
	}
	if lvl, ok := os.LookupEnv("LOGLEVEL"); ok {
		config.logLevel = logLevelMap[strings.ToLower(lvl)]
	}
	if *logLevel != "" {
		config.logLevel = logLevelMap[*logLevel]
	}
	if *listenAddress != "" {
		config.listenAddress = *listenAddress
	}
	if *metricsPath != "" {
		fmt.Println(*metricsPath)
		config.metricsPath = *metricsPath
	}
	if *slurmSqueueOverride != "" {
		cliOpts.squeue = strings.Split(*slurmSqueueOverride, " ")
	}
	if *slurmSinfoOverride != "" {
		cliOpts.sinfo = strings.Split(*slurmSinfoOverride, " ")
	}
	fetcher := NewCliFetcher(cliOpts.squeue...)
	fetcher.cache = NewAtomicThrottledCache(config.pollLimit)
	config.SetFetcher(fetcher)
	return config, nil
}

func (c *Config) SetFetcher(fetcher SlurmFetcher) {
	c.traceConf.sharedFetcher = fetcher
}

func initPromServer(config *Config) http.Handler {
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.logLevel,
	})
	slog.SetDefault(slog.New(textHandler))
	prometheus.MustRegister(NewNodeCollecter(config), NewJobsController(config))
	if traceconf := config.traceConf; traceconf.enabled {
		slog.Info("trace path enabled at path: " + config.listenAddress + traceconf.path)
		traceController := NewTraceController(config)
		http.HandleFunc(traceconf.path, traceController.uploadTrace)
		prometheus.MustRegister(traceController)
	}
	return promhttp.Handler()
}

func main() {
	flag.Parse()
	config, err := NewConfig()
	if err != nil {
		log.Fatalf("config failed to load with error %q", err)
	}
	http.Handle(config.metricsPath, initPromServer(config))
	slog.Info("serving metrics at " + config.listenAddress + config.metricsPath)
	log.Fatalf("server exited with %q", http.ListenAndServe(config.listenAddress, nil))
}