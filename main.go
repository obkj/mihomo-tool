package main

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type LogBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.lines = append(lb.lines, string(p))
	if len(lb.lines) > lb.max {
		lb.lines = lb.lines[len(lb.lines)-lb.max:]
	}
	return len(p), nil
}

func (lb *LogBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return strings.Join(lb.lines, "")
}

var (
	logBuffer = &LogBuffer{max: 100}
)

type TestProgress struct {
	Total    int    `json:"total"`
	Current  int    `json:"current"`
	Phase    string `json:"phase"` // "latency" or "speed"
	NodeName string `json:"node_name"`
	IsActive bool   `json:"is_active"`
}

var (
	testProgress     TestProgress
	testProgressLock sync.Mutex
)

type NodeResult struct {
	Name    string `json:"name"`
	Latency int    `json:"latency"`
	Speed   string `json:"speed"`
}

var (
	nodeResults     []NodeResult
	nodeResultsLock sync.Mutex
)

type Settings struct {
	UseSubscription  bool   `json:"use_subscription"`
	SubscriptionURL  string `json:"subscription_url"`
	ManualFrontProxy string `json:"manual_front_proxy"`
	Interval         int    `json:"interval"` // in minutes
	LandingProxy     string `json:"landing_proxy"`
	BestProxyName    string `json:"best_proxy_name"`
	BestProxySpeed   string `json:"best_proxy_speed"`
	LastUpdate       string `json:"last_update"`
	DownloadMirror   string `json:"download_mirror"`
	UseFallback      bool   `json:"use_fallback"`
	SpeedTestCount   int    `json:"speed_test_count"` // number of nodes to speed test, -1 for all
}

var (
	configPath   = "/data/config.yaml"
	settingsPath = "/data/settings.json"
	binPath      = "mihomo" // Base name, extension added in init()
	repo         = "MetaCubeX/mihomo"
	port         = ":58888"
	mihomoCmd    *exec.Cmd

	mihomoMutex  sync.Mutex
	settings     Settings
	settingsLock sync.Mutex

	lastSubscriptionProxies []map[string]interface{}
	proxiesLock             sync.Mutex

	isInstalling     bool
	isInstallingLock sync.Mutex

	latencyURL = "https://www.google.com/generate_204"

	speedURL = "https://dl.google.com/tag/s/appguid%3D%7B8A69D345-D564-463C-AFF1-A69D9E530F96%7D%26iid%3D%7B36F557D4-3061-6102-E389-EB8405FCE0E4%7D%26browser%3D4%26usagestats%3D0%26appname%3DGoogle%2520Chrome%26needsadmin%3Dtrue%26ap%3Dx64-stable-statsdef_0%26brand%3DGCEB/dl/chrome/install/GoogleChromeEnterpriseBundle64.zip"
)

func init() {
	if runtime.GOOS == "windows" {
		binPath = "mihomo.exe"
	} else {
		binPath = "mihomo"
	}
}

