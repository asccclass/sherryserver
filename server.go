/*
   StaticServer 主程式
*/
package SherryServer

import(
   "os"
	"fmt"
	"time"
   "context"
   "syscall"
   "net/http"
   "os/signal"
   "go.uber.org/zap"
   "github.com/joho/godotenv"
)

type Server struct {
   SystemName   string
   Logger       *zap.Logger
   Server       *http.Server
   /*
   Template     *SherryTemplate.Template
   GeoLocation  *SherryGeoLocation.SryLocation
   Wallpaper    *SryWallPaper.Bin
   Error        *SherryErrorExecuter.ErrorExecuter
   Dbconnect    *SherryDB.DBConnect
   JWTServerSecret      string
   IPInfo       *IPService.IP
   Crypt        *SryCrypt.Crypt
   Socketio     *SherrySocketIO.SrySocketio
   Linebot      *SherryLineBot.LineBot
   LineLogin    *SryLineLogin.LineLogin
   SSE          *SherrySSE.SrySSE
   JWT          *SryAuth.SryJWT
   Crawer       *SherryCrawer.SryCrawer
   OriginAllow  *SryWords.AhoCorasick
   MethodAllow  *SryWords.AhoCorasick
   Taider       *SryTAIDE.Taide
   GeminiBot    *SherryLineBot.GeminiBot  // Genini Bot
   */
}

// shutdown 關閉伺服器程序
func(app *Server) gracefulShutdown() {
   defer app.Logger.Sync()
   // kill (no param) default send syscall.SIGTERM
   // kill -2 is syscall.SIGINT
   // kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
   interrupt := make(chan os.Signal, 1)
   signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
   signal.Notify(interrupt, os.Interrupt)
   sig := <-interrupt
/*
   if app.Linebot != nil {
      app.Error.Error2Line(app.SystemName, fmt.Errorf(app.Server.Addr + " " + sig.String()))
   }
*/
   app.Logger.Info("Server is shutting down", zap.String("reason", sig.String()))
   ctx, cancel := context.WithTimeout(context.Background(), time.Second * 15)
   defer cancel()
   app.Server.SetKeepAlivesEnabled(false)
   if err := app.Server.Shutdown(ctx); err != nil {
      app.Logger.Fatal("Could not gracefully shutdown the server", zap.Error(err))
   }
   app.Logger.Info("Server stopped")
   // os.Exit(0)
}

// Start runs ListenAndServe on the http.Server with graceful shutdown
func(app *Server) Start() {
   defer app.gracefulShutdown()
   go func() {
      if err := app.Server.ListenAndServe(app.Server.Addr, app.Server.Handler); err != nil && err != http.ErrServerClosed {
         app.Logger.Fatal(err.Error(), zap.String("addr", app.Server.Addr))
      }
   }()
   app.Logger.Info("Server is ready to handle requests at " + app.Server.Addr)
}

// NewServer 建立Server
func NewServer(listenAddr, documentRoot, templatePath string) (*Server, error) {
   name := "SystemName"
   if err := godotenv.Load("envfile"); err != nil {
      return nil, fmt.Errorf("envfile not found.")
   }
   name = os.Getenv("SystemName")
   logger, err := zap.NewDevelopment(zap.AddStacktrace(zapcore.FatalLevel))
   if err != nil {
      return nil, err
   }
   errorLog, _ := zap.NewStdLogAt(logger, zap.ErrorLevel)
   srv := http.Server{
      Addr:         listenAddr,
      // Handler:      router,   // 後面再assign
      ErrorLog:     errorLog,
      ReadTimeout:  20 * time.Second,
      WriteTimeout: 10 * time.Second,
      IdleTimeout:  15 * time.Second,
   }
   return &Server {name, logger, srv }, nil
}