package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/spf13/afero"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getEnvWithDefault(key string, defaultValue string) string {
	if envValue := os.Getenv(key); envValue != "" {
		return envValue
	} else {
		return defaultValue
	}
}

var logger *log.Logger
var root string
var prefix string
var rootFs afero.Fs
var cacheFs afero.Fs

func main() {
	// Get current execution folder
	execFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Environment variables
	defaultHost := getEnvWithDefault("FOLDERGAL_HOST", "localhost")
	defaultPort, _ := strconv.Atoi(getEnvWithDefault("FOLDERGAL_PORT", "8080"))
	defaultHome := getEnvWithDefault("FOLDERGAL_HOME", execFolder)
	defaultRoot := getEnvWithDefault("FOLDERGAL_ROOT", execFolder)
	defaultPrefix := getEnvWithDefault("FOLDERGAL_PREFIX", "")
	defaultCrt := getEnvWithDefault("FOLDERGAL_CRT", "")
	defaultKey := getEnvWithDefault("FOLDERGAL_KEY", "")
	defaultHttp2, _ := strconv.ParseBool(getEnvWithDefault("FOLDERGAL_HTTP2", ""))
	defaultCacheMinutes, _ := strconv.Atoi(getEnvWithDefault("FOLDERGAL_CACHE_MINUTES", "720"))

	// Command line arguments (they override env)
	host := *flag.String("host", defaultHost, "host address to bind to")
	port := *flag.Int("port", defaultPort, "port to run at")
	home := *flag.String("home", defaultHome, "home folder")
	root = *flag.String("root", defaultRoot, "root folder to serve files from")
	prefix = *flag.String("prefix", defaultPrefix, "path prefix as in http://localhost/PREFIX/other/stuff")
	tlsCrt := *flag.String("crt", defaultCrt, "certificate file for TLS")
	tlsKey := *flag.String("key", defaultKey, "key file for TLS")
	useHttp2 := *flag.Bool("http2", defaultHttp2, "enable HTTP/2 (only with TLS)")
	cacheMinutesInt := *flag.Int("cache-minutes", defaultCacheMinutes, "minutes to keep cached resources in memory")
	cacheMinutes := time.Duration(cacheMinutesInt)*time.Minute
	flag.Parse()

	// Check keys to enable TLS
	useTls := false
	if tlsCrt == "" {
		tlsCrt = filepath.Join(home, "tls/server.crt")
	}
	if tlsKey == "" {
		tlsKey = filepath.Join(home, "tls/server.key")
	}
	if fileExists(tlsCrt) && fileExists(tlsKey) {
		useTls = true
	}

	// Set up log file
	logFile, err := os.OpenFile(filepath.Join(home, "foldergal.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Print("Error: Log file cannot be created in home directory.")
		log.Fatal(err)
	}
	defer logFile.Close()
	logger = log.New(logFile, "foldergal: ", log.Lshortfile|log.LstdFlags)
	//logger.Printf("Env is: %v", os.Environ())
	logger.Printf("Home folder is: %v", home)
	logger.Printf("Root folder is: %v", root)

	// Set up caching folder
	logger.Printf("Setting cache timeout to: %d minutes", cacheMinutesInt)
	cacheFolder := filepath.Join(home, "cache")
	err = os.MkdirAll(cacheFolder, os.ModeDir)
	if err != nil {
		log.Fatal(err)
	} else {
		base := afero.NewBasePathFs(afero.NewOsFs(), cacheFolder)
		layer := afero.NewMemMapFs()
		cacheFs = afero.NewCacheOnReadFs(base, layer, cacheMinutes)
	}

	// Set root media folder
	//rootFs := filteredFileSystem{http.Dir(root)}
	base := afero.NewOsFs()
	layer := afero.NewMemMapFs()
	rootFs = afero.NewCacheOnReadFs(base, layer, cacheMinutes)
	afs := afero.NewHttpFs(rootFs)
	srvFs := filteredFileSystem{afs.Dir(root)}

	// Routing
	httpmux := http.NewServeMux()
	if prefix != "" {
		prefixPath := fmt.Sprintf("/%s/", prefix)
		logger.Printf("Using prefix: %s", prefixPath)
		httpmux.Handle(prefixPath, http.StripPrefix(prefixPath, http.FileServer(srvFs)))
	}
	bind := fmt.Sprintf("%s:%d", host, port)
	httpmux.Handle("/", http.FileServer(srvFs))

	// Server start sequence
	if port == 0 {
		log.Fatalf("Error: misconfigured port %d", port)
	}
	var srvErr error
	if useTls {
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(tlsCrt)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		if useHttp2 {
			logger.Print("Using HTTP/2")
			tlsConfig.NextProtos = []string{"h2"}
		} else {
			tlsConfig.NextProtos = []string{"http/1.1"}
		}
		srv := &http.Server{
			Addr:      bind,
			Handler:   httpmux,
			TLSConfig: tlsConfig,
		}
		logger.Printf("Using certificate: %s and key: %s", tlsCrt, tlsKey)
		logger.Printf("Running at https://%v", bind)
		srvErr = srv.ListenAndServeTLS(tlsCrt, tlsKey)
	} else {
		logger.Printf("Running at http://%v", bind)
		srvErr = http.ListenAndServe(bind, httpmux)
	}
	defer log.Fatal(srvErr)
}