type Proxy struct {
	Name              string                 `yaml:"name"`
	Type              string                 `yaml:"type"`
	Server            string                 `yaml:"server"`
	Port              int                    `yaml:"port"`
	UDP               bool                   `yaml:"udp"`
	UUID              string                 `yaml:"uuid,omitempty"`
	Password          string                 `yaml:"password,omitempty"`
	Cipher            string                 `yaml:"cipher,omitempty"`
	AlterId           int                    `yaml:"alterId"`
	TLS               bool                   `yaml:"tls,omitempty"`
	Servername        string                 `yaml:"servername,omitempty"`
	SkipCertVerify    bool                   `yaml:"skip-cert-verify,omitempty"`
	Alpn              []string               `yaml:"alpn,omitempty"`
	Network           string                 `yaml:"network,omitempty"`
	Flow              string                 `yaml:"flow,omitempty"`
	ClientFingerprint string                 `yaml:"client-fingerprint,omitempty"`
	RealityOpts       map[string]interface{} `yaml:"reality-opts,omitempty"`
	WsOpts            map[string]interface{} `yaml:"ws-opts,omitempty"`
	GRPCOpts          map[string]interface{} `yaml:"grpc-opts,omitempty"`
	H2Opts            map[string]interface{} `yaml:"h2-opts,omitempty"`
	TCPOpts           map[string]interface{} `yaml:"tcp-opts,omitempty"`
	Plugin            string                 `yaml:"plugin,omitempty"`
	PluginOpts        map[string]interface{} `yaml:"plugin-opts,omitempty"`
	DialerProxy       string                 `yaml:"dialer-proxy,omitempty"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	log.SetOutput(io.MultiWriter(os.Stdout, logBuffer))

	// 让程序能够找到自己的静态资源文件 (index.html, css, js)
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("无法获取可执行文件路径: %v", err)
	}
	http.Handle("/", http.FileServer(http.Dir(filepath.Dir(exePath))))

	http.HandleFunc("/api/config", handleSaveConfig)
	http.HandleFunc("/api/kernel/install", handleInstallKernel)
	http.HandleFunc("/api/kernel/status", handleKernelStatus)
	http.HandleFunc("/api/service/start", handleStartService)
	http.HandleFunc("/api/service/stop", handleStopService)
	http.HandleFunc("/api/settings", handleSettings)
	http.HandleFunc("/api/subscription/update", handleManualUpdate)
	http.HandleFunc("/api/test/status", handleTestStatus)
	http.HandleFunc("/api/restart", handleRestart)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/config/raw", handleRawConfig)
	http.HandleFunc("/api/proxies/detailed", handleDetailedProxies)

	ensureFirewallRules()
	loadSettings()

	// Check if port is already in use
	portsToClear := []string{port, ":7890", ":7891", ":9090"}
	for _, p := range portsToClear {
		if isPortInUse(p) {
			log.Printf("Port %s is already in use. Attempting to clear processes...", p)
			if runtime.GOOS == "windows" {
				killProcessesByPort(p)
			}
		}
	}

	go startSubscriptionJob()

	srv := &http.Server{
		Addr:    port,
		Handler: nil,
	}

	// Channel to listen for signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Mihomo-Tool starting on http://localhost:58888")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-stop
	log.Println("Shutting down gracefully...")

	// Stop Mihomo child process
	stopMihomoInternal()

	// Context with timeout for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Mihomo-Tool stopped.")
}

func isPortInUse(addr string) bool {
	// Simple check by trying to listen
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

func handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.WriteFile(configPath, []byte(req.YAML), 0644); err != nil {
		sendResponse(w, "error", fmt.Sprintf("Failed to write config: %v", err))
		return
	}
	sendResponse(w, "success", "Configuration saved and applied")
	go restartMihomo()
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	go restartMihomo()
	sendResponse(w, "success", "Mihomo restarting")
}

func handleInstallKernel(w http.ResponseWriter, r *http.Request) {
	go func() {
		err := downloadAndInstall()
		if err != nil {
			log.Printf("Install failed: %v", err)
		}
	}()
	sendResponse(w, "success", "Installation started in background")
}

func handleKernelStatus(w http.ResponseWriter, r *http.Request) {
	status := "Not Installed"
	if _, err := os.Stat(binPath); err == nil {
		status = "Installed"
	}
	mihomoMutex.Lock()
	if mihomoCmd != nil && mihomoCmd.Process != nil {
		if mihomoCmd.ProcessState == nil || !mihomoCmd.ProcessState.Exited() {
			status = "Running"
		}
	}
	mihomoMutex.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func handleStartService(w http.ResponseWriter, r *http.Request) {
	isInstallingLock.Lock()
	if isInstalling {
		isInstallingLock.Unlock()
		sendResponse(w, "error", "Installation in progress")
		return
	}
	isInstallingLock.Unlock()

	mihomoMutex.Lock()
	defer mihomoMutex.Unlock()

	if mihomoCmd != nil && mihomoCmd.Process != nil && (mihomoCmd.ProcessState == nil || !mihomoCmd.ProcessState.Exited()) {
		sendResponse(w, "error", "Mihomo is already running")
		return
	}

	cmd := exec.Command("./"+binPath, "-f", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		sendResponse(w, "error", fmt.Sprintf("Failed to start Mihomo: %v", err))
		return
	}
	mihomoCmd = cmd
	sendResponse(w, "success", "Mihomo started")
}

func handleStopService(w http.ResponseWriter, r *http.Request) {
	mihomoMutex.Lock()
	defer mihomoMutex.Unlock()

	if mihomoCmd == nil || mihomoCmd.Process == nil {
		sendResponse(w, "error", "Mihomo is not running")
		return
	}

	// Try to kill, but don't error if it's already dead
	err := mihomoCmd.Process.Kill()
	if err != nil && !strings.Contains(err.Error(), "process already finished") && !strings.Contains(err.Error(), "Access is denied") {
		sendResponse(w, "error", fmt.Sprintf("Failed to stop Mihomo: %v", err))
		return
	}
	mihomoCmd = nil
	sendResponse(w, "success", "Mihomo stopped")
}

func downloadAndInstall() error {
	isInstallingLock.Lock()
	if isInstalling {
		isInstallingLock.Unlock()
		return fmt.Errorf("installation already in progress")
	}
	isInstalling = true
	isInstallingLock.Unlock()
	defer func() {
		isInstallingLock.Lock()
		isInstalling = false
		isInstallingLock.Unlock()
	}()

	// Stop Mihomo before installation to release file lock
	stopMihomoInternal()

	osType := runtime.GOOS
	arch := runtime.GOARCH

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	var downloadURL string
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		// Priority matching
		if strings.Contains(name, osType) && strings.Contains(name, arch) {
			if osType == "windows" && arch == "amd64" && !strings.Contains(name, "compatible") {
				continue
			}
			if strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".zip") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	if downloadURL == "" {
		for _, asset := range release.Assets {
			name := strings.ToLower(asset.Name)
			if strings.Contains(name, osType) && strings.Contains(name, arch) && (strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".zip")) {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find binary for %s-%s", osType, arch)
	}

	settingsLock.Lock()
	m := settings.DownloadMirror
	settingsLock.Unlock()
	if m == "" {
		m = "https://gh-proxy.org/" // Default if empty
	}

	fullDownloadURL := m + downloadURL
	log.Printf("Downloading %s (via mirror)...", fullDownloadURL)
	resp, err = http.Get(fullDownloadURL)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Ensure destination directory exists and is writable
	tempBin := binPath + ".tmp"
	if strings.HasSuffix(downloadURL, ".gz") {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gr.Close()
		out, err := os.OpenFile(tempBin, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer out.Close()
		io.Copy(out, gr)
	} else if strings.HasSuffix(downloadURL, ".zip") {
		zipFile := "mihomo.zip"
		out, _ := os.Create(zipFile)
		io.Copy(out, resp.Body)
		out.Close()
		unzip(zipFile, ".")
		os.Remove(zipFile)

		found := false
		files, _ := os.ReadDir(".")
		for _, f := range files {
			fname := strings.ToLower(f.Name())
			if strings.HasPrefix(fname, "mihomo") && (strings.HasSuffix(fname, ".exe") || osType != "windows") && !strings.Contains(fname, "mihomo-tool") {
				os.Rename(f.Name(), tempBin)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("could not find binary in zip")
		}
	}

	// Final swap
	if osType == "windows" {
		os.Remove(binPath) // Try to remove old one if it exists
	}
	if err := os.Rename(tempBin, binPath); err != nil {
		return fmt.Errorf("failed to replace binary: %v", err)
	}

	log.Printf("Installation complete.")
	return nil
}

func stopMihomoInternal() {
	mihomoMutex.Lock()
	defer mihomoMutex.Unlock()
	if mihomoCmd != nil && mihomoCmd.Process != nil {
		mihomoCmd.Process.Kill()
		mihomoCmd.Process.Wait()
		mihomoCmd = nil
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()
			io.Copy(f, rc)
		}
	}
	return nil
}

func sendResponse(w http.ResponseWriter, status, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Status: status, Message: message})
}

func loadSettings() {
	settingsLock.Lock()
	defer settingsLock.Unlock()
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		json.Unmarshal(data, &settings)
	}
	if settings.Interval <= 0 {
		settings.Interval = 60 // Default 1 hour
	}
}

func saveSettings() {
	settingsLock.Lock()
	defer settingsLock.Unlock()
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, data, 0644)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		settingsLock.Lock()
		json.NewEncoder(w).Encode(settings)
		settingsLock.Unlock()
		return
	}
	if r.Method == http.MethodPost {
		var req Settings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		settingsLock.Lock()
		settings.UseSubscription = req.UseSubscription
		settings.SubscriptionURL = req.SubscriptionURL
		settings.ManualFrontProxy = req.ManualFrontProxy
		settings.Interval = req.Interval
		settings.LandingProxy = req.LandingProxy
		settings.DownloadMirror = req.DownloadMirror
		settings.UseFallback = req.UseFallback
		settings.SpeedTestCount = req.SpeedTestCount
		settingsLock.Unlock()
		saveSettings()
		sendResponse(w, "success", "Settings saved")

		// If settings changed, we might want to regenerate config immediately
		go generateConfigAndRestart()
	}
}

func handleManualUpdate(w http.ResponseWriter, r *http.Request) {
	go updateSubscription()
	sendResponse(w, "success", "Update triggered")
}

func handleTestStatus(w http.ResponseWriter, r *http.Request) {
	testProgressLock.Lock()
	defer testProgressLock.Unlock()
	json.NewEncoder(w).Encode(testProgress)
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, logBuffer.String())
}

func handleRawConfig(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		http.Error(w, "Config not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func handleDetailedProxies(w http.ResponseWriter, r *http.Request) {
	nodeResultsLock.Lock()
	defer nodeResultsLock.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodeResults)
}

func startSubscriptionJob() {
	for {
		settingsLock.Lock()
		interval := settings.Interval
		settingsLock.Unlock()

		if interval <= 0 {
			interval = 60
		}
		time.Sleep(time.Duration(interval) * time.Minute)

		settingsLock.Lock()
		urlStr := settings.SubscriptionURL
		useSub := settings.UseSubscription
		settingsLock.Unlock()

		// Only update if in subscription mode and URL is set
		if useSub && urlStr != "" {
			updateSubscription()
		}
	}
}

func updateSubscription() {
	const maxRetries = 3
	settingsLock.Lock()
	urlStr := settings.SubscriptionURL
	useSub := settings.UseSubscription
	settingsLock.Unlock()

	if !useSub || urlStr == "" {
		generateConfigAndRestart()
		return
	}

	log.Printf("Updating subscription from %s", urlStr)

	var proxies []map[string]interface{}
	for i := 0; i < maxRetries; i++ {
		client := &http.Client{Timeout: 60 * time.Second}
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			time.Sleep(time.Second * 5) // Wait before retrying
			continue
		}
		req.Header.Set("User-Agent", "v2rayN 6.56.1")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Failed to download subscription: %v (Check network/proxy). Retry %d/%d", err, i+1, maxRetries)
			time.Sleep(time.Second * 5) // Wait before retrying
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Subscription server returned status: %d. Retry %d/%d", resp.StatusCode, i+1, maxRetries)
			time.Sleep(time.Second * 5) // Wait before retrying
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		proxies = parseSubscription(body)

		if len(proxies) > 0 {
			break // Success
		} else {
			log.Printf("No valid proxies found in attempt %d/%d", i+1, maxRetries)
		}
		time.Sleep(time.Second * 5) // Wait before retrying
	}

	bestProxy, speed := testProxies(proxies)

	// If testing fails, fall back to the first proxy in the list to ensure the subscription can still be updated.
	if bestProxy == "" && len(proxies) > 0 {
		log.Printf("Proxy test failed or returned no working nodes. Falling back to the first proxy in the subscription list.")
		bestProxy = proxies[0]["name"].(string)
		speed = "N/A (fallback)"
	}

	if bestProxy != "" { // This will be true if test succeeded OR if fallback was successful
		settingsLock.Lock()
		settings.BestProxyName = bestProxy
		settings.BestProxySpeed = speed
		settings.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
		settingsLock.Unlock()
		saveSettings()

		proxiesLock.Lock()
		lastSubscriptionProxies = proxies
		proxiesLock.Unlock()

		log.Printf("Best proxy selected: %s (%s)", bestProxy, speed)
		generateConfigAndRestart()
	} else {
		log.Printf("No proxies found in subscription to test or fall back to.")
	}
}

func testProxies(proxies []map[string]interface{}) (string, string) {
	log.Printf("Testing %d proxies for latency...", len(proxies))

	const maxTestRetries = 2
	// Create temp test config with all ports explicitly set to avoid 58888
	testConfig := make(map[string]interface{})
	testConfig["port"] = 10002
	testConfig["socks-port"] = 10003
	testConfig["mixed-port"] = 10004
	testConfig["http-port"] = 10005
	testConfig["redir-port"] = 10006
	testConfig["tproxy-port"] = 10007
	testConfig["external-controller"] = "127.0.0.1:19092"
	testConfig["mode"] = "global"
	testConfig["allow-lan"] = false
	testConfig["log-level"] = "error" // Reduce log noise
	testConfig["proxies"] = proxies
	testConfig["dns"] = map[string]interface{}{
		"enable":     true,
		"ipv6":       false,
		"nameserver": []string{"8.8.8.8", "1.1.1.1"},
	}

	testConfigPath := "test_config.yaml"
	data, _ := yaml.Marshal(testConfig)
	os.WriteFile(testConfigPath, data, 0644)
	defer os.Remove(testConfigPath)

	// Start temp Mihomo
	cmd := exec.Command("./"+binPath, "-f", testConfigPath)
	stderr, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[TestMihomo] %s", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start test Mihomo: %v", err)
		return "", ""
	}
	defer cmd.Process.Kill()

	// Wait for Mihomo to start and initialize control API
	time.Sleep(5 * time.Second)

	type LatencyResult struct {
		name    string
		latency int
	}
	resultsChan := make(chan LatencyResult, len(proxies))
	var wg sync.WaitGroup

	testProgressLock.Lock()
	testProgress = TestProgress{
		Total:    len(proxies),
		Current:  0,
		Phase:    "latency",
		IsActive: true,
	}
	testProgressLock.Unlock()

	// Worker pool to avoid overloading the local API
	sem := make(chan struct{}, 10)

	for _, p := range proxies {
		name, _ := p["name"].(string)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// API delay test
			apiURL := fmt.Sprintf("http://localhost:19092/proxies/%s/delay?timeout=5000&url=%s", url.PathEscape(name), url.QueryEscape(latencyURL))
			client := &http.Client{Timeout: 10 * time.Second}
			var resp *http.Response
			var err error

			for i := 0; i < maxTestRetries; i++ {
				resp, err = client.Get(apiURL)
				if err == nil {
					break // Success
				}
				log.Printf("Latency test failed for %s, retry %d/%d: %v", name, i+1, maxTestRetries, err)
			}

			defer resp.Body.Close()

			var result struct {
				Delay int    `json:"delay"`
				Msg   string `json:"message"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return
			}

			if result.Delay > 0 {
				resultsChan <- LatencyResult{name, result.Delay}
			}

			testProgressLock.Lock()
			testProgress.Current++
			testProgress.NodeName = name
			testProgressLock.Unlock()
		}(name)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var latencies []LatencyResult
	for r := range resultsChan {
		latencies = append(latencies, r)
	}

	if len(latencies) == 0 {
		log.Printf("No valid proxies found after latency test")
		testProgressLock.Lock()
		testProgress.IsActive = false
		testProgressLock.Unlock()
		return "", ""
	}

	// Sort by latency
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i].latency < latencies[j].latency
	})

	// Speed test limit
	settingsLock.Lock()
	limit := settings.SpeedTestCount
	settingsLock.Unlock()

	if limit == 0 {
		limit = 3 // Default
	}
	if limit == -1 || limit > len(latencies) {
		limit = len(latencies)
	}

	testProgressLock.Lock()
	testProgress.Phase = "speed"
	testProgress.Total = limit
	testProgress.Current = 0
	testProgressLock.Unlock()

	type SpeedResult struct {
		name  string
		bytes int64
	}
	var bestSpeedName string
	var maxBytes int64

	var speedResults []SpeedResult
	for i := 0; i < limit; i++ {
		name := latencies[i].name
		log.Printf("Speed testing %s...", name)

		testProgressLock.Lock()
		testProgress.Current = i + 1
		testProgress.NodeName = name
		testProgressLock.Unlock()

		// Set proxy to global
		putReq, _ := http.NewRequest("PUT", "http://localhost:19092/proxies/GLOBAL", strings.NewReader(fmt.Sprintf(`{"name": "%s"}`, name)))
		_, err := http.DefaultClient.Do(putReq)
		if err != nil {
			log.Printf("Failed to set global proxy for speed test: %v", err)
			continue
		}

		// Measure speed
		bytesRead := measureSpeedThroughProxy(10002)
		sFormatted := formatSpeed(bytesRead / 5)
		log.Printf("Proxy %s speed: %s/s", name, sFormatted)

		speedResults = append(speedResults, SpeedResult{name, bytesRead})

		if bytesRead > maxBytes {
			maxBytes = bytesRead
			bestSpeedName = name
		}
	}

	// Update nodeResults
	nodeResultsLock.Lock()
	nodeResults = nil
	for _, l := range latencies {
		nr := NodeResult{
			Name:    l.name,
			Latency: l.latency,
			Speed:   "",
		}
		for _, s := range speedResults {
			if s.name == l.name {
				nr.Speed = formatSpeed(s.bytes / 5)
				break
			}
		}
		nodeResults = append(nodeResults, nr)
	}
	nodeResultsLock.Unlock()

	testProgressLock.Lock()
	testProgress.IsActive = false
	testProgressLock.Unlock()

	if bestSpeedName == "" {
		return latencies[0].name, "N/A"
	}

	return bestSpeedName, formatSpeed(maxBytes / 5)
}

