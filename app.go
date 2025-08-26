package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	SteamDir string
}

//go:embed assets/user32.dll
var user32dll []byte

type Game struct {
	AppID int    `json:"appid"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

// const configFileName = "config.json"

type Config struct {
	SteamDir string `json:"steam_dir"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

var (
	gameCache   = make(map[int]Game) // appid -> Game
	cacheExpiry = make(map[int]time.Time)
	cacheTTL    = 24 * time.Hour
	cacheMutex  sync.Mutex
	// cacheFile   = "cache.json"
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
	runtime.LogDebug(a.ctx, "Startup called")
	fmt.Println("SteamDir:", a.SteamDir)
	fmt.Println("Assets loaded from frontend/dist")
}

func configPath() (string, error) {
	return xdg.ConfigFile("GreenLuma/config.json")
}

func cachePath() (string, error) {
	return xdg.CacheFile("GreenLuma/cache.json")
}

func (a *App) SelectSteamDir() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите директорию Steam",
	})
	if err != nil || dir == "" {
		return "", errors.New("директория не выбрана")
	}

	if !a.validateSteamDir(dir) {
		return "", errors.New("в выбранной директории нет steam.exe")
	}

	a.SteamDir = dir
	a.saveConfig()
	return dir, nil
}

func (a *App) GetInstalledGames() ([]Game, error) {
	if a.SteamDir == "" {
		fmt.Println("SteamDir пустой")
		return nil, errors.New("SteamDir не выбран")
	}

	if len(gameCache) == 0 {
		fmt.Println("Загружаем кэш с диска...")
		loadCacheFromDisk()
	}

	appListDir := filepath.Join(a.SteamDir, "AppList")
	entries, err := os.ReadDir(appListDir)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	gamesMu := sync.Mutex{}
	games := []Game{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		path := filepath.Join(appListDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		appid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}

		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			game, err := fetchSteamAppInfo(id)
			if err != nil {
				return
			}
			gamesMu.Lock()
			games = append(games, game)
			gamesMu.Unlock()
		}(appid)
	}

	wg.Wait()
	return games, nil
}

func (a *App) RemoveGame(appid int) error {
	if a.SteamDir == "" {
		return errors.New("SteamDir не выбран")
	}

	appListDir := filepath.Join(a.SteamDir, "AppList")
	entries, err := os.ReadDir(appListDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		path := filepath.Join(appListDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		if id == appid {
			os.Remove(path)
			break
		}
	}

	entries, _ = os.ReadDir(appListDir)
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		files = append(files, filepath.Join(appListDir, entry.Name()))
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] < files[j]
	})

	for i, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		os.WriteFile(filepath.Join(appListDir, fmt.Sprintf("%d.txt", i)), data, 0644)
		if f != filepath.Join(appListDir, fmt.Sprintf("%d.txt", i)) {
			os.Remove(f)
		}
	}
	return nil
}

