package main

import (
	"flag"
	slog "log"
	"net"
	"time"

	"github.com/cockroachdb/cmux"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/shunfei/cronsun"
	"github.com/shunfei/cronsun/conf"
	"github.com/shunfei/cronsun/event"
	"github.com/shunfei/cronsun/log"
	"github.com/shunfei/cronsun/web"
)

var (
	level = flag.Int("l", 0, "log level, -1:debug, 0:info, 1:warn, 2:error")
)

func main() {
	flag.Parse()

	lcf := zap.NewDevelopmentConfig()
	lcf.Level.SetLevel(zapcore.Level(*level))
	lcf.Development = false
	logger, err := lcf.Build(zap.AddCallerSkip(1))
	if err != nil {
		slog.Fatalln("new log err:", err.Error())
	}
	log.SetLogger(logger.Sugar())

	if err := cronsun.Init(); err != nil {
		log.Errorf(err.Error())
		return
	}

	l, err := net.Listen("tcp", conf.Config.Web.BindAddr)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	// Create a cmux.
	m := cmux.New(l)
	httpL := m.Match(cmux.HTTP1Fast())
	httpServer, err := web.InitServer()
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	if conf.Config.Mail.Enable {
		var noticer cronsun.Noticer

		if len(conf.Config.Mail.HttpAPI) > 0 {
			noticer = &cronsun.HttpAPI{}
		} else {
			mailer, err := cronsun.NewMail(10 * time.Second)
			if err != nil {
				log.Errorf(err.Error())
				return
			}
			noticer = mailer
		}
		go cronsun.StartNoticer(noticer)
	}

	go func() {
		err := httpServer.Serve(httpL)
		if err != nil {
			panic(err.Error())
		}
	}()

	go m.Serve()

	log.Infof("cronsun web server started on %s, Ctrl+C or send kill sign to exit", conf.Config.Web.BindAddr)
	// 注册退出事件
	event.On(event.EXIT, conf.Exit)
	// 监听退出信号
	event.Wait()
	event.Emit(event.EXIT, nil)
	log.Infof("exit success")
}