func measureSpeedThroughProxy(port int) int64 {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get(speedURL)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	// Measure for 5 seconds
	start := time.Now()
	var totalRead int64
	buf := make([]byte, 32*1024)

	for time.Since(start) < 5*time.Second {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			totalRead += int64(n)
		}
		if err != nil {
			break
		}
	}
	return totalRead
}

func formatSpeed(bytesPerSec int64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%d B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.2f KB/s", float64(bytesPerSec)/1024)
	} else {
		return fmt.Sprintf("%.2f MB/s", float64(bytesPerSec)/(1024*1024))
	}
}

func testProxyLatency(_ map[string]interface{}) int64 {
	// Not used anymore in favor of Mihomo API latency test
	return -1
}

func generateConfigAndRestart() {
	settingsLock.Lock()
	bestProxy := settings.BestProxyName
	useSub := settings.UseSubscription
	urlStr := settings.SubscriptionURL
	manualFront := settings.ManualFrontProxy
	landingLink := settings.LandingProxy
	useFallback := settings.UseFallback
	settingsLock.Unlock()

	var frontProxyRaw map[string]interface{}
	var subProxies []map[string]interface{}

	if useSub {
		if urlStr == "" {
			return
		}

		proxiesLock.Lock()
		subProxies = lastSubscriptionProxies
		proxiesLock.Unlock()

		if len(subProxies) == 0 {
			log.Printf("Proxy cache empty, triggering subscription update...")
			go updateSubscription()
			return
		}

		if !useFallback {
			if bestProxy == "" {
				frontProxyRaw = subProxies[0]
			} else {
				for _, p := range subProxies {
					if p["name"] == bestProxy {
						frontProxyRaw = p
						break
					}
				}
			}
		}
	} else {
		if manualFront == "" {
			return
		}
		frontProxyRaw = parseProxyURL(manualFront, "proxy-front")
	}

	landingProxyItemRaw := parseProxyURL(landingLink, "proxy-landing")
	if landingProxyItemRaw == nil {
		return
	}
	landingProxyItem := sanitizeProxy(landingProxyItemRaw)
	landingProxyItem.Name = "proxy-landing"

	var finalProxies []interface{}
	var proxyFrontGroup map[string]interface{}

	if useSub && useFallback {
		// Use nodeResults for sorting and renaming
		nodeResultsLock.Lock()
		results := make([]NodeResult, len(nodeResults))
		copy(results, nodeResults)
		nodeResultsLock.Unlock()

		// Sort results by speed (descending), then latency (ascending)
		sort.Slice(results, func(i, j int) bool {
			// Extract numeric value from speed string (e.g., "10.5 MB/s")
			getSpeedVal := func(s string) float64 {
				if s == "" || s == "N/A" {
					return -1
				}
				fields := strings.Fields(s)
				if len(fields) < 2 {
					return -1
				}
				val, _ := strconv.ParseFloat(fields[0], 64)
				unit := fields[1]
				switch unit {
				case "KB/s":
					val *= 1024
				case "MB/s":
					val *= 1024 * 1024
				case "GB/s":
					val *= 1024 * 1024 * 1024
				}
				return val
			}
			si := getSpeedVal(results[i].Speed)
			sj := getSpeedVal(results[j].Speed)
			if si != sj {
				return si > sj
			}
			// Same speed or no speed, sort by latency
			li := results[i].Latency
			lj := results[j].Latency
			if li <= 0 {
				li = 99999
			}
			if lj <= 0 {
				lj = 99999
			}
			return li < lj
		})

		// Add sorted and renamed subscription proxies
		var subProxyNames []string
		nameMap := make(map[string]string) // original name -> renamed name

		for _, res := range results {
			originalName := res.Name
			var p map[string]interface{}
			for _, sp := range subProxies {
				if sp["name"] == originalName {
					p = sp
					break
				}
			}
			if p == nil {
				continue
			}

			sanitized := sanitizeProxy(p)
			if sanitized != nil {
				// Rename node to include speed/latency
				info := ""
				if res.Speed != "" && res.Speed != "N/A" {
					info = res.Speed
				} else if res.Latency > 0 {
					info = fmt.Sprintf("%dms", res.Latency)
				}
				if info != "" {
					sanitized.Name = fmt.Sprintf("%s [%s]", originalName, info)
				}
				nameMap[originalName] = sanitized.Name

				finalProxies = append(finalProxies, sanitized)
				subProxyNames = append(subProxyNames, sanitized.Name)
			}
		}

		// If some proxies were not in nodeResults (should not happen normally but for safety)
		for _, p := range subProxies {
			originalName, _ := p["name"].(string)
			if _, exists := nameMap[originalName]; !exists {
				sanitized := sanitizeProxy(p)
				if sanitized != nil {
					finalProxies = append(finalProxies, sanitized)
					subProxyNames = append(subProxyNames, sanitized.Name)
				}
			}
		}

		// Create fallback group
		proxyFrontGroup = map[string]interface{}{
			"name":     "proxy-front",
			"type":     "fallback",
			"proxies":  subProxyNames,
			"url":      "https://www.google.com/generate_204",
			"interval": 60,
		}
	} else {
		if frontProxyRaw == nil {
			return
		}
		frontProxy := sanitizeProxy(frontProxyRaw)
		frontProxy.Name = "proxy-front"
		finalProxies = append(finalProxies, frontProxy)
	}

	// Landing proxy uses proxy-front as dialer
	landingProxyItem.DialerProxy = "proxy-front"
	finalProxies = append(finalProxies, landingProxyItem)

	var groups []interface{}
	if proxyFrontGroup != nil {
		groups = append(groups, proxyFrontGroup)
	}
	groups = append(groups, map[string]interface{}{
		"name":    "Relay-Chain",
		"type":    "select",
		"proxies": []string{"proxy-landing"},
	})

	type MihomoConfig struct {
		SocksPort          int           `yaml:"socks-port"`
		Port               int           `yaml:"port"`
		MixedPort          int           `yaml:"mixed-port"`
		AllowLan           bool          `yaml:"allow-lan"`
		LogLevel           string        `yaml:"log-level"`
		ExternalController string        `yaml:"external-controller"`
		Proxies            []interface{} `yaml:"proxies"`
		ProxyGroups        []interface{} `yaml:"proxy-groups"`
		Rules              []string      `yaml:"rules"`
	}

	resConfig := MihomoConfig{
		SocksPort:          7891,
		Port:               7890,
		MixedPort:          7892,
		AllowLan:           true,
		LogLevel:           "info",
		ExternalController: "127.0.0.1:9090",
		Proxies:            finalProxies,
		ProxyGroups:        groups,
		Rules: []string{
			"MATCH,Relay-Chain",
		},
	}

	out, _ := yaml.Marshal(resConfig)
	os.WriteFile(configPath, out, 0644)
	log.Printf("Successfully generated and saved config.yaml")

	restartMihomo()
}

