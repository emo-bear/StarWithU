package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"landx/internal/media"
	"landx/internal/store"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是对前端暴露的全部后端能力。
type App struct {
	ctx       context.Context
	mu        sync.Mutex
	st        *store.Store
	dataDir   string
	loveStart string
}

type Settings struct {
	DataDir    string `json:"dataDir"`
	EventCount int    `json:"eventCount"`
	ImageCount int    `json:"imageCount"`
	LoveStart  string `json:"loveStart"` // 相爱开始的日期 YYYY-MM-DD
	SpanDays   int    `json:"spanDays"`  // 从相爱那天到今天
}

type config struct {
	DataDir   string `json:"dataDir"`
	LoveStart string `json:"loveStart"`
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.boot(); err != nil {
		runtime.LogErrorf(ctx, "启动失败: %v", err)
	}
}

func (a *App) boot() error {
	dir, love := a.loadConfig()
	if dir == "" {
		dir = defaultDataDir()
	}
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		return err
	}
	st, err := store.Open(dir)
	if err != nil {
		return err
	}
	a.dataDir = dir
	a.st = st
	if love == "" {
		love = today()
	}
	a.loveStart = love
	a.saveConfig()
	if err := a.seedIfEmpty(); err != nil {
		return err
	}
	if n, err := a.st.Count(); err == nil {
		a.logf("启动完成，数据目录 %s，共 %d 条记录", dir, n)
	}
	return nil
}

// defaultDataDir 优先放在程序旁边的 data 目录（便携）；不可写时退到「文档\Star With U」。
func defaultDataDir() string {
	portable := filepath.Join(execDir(), "data")
	if canWrite(portable) {
		return portable
	}
	if home, err := os.UserHomeDir(); err == nil {
		doc := filepath.Join(home, "Documents", "Star With U")
		// 早期版本用的是「噜噜华」，有数据就直接接着用
		legacy := filepath.Join(home, "Documents", "噜噜华")
		if _, err := os.Stat(filepath.Join(legacy, "star.db")); err == nil {
			return legacy
		}
		if _, err := os.Stat(filepath.Join(legacy, "lulu.db")); err == nil {
			return legacy
		}
		if canWrite(doc) {
			return doc
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "StarWithU", "data")
	}
	return portable
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func canWrite(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".writable")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}

func (a *App) imagesDir() string { return filepath.Join(a.dataDir, "images") }

// ---------- 事件 ----------

func (a *App) ListEvents() ([]store.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.st == nil {
		return nil, errors.New("数据库未就绪")
	}
	list, err := a.st.List()
	if err != nil {
		a.logf("ListEvents 失败: %v", err)
		return nil, err
	}
	return list, nil
}

// logf 把关键动作记到数据目录的 app.log，方便排查；超过 256KB 自动重来。
func (a *App) logf(format string, args ...any) {
	if a.dataDir == "" {
		return
	}
	p := filepath.Join(a.dataDir, "app.log")
	if fi, err := os.Stat(p); err == nil && fi.Size() > 256*1024 {
		_ = os.Remove(p)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

func (a *App) GetEvent(id int64) (store.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.st.Get(id)
}

// SaveEvent 保存一条记录（ID 为 0 表示新建），返回落库后的完整数据。
func (a *App) SaveEvent(in store.Event) (store.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = "未命名的一天"
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}
	if err := a.st.Save(&in); err != nil {
		return in, err
	}
	return a.st.Get(in.ID)
}

func (a *App) DeleteEvent(id int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.st.Delete(id)
}

// ---------- 图片 ----------

// PickImages 弹出多选对话框，把选中的图片导入数据目录，返回文件名列表。
func (a *App) PickImages() ([]string, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择照片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片 (*.jpg;*.jpeg;*.png;*.webp;*.bmp;*.gif)", Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.bmp;*.gif"},
		},
	})
	if err != nil || len(paths) == 0 {
		return []string{}, err
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		name, err := media.Import(p, a.imagesDir())
		if err != nil {
			runtime.LogErrorf(a.ctx, "导入失败 %s: %v", p, err)
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// ImageData 返回可直接渲染的 data URL。
func (a *App) ImageData(name string) (string, error) {
	return media.DataURL(a.imagesDir(), name)
}

// ---------- 设置 ----------

func (a *App) GetSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()

	s := Settings{DataDir: a.dataDir, LoveStart: a.loveStart}
	if s.LoveStart == "" {
		s.LoveStart = today()
	}
	if t, err := time.ParseInLocation("2006-01-02", s.LoveStart, time.Local); err == nil {
		now := time.Now()
		days := int(now.Sub(t).Hours()/24) + 1
		if days < 1 {
			days = 1
		}
		s.SpanDays = days
	}
	if a.st == nil {
		return s
	}
	list, err := a.st.List()
	if err == nil {
		s.EventCount = len(list)
		for _, e := range list {
			s.ImageCount += len(e.Images)
		}
	}
	return s
}

// SetLoveStart 记录「相爱开始的那天」，日期格式 YYYY-MM-DD。
func (a *App) SetLoveStart(date string) (Settings, error) {
	date = strings.TrimSpace(date)
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return Settings{}, errors.New("日期格式不对，需要 YYYY-MM-DD")
	}
	if t.Year() < 1900 || t.After(time.Now().AddDate(0, 0, 1)) {
		return Settings{}, errors.New("日期超出范围")
	}
	a.mu.Lock()
	a.loveStart = date
	a.saveConfig()
	a.mu.Unlock()
	return a.GetSettings(), nil
}

