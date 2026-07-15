package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
)

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9101").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		fcgiURI       = kingpin.Flag("opcache.fcgi-uri", "Connection string to FastCGI server(s). Several URI can be provided, separated by semicolon.").Default("tcp://127.0.0.1:9000").String()
		scriptPath    = kingpin.Flag("opcache.script-path", "Path to PHP script which echoes json-encoded OPcache status").Default("").String()
		scriptDir     = kingpin.Flag("opcache.script-dir", "Path to directory where temporary PHP file will be created").Default("").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)

	if err := run(*listenAddress, *metricsPath, *fcgiURI, *scriptPath, *scriptDir); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}

func run(listenAddress, metricsPath, fcgiURI, scriptPath, scriptDir string) error {
	if len(scriptPath) == 0 {
		file, err := os.CreateTemp(scriptDir, "opcache.*.php")
		if err != nil {
			return err
		}

		file.Chmod(0644)

		payload := "<?php\necho(json_encode(opcache_get_status()));\n"
		_, err = file.WriteString(payload)
		if err != nil {
			return err
		}

		scriptPath = file.Name()

		defer file.Close()
		defer os.Remove(file.Name())
	}

	prometheus.MustRegister(version.NewCollector("opcache_exporter"))

	for _, uri := range strings.Split(fcgiURI, ";") {
		exporter, err := NewExporter(uri, scriptPath)
		if err != nil {
			return err
		}

		prometheus.MustRegister(exporter)
	}

	html := strings.Join([]string{
		`<html>`,
		`  <head>`,
		`    <title>OPcache Exporter</title>`,
		`  </head>`,
		`  <body>`,
		`    <h1>OPcache Exporter</h1>`,
		`    <p>`,
		`      <a href="` + metricsPath + `">Metrics</a>`,
		`    </p>`,
		`  </body>`,
		`</html>`,
	}, "\n")

	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	})

	return http.ListenAndServe(listenAddress, nil)
}