func sanitizeProxy(p map[string]interface{}) *Proxy {
	if p == nil {
		return nil
	}

	res := &Proxy{}

	// Helper to get string
	getStr := func(key string) string {
		if val, ok := p[key].(string); ok {
			return val
		}
		return ""
	}

	// Helper to get int
	getInt := func(key string) int {
		if val, ok := p[key].(int); ok {
			return val
		}
		if val, ok := p[key].(float64); ok {
			return int(val)
		}
		return 0
	}

	// Helper to get bool
	getBool := func(key string) bool {
		if val, ok := p[key].(bool); ok {
			return val
		}
		return false
	}

	res.Name = getStr("name")
	res.Type = getStr("type")
	res.Server = getStr("server")
	res.Port = getInt("port")
	res.UDP = getBool("udp")
	res.UUID = getStr("uuid")
	res.Password = getStr("password")
	res.Cipher = getStr("cipher")
	res.AlterId = getInt("alterId")
	res.TLS = getBool("tls")
	res.Servername = getStr("servername")
	res.SkipCertVerify = getBool("skip-cert-verify")

	if val, ok := p["alpn"].([]string); ok {
		res.Alpn = val
	}

	res.Network = getStr("network")
	res.Flow = getStr("flow")
	res.ClientFingerprint = getStr("client-fingerprint")

	if val, ok := p["reality-opts"].(map[string]interface{}); ok {
		res.RealityOpts = val
	}
	if val, ok := p["ws-opts"].(map[string]interface{}); ok {
		res.WsOpts = val
	}
	if val, ok := p["grpc-opts"].(map[string]interface{}); ok {
		res.GRPCOpts = val
	}
	if val, ok := p["h2-opts"].(map[string]interface{}); ok {
		res.H2Opts = val
	}
	if val, ok := p["tcp-opts"].(map[string]interface{}); ok {
		res.TCPOpts = val
	}

	res.Plugin = getStr("plugin")
	if val, ok := p["plugin-opts"].(map[string]interface{}); ok {
		res.PluginOpts = val
	}

	res.DialerProxy = getStr("dialer-proxy")

	// Specific logic for VLESS
	if res.Type == "vless" {
		res.Cipher = ""
		res.AlterId = 0
	}

	return res
}

