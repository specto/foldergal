package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func containsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			Logger.Printf("Detected dot: %s", name)
			return true
		}
	}
	return false
}

func validMediaFile(name string) bool {
	detectedMime, _ := mimetype.DetectFile(name)
	isMedia := false
	mimePrefixes := []string{"image", "video", "audio"}
	for mime := detectedMime; mime != nil; mime = mime.Parent() {
		for _, mimePrefix := range mimePrefixes {
			if strings.HasPrefix(mime.String(), mimePrefix) {
				isMedia = true
			}
		}
	}
	return isMedia
}

type filteredFile struct {
	http.File
}

type filteredFileSystem struct {
	http.FileSystem
}

func (fs filteredFileSystem) Open(name string) (http.File, error) {
	if containsDotFile(name) {
		return nil, os.ErrNotExist
	}

	file, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	return filteredFile{file}, err
}

func (f filteredFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := f.File.Readdir(n)
	for _, file := range files { // Filter out the dot files from listing
		if !containsDotFile(file.Name()) {
			fis = append(fis, file)
		}
	}
	return
}

var Logger *log.Logger
var root string
var prefix string

func main() {
	// Get current execution folder
	execFolder, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	execFolder = filepath.Dir(execFolder)

	// Environment variables
	defaultHost := getEnvWithDefault("FOLDERGAL_HOST", "localhost")
	defaultPort, _ := strconv.Atoi(getEnvWithDefault("FOLDERGAL_PORT", "8080"))

	defaultHome := getEnvWithDefault("FOLDERGAL_HOME", execFolder)
	defaultRoot := getEnvWithDefault("FOLDERGAL_ROOT", execFolder)
	defaultPrefix := getEnvWithDefault("FOLDERGAL_PREFIX", "")
	defaultCrt := getEnvWithDefault("FOLDERGAL_CRT", "")
	defaultKey := getEnvWithDefault("FOLDERGAL_KEY", "")
	defaultHttp2, _ := strconv.ParseBool(getEnvWithDefault("FOLDERGAL_HTTP2", ""))

	// Command line arguments
	host := *flag.String("host", defaultHost, "host address to bind to")
	port := *flag.Int("port", defaultPort, "port to run at")
	home := *flag.String("home", defaultHome, "home folder")
	root = *flag.String("root", defaultRoot, "root folder to serve files from")
	prefix = *flag.String("prefix", defaultPrefix, "path prefix e.g. http://localhost/PREFIX/other/stuff")
	tlsCrt := *flag.String("crt", defaultCrt, "certificate file for TLS")
	tlsKey := *flag.String("key", defaultKey, "key file for TLS")
	useHttp2 := *flag.Bool("http2", defaultHttp2, "enable HTTP/2")
	flag.Parse()

	// Check keys to enable tls
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
	Logger = log.New(logFile, "foldergal: ", log.Lshortfile|log.LstdFlags)
	//Logger.Printf("Env is: %v", os.Environ())
	Logger.Printf("Home folder is: %v", home)
	Logger.Printf("Root folder is: %v", root)

	// Routing
	httpmux := http.NewServeMux()
	fs := filteredFileSystem{http.Dir(root)}
	if prefix != "" {
		prefixPath := fmt.Sprintf("/%s/", prefix)
		Logger.Printf("Using prefix: %s", prefixPath)
		httpmux.Handle(prefixPath, http.StripPrefix(prefixPath, http.FileServer(fs)))
	}
	bind := fmt.Sprintf("%s:%d", host, port)
	httpmux.Handle("/", http.FileServer(fs))

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
			Logger.Print("Using HTTP/2")
			tlsConfig.NextProtos = []string{"h2"}
		} else {
			tlsConfig.NextProtos = []string{"http/1.1"}
		}
		srv := &http.Server{
			Addr:      bind,
			Handler:   httpmux,
			TLSConfig: tlsConfig,
		}
		Logger.Printf("Using certificate: %s and key: %s", tlsCrt, tlsKey)
		Logger.Printf("Running at https://%v", bind)
		srvErr = srv.ListenAndServeTLS(tlsCrt, tlsKey)
	} else {
		Logger.Printf("Running at http://%v", bind)
		srvErr = http.ListenAndServe(bind, httpmux)
	}
	defer log.Fatal(srvErr)
}
