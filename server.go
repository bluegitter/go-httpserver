package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const maxLogFiles = 10

var (
	currentLogFile int
	lastLogDate    time.Time
	logMutex       sync.Mutex
	fileLogger     *log.Logger // 用于文件的日志记录器
	consoleLogger  *log.Logger // 用于控制台的日志记录器
)

func init() {
	// 初始化 fileLogger，不包含颜色代码
	logFile, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Error opening server.log: %v", err)
	}
	fileLogger = log.New(logFile, "", log.LstdFlags)

	// 初始化 consoleLogger，包含颜色代码
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags)

	lastLogDate = time.Now().Truncate(24 * time.Hour)
}

func rotateLogFile() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	// 计算新的日志文件名
	currentLogFile = (currentLogFile % maxLogFiles) + 1
	newLogFileName := fmt.Sprintf("server%d.log", currentLogFile)

	// 重命名当前的 server.log
	err := os.Rename("server.log", newLogFileName)
	if err != nil {
		return err
	}

	// 创建一个新的 server.log 文件
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	// 更新 fileLogger 以使用新的文件
	fileLogger.SetOutput(file)

	// 更新 lastLogDate 为今天
	lastLogDate = time.Now().Truncate(24 * time.Hour)
	return nil
}

func checkLogRotation() {
	today := time.Now().Truncate(24 * time.Hour)
	if lastLogDate.Before(today) {
		err := rotateLogFile()
		if err != nil {
			log.Fatalf("Error rotating log file: %v", err)
		}
	}
}

// ANSI 颜色代码
const (
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorReset   = "\033[0m"
)

func coloredMethod(method string) string {
	uppercaseMethod := strings.ToUpper(method)

	switch uppercaseMethod {
	case "GET":
		return colorBlue + uppercaseMethod + colorReset
	case "POST":
		return colorGreen + uppercaseMethod + colorReset
	case "PUT":
		return colorYellow + uppercaseMethod + colorReset
	case "DELETE":
		return colorRed + uppercaseMethod + colorReset
	default:
		return colorMagenta + uppercaseMethod + colorReset
	}
}

// 定义一个 HTTP 日志记录器
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	length      int
	wroteHeader bool // 新增字段，用于跟踪是否已经写入头部
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK, 0, false}
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if !lrw.wroteHeader {
		lrw.WriteHeader(http.StatusOK)
	}
	size, err := lrw.ResponseWriter.Write(b)
	lrw.length += size
	return size, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	if lrw.wroteHeader {
		return // 如果头部已经写入，直接返回
	}
	lrw.ResponseWriter.WriteHeader(statusCode)
	lrw.statusCode = statusCode
	lrw.wroteHeader = true // 设置标志，表示头部已经写入
}

// 包装处理函数以记录日志
func logRequest(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checkLogRotation()

		start := time.Now()
		lrw := NewLoggingResponseWriter(w)
		handler.ServeHTTP(lrw, r)
		duration := time.Since(start)
		method := coloredMethod(r.Method)

		// 从 r.RemoteAddr 中提取 IP 地址
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// 如果无法解析 IP 地址，使用原始的 RemoteAddr
			ip = r.RemoteAddr
		}

		// 控制台日志（包含颜色）
		consoleLogger.Printf("%s [%s] %s %d %d %d\n",
			colorCyan+ip+colorReset, method, colorYellow+r.URL.Path+colorReset, lrw.statusCode, duration.Milliseconds(), lrw.length)

		// 文件日志（不包含颜色）
		fileLogger.Printf("%s [%s] %s %d %d %d\n",
			ip, r.Method, r.URL.Path, lrw.statusCode, duration.Milliseconds(), lrw.length)
	}
}

var ctx = context.Background()
var redisClient *redis.Client

// 定义一个结构体用于JSON响应
type CountResponse struct {
	Page  string `json:"page"`
	Count int64  `json:"count"`
}

func countHandler(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		http.Error(w, "Page parameter is missing", http.StatusBadRequest)
		return
	}

	redisKey := "page.count." + page

	newCount, err := redisClient.Incr(ctx, redisKey).Result()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 创建响应对象
	response := CountResponse{
		Page:  page,
		Count: newCount,
	}

	// 设置响应头为JSON
	w.Header().Set("Content-Type", "application/json")

	// 编码并发送JSON响应
	json.NewEncoder(w).Encode(response)
}

func main() {
	// 定义命令行参数，默认端口为 8080
	var port string
	flag.StringVar(&port, "p", "8080", "Define what TCP port to bind to")

	// 添加 -h 和 --help 选项
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse() // 解析命令行参数

	lastLogDate = time.Now().Truncate(24 * time.Hour)

	http.HandleFunc("/count", countHandler)
	// 设置文件服务器
	fileServer := http.FileServer(http.Dir("."))
	http.Handle("/", logRequest(fileServer))

	consoleLogger.Printf(colorGreen+"Starting server on :%s\n"+colorReset, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		consoleLogger.Fatal("Error starting server: ", err)
	}

}