func parseSubscription(body []byte) []map[string]interface{} {
	log.Printf("Parsing subscription data (%d bytes)...", len(body))
	// 1. Try Clash YAML
	var clashConfig struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(body, &clashConfig); err == nil && len(clashConfig.Proxies) > 0 {
		log.Printf("Detected Clash YAML format, found %d proxies", len(clashConfig.Proxies))
		return clashConfig.Proxies
	}

	// 2. Try Base64 encoded link list
	content := string(body)
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content)); err == nil {
		log.Printf("Detected Base64 encoded subscription")
		content = string(decoded)
	} else {
		log.Printf("Attempting plaintext link list parsing")
	}

	// 3. Parse links line by line
	var proxies []map[string]interface{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		p := parseProxyURL(line, "")
		if p != nil {
			if p["name"] == "" {
				p["name"] = fmt.Sprintf("node-%d", len(proxies)+1)
			}
			proxies = append(proxies, p)
		}
	}
	log.Printf("Found %d valid proxies in link list", len(proxies))
	return proxies
}

func parseProxyURL(link string, name string) map[string]interface{} {
	link = strings.TrimSpace(link)
	if link == "" {
		return nil
	}

	log.Printf("Parsing proxy link: %s...", func() string {
		if len(link) > 50 {
			return link[:50] + "..."
		}
		return link
	}())

	// Extract name from #fragment if present and name is empty
	if name == "" {
		if idx := strings.LastIndex(link, "#"); idx != -1 {
			name, _ = url.PathUnescape(link[idx+1:])
			link = link[:idx]
			log.Printf("Extracted name from fragment: %s", name)
		}
	}

	u, err := url.Parse(link)
	if err != nil {
		return nil
	}

	proxy := make(map[string]interface{})
	proxy["name"] = name
	proxy["udp"] = true

	switch u.Scheme {
	case "ss":
		proxy["type"] = "ss"
		if u.User != nil {
			userData := u.User.String()
			decoded, _ := base64.StdEncoding.DecodeString(userData)
			parts := strings.Split(string(decoded), ":")
			if len(parts) == 2 {
				proxy["cipher"] = parts[0]
				proxy["password"] = parts[1]
			}
		}
		proxy["server"] = u.Hostname()
		portNum, _ := strconv.Atoi(u.Port())
		proxy["port"] = portNum

		q := u.Query()
		if pluginStr := q.Get("plugin"); pluginStr != "" {
			parts := strings.Split(pluginStr, ";")
			if len(parts) > 0 {
				proxy["plugin"] = parts[0]
				if len(parts) > 1 {
					opts := make(map[string]interface{})
					for _, part := range parts[1:] {
						kv := strings.SplitN(part, "=", 2)
						if len(kv) == 2 {
							opts[kv[0]] = kv[1]
						}
					}
					proxy["plugin-opts"] = opts
				}
			}
		}
	case "trojan":
		proxy["type"] = "trojan"
		proxy["password"] = u.User.Username()
		proxy["server"] = u.Hostname()
		portNum, _ := strconv.Atoi(u.Port())
		proxy["port"] = portNum
		proxy["servername"] = u.Query().Get("sni")
		if u.Query().Get("allowInsecure") == "1" || u.Query().Get("skip-cert-verify") == "true" {
			proxy["skip-cert-verify"] = true
		}
		if alpn := u.Query().Get("alpn"); alpn != "" {
			proxy["alpn"] = strings.Split(alpn, ",")
		}
		if network := u.Query().Get("type"); network == "ws" {
			proxy["network"] = "ws"
			proxy["ws-opts"] = map[string]interface{}{
				"path": u.Query().Get("path"),
				"headers": map[string]interface{}{
					"Host": u.Query().Get("host"),
				},
			}
		}
	case "vless":
		proxy["type"] = "vless"
		proxy["server"] = u.Hostname()
		portNum, _ := strconv.Atoi(u.Port())
		proxy["port"] = portNum
		proxy["uuid"] = u.User.Username()

		q := u.Query()
		proxy["servername"] = q.Get("sni")
		if q.Get("allowInsecure") == "1" || q.Get("skip-cert-verify") == "true" {
			proxy["skip-cert-verify"] = true
		}
		if alpn := q.Get("alpn"); alpn != "" {
			proxy["alpn"] = strings.Split(alpn, ",")
		}
		security := q.Get("security")
		switch security {
		case "reality":
			proxy["reality-opts"] = map[string]interface{}{
				"public-key": q.Get("pbk"),
				"short-id":   q.Get("sid"),
			}
			proxy["client-fingerprint"] = q.Get("fp")
		case "tls":
			proxy["tls"] = true
		}
		if q.Get("fp") != "" {
			proxy["client-fingerprint"] = q.Get("fp")
		}
		if q.Get("flow") != "" {
			flow := q.Get("flow")
			proxy["flow"] = flow
			if strings.Contains(flow, "vision") {
				proxy["tls"] = true
			}
		}

		network := q.Get("type")
		switch network {
		case "ws":
			proxy["network"] = "ws"
			proxy["ws-opts"] = map[string]interface{}{
				"path": q.Get("path"),
				"headers": map[string]interface{}{
					"Host": q.Get("host"),
				},
			}
		case "grpc":
			proxy["network"] = "grpc"
			proxy["grpc-opts"] = map[string]interface{}{
				"grpc-service-name": q.Get("serviceName"),
			}
		case "h2":
			proxy["network"] = "h2"
			proxy["h2-opts"] = map[string]interface{}{
				"host": []string{q.Get("host")},
				"path": q.Get("path"),
			}
		}
	case "vmess":
		proxy["type"] = "vmess"
		// vmess://base64(json)
		if u.Host != "" {
			data := u.Host + u.Path
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err == nil {
				var v struct {
					Add  string      `json:"add"`
					Port interface{} `json:"port"` // Can be string or int
					ID   string      `json:"id"`
					Aid  interface{} `json:"aid"` // Can be string or int
					Scy  string      `json:"scy"`
					Net  string      `json:"net"`
					Type string      `json:"type"`
					Host string      `json:"host"`
					Path string      `json:"path"`
					TLS  string      `json:"tls"`
					Sni  string      `json:"sni"`
					Ps   string      `json:"ps"`
				}
				if err := json.Unmarshal(decoded, &v); err == nil {
					finalName := name
					if finalName == "" {
						finalName = v.Ps
					}
					if finalName == "" {
						finalName = "vmess-node"
					}
					proxy["name"] = finalName
					proxy["server"] = v.Add
					var portNum int
					switch p := v.Port.(type) {
					case float64:
						portNum = int(p)
					case string:
						portNum, _ = strconv.Atoi(p)
					}
					proxy["port"] = portNum
					proxy["uuid"] = v.ID
					var aidNum int
					switch a := v.Aid.(type) {
					case float64:
						aidNum = int(a)
					case string:
						aidNum, _ = strconv.Atoi(a)
					}
					proxy["alterId"] = aidNum
					log.Printf("Parsed VMess: name=%s, server=%s, port=%d, uuid=%s, alterId=%d", proxy["name"], proxy["server"], proxy["port"], proxy["uuid"], proxy["alterId"])
					if v.Scy == "" {
						proxy["cipher"] = "auto"
					} else {
						proxy["cipher"] = v.Scy
					}
					if v.TLS == "tls" {
						proxy["tls"] = true
					}
					if v.Sni != "" {
						proxy["servername"] = v.Sni
					}
					proxy["network"] = v.Net
					switch v.Net {
					case "ws":
						proxy["ws-opts"] = map[string]interface{}{
							"path": v.Path,
							"headers": map[string]interface{}{
								"Host": v.Host,
							},
						}
					case "tcp":
						if v.Type != "" && v.Type != "none" {
							proxy["tcp-opts"] = map[string]interface{}{
								"header": map[string]interface{}{
									"type": v.Type,
								},
							}
						}
					case "grpc":
						proxy["grpc-opts"] = map[string]interface{}{
							"grpc-service-name": v.Path,
						}
					case "h2":
						proxy["h2-opts"] = map[string]interface{}{
							"host": []string{v.Host},
							"path": v.Path,
						}
					}
				}
			}
		}
	default:
		return nil
	}
	return proxy
}

