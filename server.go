/*
   StaticServer 主程式
*/
package SherryServer

import(
   "os"
   "fmt"
   "time"
   "strings"
   "context"
   "syscall"
   "net/http"
   "os/signal"
   "go.uber.org/zap"
   "go.uber.org/zap/zapcore"
   "github.com/joho/godotenv"
   "github.com/gorilla/sessions"
   "github.com/asccclass/sherryserver/libs/socketio"
   "github.com/asccclass/sherryserver/libs/calendar"
   "github.com/asccclass/sherryserver/libs/errorexecuter"
)

type Server struct {
   SystemName   string
   Logger       *zap.Logger
   Server       *http.Server
   OriginAllow  *AhoCorasick
   MethodAllow  *AhoCorasick
   SessionManager *sessions.CookieStore
   Socketio     *SherrySocketIO.SrySocketio
   Calendar	*SherryCalendar.Calendar
   Error        *SherryErrorExecuter.ErrorExecuter
   /*
   LiveKit	*SryLiveKit.LiveKit
   Template	*SherryPages.Page
   GeoLocation  *SherryGeoLocation.SryLocation
   Wallpaper    *SryWallPaper.Bin
   Dbconnect    *SherryDB.DBConnect
   JWTServerSecret      string
   IPInfo       *IPService.IP
   Crypt        *SryCrypt.Crypt
   Linebot      *SherryLineBot.LineBot
   LineLogin    *SryLineLogin.LineLogin
   SSE          *SherrySSE.SrySSE
   JWT          *SryAuth.SryJWT
   Crawer       *SherryCrawer.SryCrawer
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
      keyfile := os.Getenv("sslKeyfile")
      certfile := os.Getenv("sslCertification")
      if keyfile == "" || certfile == "" {
         if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            app.Logger.Fatal(err.Error(), zap.String("addr", app.Server.Addr))
          }
      } else {
         if err := app.Server.ListenAndServeTLS(certfile, keyfile); err != nil && err != http.ErrServerClosed {
            app.Logger.Fatal(err.Error(), zap.String("addr", app.Server.Addr))
         }
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
   srv := &http.Server{
      Addr:         listenAddr,
      Handler:      nil,   // 後面再assign
      ErrorLog:     errorLog,
      ReadTimeout:  20 * time.Second,
      WriteTimeout: 10 * time.Second,
      IdleTimeout:  120 * time.Second,
   }
   // 建立 Session Store
   sessionManager := sessions.NewCookieStore([]byte("$$justgps@gmail.com#$&&%&&$$$$"))
   // 處理Original 
   orglists := NewAhoCorasick()
   methodlists := NewAhoCorasick()
   orgs := os.Getenv("OriginAllowList")   // ex."http://127.0.0.1:9999";....
   if orgs != "" {  // 有設定CROS
      orglists.AddPatterns(orgs, ";")
   }
   if methods := os.Getenv("AllowMethods"); methods != "" {
      methodlists.AddPatterns(strings.ToUpper(methods), ";")
   }
   // socketio Tool
   skio := SherrySocketIO.NewSrySocketio()
   // Calendar Tool
   cal := SherryCalendar.NewSryCalendar()

   // ErrorExecuter
   sryerror, _ := SherryErrorExecuter.NewErrorExecuter()

   return &Server {name, logger, srv, orglists, methodlists, sessionManager, skio, cal, sryerror }, nil
}