func (a *App) SearchGames(query string) ([]Game, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	params := url.Values{}
	params.Set("term", q)
	params.Set("cc", "us")
	params.Set("l", "en")

	endpoint := url.URL{
		Scheme:   "https",
		Host:     "store.steampowered.com",
		Path:     "/api/storesearch/",
		RawQuery: params.Encode(),
	}

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("steam storesearch: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			AppID int    `json:"id"`
			Name  string `json:"name"`
			Img   string `json:"tiny_image"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	games := make([]Game, 0, len(result.Items))
	for _, item := range result.Items {
		games = append(games, Game{
			AppID: item.AppID,
			Name:  item.Name,
			Image: item.Img,
		})
	}

	return games, nil
}

func (a *App) AddGame(appid int) error {
	appListDir := filepath.Join(a.SteamDir, "AppList")
	// ищем новый индекс
	files, _ := os.ReadDir(appListDir)
	index := len(files)
	return os.WriteFile(filepath.Join(appListDir, fmt.Sprintf("%d.txt", index)), []byte(strconv.Itoa(appid)), 0644)
}

func (a *App) GetSteamDir() (string, error) {
	for !a.validateSteamDir(a.SteamDir) {
		a.SelectSteamDir()
	}

	return a.SteamDir, nil
}

func (a *App) IsDllInstalled() (bool, error) {
	if a.SteamDir == "" {
		return false, errors.New("SteamDir не выбран")
	}
	dllPath := filepath.Join(a.SteamDir, "user32.dll")
	if _, err := os.Stat(dllPath); err == nil {
		return true, nil
	}
	return false, nil
}

func (a *App) InstallDll() error {
	if a.SteamDir == "" {
		return errors.New("SteamDir не выбран")
	}
	dllPath := filepath.Join(a.SteamDir, "user32.dll")
	err := os.WriteFile(dllPath, user32dll, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) RemoveDll() error {
	if a.SteamDir == "" {
		return errors.New("SteamDir не выбран")
	}
	dllPath := filepath.Join(a.SteamDir, "user32.dll")
	if _, err := os.Stat(dllPath); err != nil {
		return errors.New("dll не установлен")
	}
	return os.Remove(dllPath)
}

func (a *App) DeleteSteamCache() (string, error) {
	if a.SteamDir == "" {
		return "", errors.New("SteamDir не выбран")
	}

	cacheFile := filepath.Join(a.SteamDir, "appcache", "packageinfo.vdf")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return "Кэш уже очищен", nil
	}

	if err := os.Remove(cacheFile); err != nil {
		return "", err
	}

	return "Кэш очищен!", nil
}

func (a *App) validateSteamDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "steam.exe")); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func fetchSteamAppInfo(appid int) (Game, error) {
	cacheMutex.Lock()
	if g, ok := gameCache[appid]; ok && time.Now().Before(cacheExpiry[appid]) {
		cacheMutex.Unlock()
		return g, nil
	}
	cacheMutex.Unlock()

	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&cc=us&l=en", appid)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := httpClient.Do(req)
	if err != nil {
		cacheMutex.Lock()
		if g, ok := gameCache[appid]; ok {
			cacheMutex.Unlock()
			return g, nil
		}
		cacheMutex.Unlock()
		return Game{}, err
	}
	defer resp.Body.Close()

	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Name        string `json:"name"`
			HeaderImage string `json:"header_image"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Game{}, err
	}

	game := Game{AppID: appid, Name: "Unknown", Image: ""}
	if entry, ok := result[strconv.Itoa(appid)]; ok && entry.Success {
		game.Name = entry.Data.Name
		game.Image = entry.Data.HeaderImage
	}

	cacheMutex.Lock()
	gameCache[appid] = game
	cacheExpiry[appid] = time.Now().Add(cacheTTL)
	cacheMutex.Unlock()
	go saveCacheToDisk()

	return game, nil
}

/* Old config funcs
func (a *App) loadConfig() {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		fmt.Println("Ошибка при загрузке файла конфига:", err)
		return
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Println("Ошибка при сериализации файла конфига:", err)
		return
	}
	a.SteamDir = cfg.SteamDir
}

func (a *App) saveConfig() {
	cfg := Config{SteamDir: a.SteamDir}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configFileName, data, 0644)
}
*/

func (a *App) loadConfig() {
	path, err := configPath()
	if err != nil {
		fmt.Println("Ошибка определения пути конфига:", err)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Конфиг не найден, будет создан новый:", err)
		return
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Println("Ошибка сериализации конфига:", err)
		return
	}
	a.SteamDir = cfg.SteamDir
}

func (a *App) saveConfig() {
	path, err := configPath()
	if err != nil {
		fmt.Println("Ошибка определения пути конфига:", err)
		return
	}

	cfg := Config{SteamDir: a.SteamDir}
	data, _ := json.MarshalIndent(cfg, "", "  ")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Println("Ошибка создания папки конфига:", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Println("Ошибка сохранения конфига:", err)
	}
}

/* Old cache funcs
func loadCacheFromDisk() {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		fmt.Println("Ошибка при чтении файла кэша:", err)
		return
	}

	var games []Game
	if err := json.Unmarshal(data, &games); err != nil {
		fmt.Println("Ошибка при сериализации игр из кэша:", err)
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	for _, g := range games {
		gameCache[g.AppID] = g
		cacheExpiry[g.AppID] = time.Now().Add(cacheTTL)
	}
}

func saveCacheToDisk() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	games := make([]Game, 0, len(gameCache))
	for _, g := range gameCache {
		games = append(games, g)
	}

	data, _ := json.MarshalIndent(games, "", "  ")
	os.WriteFile(cacheFile, data, 0644)
}
*/

func loadCacheFromDisk() {
	path, err := cachePath()
	if err != nil {
		fmt.Println("Ошибка определения пути кэша:", err)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Ошибка чтения кэша:", err)
		return
	}

	var games []Game
	if err := json.Unmarshal(data, &games); err != nil {
		fmt.Println("Ошибка сериализации кэша:", err)
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	for _, g := range games {
		gameCache[g.AppID] = g
		cacheExpiry[g.AppID] = time.Now().Add(cacheTTL)
	}
}

func saveCacheToDisk() {
	path, err := cachePath()
	if err != nil {
		fmt.Println("Ошибка определения пути кэша:", err)
		return
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	games := make([]Game, 0, len(gameCache))
	for _, g := range gameCache {
		games = append(games, g)
	}

	data, _ := json.MarshalIndent(games, "", "  ")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Println("Ошибка создания папки кэша:", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Println("Ошибка сохранения кэша:", err)
	}
}