func restartMihomo() {
	isInstallingLock.Lock()
	if isInstalling {
		isInstallingLock.Unlock()
		log.Printf("Mihomo restart skipped: installation in progress")
		return
	}
	isInstallingLock.Unlock()

	mihomoMutex.Lock()
	defer mihomoMutex.Unlock()

	if mihomoCmd != nil && mihomoCmd.Process != nil {
		mihomoCmd.Process.Kill()
		mihomoCmd.Process.Wait()
	}

	cmd := exec.Command("./"+binPath, "-f", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to restart Mihomo: %v", err)
		return
	}
	mihomoCmd = cmd
}

func ensureFirewallRules() {
	ports := []string{"58888", "7890", "7891", "9090"}

	if runtime.GOOS == "windows" {
		log.Println("Ensuring Windows firewall rules...")
		for _, port := range ports {
			ruleName := fmt.Sprintf("Mihomo-Manager-Port-%s", port)
			checkCmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name="+ruleName)
			if err := checkCmd.Run(); err != nil {
				addCmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
					"name="+ruleName,
					"dir=in",
					"action=allow",
					"protocol=TCP",
					"localport="+port,
				)
				if err := addCmd.Run(); err != nil {
					log.Printf("Warning: Failed to add Windows firewall rule for port %s (missing admin rights?): %v", port, err)
				} else {
					log.Printf("Added Windows firewall rule for port %s", port)
				}
			}
		}
		return
	}

	if runtime.GOOS == "linux" {
		log.Println("Checking for Linux firewall managers...")
		// Check for UFW
		if _, err := exec.LookPath("ufw"); err == nil {
			log.Println("UFW detected, ensuring rules...")
			for _, port := range ports {
				exec.Command("ufw", "allow", port+"/tcp").Run()
			}
		}
		// Check for firewalld
		if _, err := exec.LookPath("firewall-cmd"); err == nil {
			log.Println("Firewalld detected, ensuring rules...")
			for _, port := range ports {
				exec.Command("firewall-cmd", "--permanent", "--add-port="+port+"/tcp").Run()
			}
			exec.Command("firewall-cmd", "--reload").Run()
		}
		// Fallback to iptables
		if _, err := exec.LookPath("iptables"); err == nil {
			log.Println("iptables detected, ensuring rules...")
			for _, port := range ports {
				// Check if rule exists
				checkCmd := exec.Command("iptables", "-C", "INPUT", "-p", "tcp", "--dport", port, "-j", "ACCEPT")
				if err := checkCmd.Run(); err != nil {
					// Rule doesn't exist, add it
					exec.Command("iptables", "-I", "INPUT", "-p", "tcp", "--dport", port, "-j", "ACCEPT").Run()
				}
			}
		}
	}
}

func killProcessesByPort(addr string) {
	// Extract port number from :xxxx
	portStr := strings.TrimPrefix(addr, ":")

	if runtime.GOOS == "windows" {
		// Use netstat to find PID
		out, err := exec.Command("netstat", "-ano").Output()
		if err != nil {
			return
		}

		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "LISTENING") && strings.Contains(line, ":"+portStr) {
				fields := strings.Fields(line)
				if len(fields) >= 5 {
					pid := fields[len(fields)-1]
					// Don't kill ourself
					if pid == strconv.Itoa(os.Getpid()) {
						continue
					}
					log.Printf("Found process %s using port %s, killing...", pid, portStr)
					exec.Command("taskkill", "/F", "/PID", pid, "/T").Run()
				}
			}
		}
		// Give some time for OS to release ports
		time.Sleep(1 * time.Second)
	}
}