// ChooseDataDir 让用户把数据目录换到任意盘符下的文件夹（数据会自动搬过去）。
func (a *App) ChooseDataDir() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "选择数据存放位置",
		DefaultDirectory: a.dataDir,
	})
	if err != nil || dir == "" {
		return a.dataDir, err
	}

	oldDir := a.dataDir
	newDir := filepath.Join(dir, "Star With U")
	// 用户挑的目录本身就是一个数据目录时，直接用它
	if _, err := os.Stat(filepath.Join(dir, "star.db")); err == nil {
		newDir = dir
	}
	if filepath.Clean(newDir) == filepath.Clean(oldDir) {
		return a.dataDir, nil
	}

	a.st.Close()
	if err := copyTree(oldDir, newDir); err != nil {
		if st, e := store.Open(oldDir); e == nil {
			a.st = st
		}
		return oldDir, err
	}
	st, err := store.Open(newDir)
	if err != nil {
		if s2, e := store.Open(oldDir); e == nil {
			a.st = s2
		}
		return oldDir, err
	}
	a.st = st
	a.dataDir = newDir
	a.saveConfig()
	return newDir, nil
}

func (a *App) OpenDataDir() error {
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		return err
	}
	return exec.Command("explorer", a.dataDir).Start()
}

// ---------- 配置 ----------

func configPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = execDir()
	}
	return filepath.Join(base, "StarWithU", "config.json")
}

// legacyConfigPath 早期版本存放配置的位置。
func legacyConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "LuluHua", "config.json")
}

func (a *App) loadConfig() (dir string, love string) {
	path := configPath()
	if _, err := os.Stat(path); err != nil {
		path = legacyConfigPath()
	}
	if path == "" {
		return "", ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var c config
	if json.Unmarshal(raw, &c) != nil {
		return "", ""
	}
	if c.DataDir != "" {
		if _, err := os.Stat(c.DataDir); err == nil {
			dir = c.DataDir
		}
	}
	return dir, c.LoveStart
}

func (a *App) saveConfig() {
	p := configPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	raw, _ := json.MarshalIndent(config{DataDir: a.dataDir, LoveStart: a.loveStart}, "", "  ")
	_ = os.WriteFile(p, raw, 0o644)
}

func execDir() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

func copyTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// seedIfEmpty 首次运行时放几条示例，让星球不是空的。
func (a *App) seedIfEmpty() error {
	n, err := a.st.Count()
	if err != nil || n > 0 {
		return err
	}
	seed := []store.Event{
		{Title: "第一次见你", Date: "2025-11-20", Description: "在人群中一眼就看见你。那天天气很好，我却只记得你笑起来的样子。"},
		{Title: "平安夜的苹果", Date: "2025-12-24", Description: "你塞给我一个系着红绳的苹果，说吃了就会平安。我到现在都没舍得吃。"},
		{Title: "跨年的烟花", Date: "2026-01-01", Description: "零点的烟花在头顶炸开，你在旁边喊我的名字。我说新年快乐，其实想说别的。"},
		{Title: "第一个情人节", Date: "2026-02-14", Description: "花买得太急，被挤坏了三朵。你说没关系，剩下的最好看。"},
		{Title: "一起去看海", Date: "2026-03-08", Description: "风很大，你把外套脱给我。海水很凉，我们踩了很久也没觉得冷。"},
		{Title: "雨天的伞", Date: "2026-04-05", Description: "伞不够大，你半边肩膀全湿了，还说不冷。回去就感冒了，笨蛋。"},
		{Title: "520 的蛋糕", Date: "2026-05-20", Description: "你偷偷订了蛋糕，蜡烛插歪了。许愿的时候我睁着眼睛看你。"},
		{Title: "深夜的电话", Date: "2026-06-18", Description: "凌晨两点你说睡不着，我们就这么聊到天亮，谁也没提困。"},
		{Title: "第一次一起做饭", Date: "2026-07-30", Description: "厨房像打过仗，菜有点咸。但那顿饭我记了很久。"},
		{Title: "山顶的星星", Date: "2026-08-25", Description: "爬了三个小时，山顶冷得要命，星空铺满整个天。你说以后每年都来。"},
		{Title: "一起搬家", Date: "2026-10-03", Description: "两个行李箱，一屋子纸箱。从那天起，「家」这个字有了具体的地址。"},
		{Title: "今天也很喜欢你", Date: "2026-11-08", Description: "没有什么特别的事，只是突然想记下来：今天也很喜欢你。"},
	}
	for i := range seed {
		if err := a.st.Save(&seed[i]); err != nil {
			return err
		}
	}
	return nil
}
