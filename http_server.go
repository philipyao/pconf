package phttp

import (
    "net/http"
    "context"
    "time"
    "net"
)
const (
    defaultShutdownInterval         = 3
)

type tcpKeepAliveListener struct {
    *net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
    tc, err := ln.AcceptTCP()
    if err != nil {
        return
    }
    tc.SetKeepAlive(true)
    tc.SetKeepAlivePeriod(3 * time.Minute)
    return tc, nil
}

type HTTPWorker struct {
    addr        string
    srv         *http.Server
    static      *static
    router
    logFunc     func(format string, args ...interface{})
}

func New(addr string) *HTTPWorker {
    w := &HTTPWorker{addr: addr}
    w.initRouter()
    return w
}

func (w *HTTPWorker) Serve() error {
    w.mergeRoute()

    w.srv = &http.Server{
        Addr: w.addr,
        Handler: w,
        //ReadTimeout:    a.Conf.App.ReadTimeout,
        //WriteTimeout:   a.Conf.App.WriteTimeout,
        //MaxHeaderBytes: a.Conf.App.MaxHeaderBytes,
    }
    if w.logFunc != nil {
        w.logFunc("[http] start listening on %v.", w.srv.Addr)
    }
    ln, err := net.Listen("tcp", w.srv.Addr)
    if err != nil {
        return err
    }

    go w.srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})

    return nil
}

func (w *HTTPWorker) Close() {
    if w.logFunc != nil {
        w.logFunc("[http] stop listening on %v.", w.srv.Addr)
    }
    ctx, _ := context.WithTimeout(context.Background(), defaultShutdownInterval * time.Second)
    w.srv.Shutdown(ctx)
}

func (w *HTTPWorker) SetLog(l func(format string, args ...interface{})) {
    w.logFunc = l
}

//实现ServeMux接口
func (w *HTTPWorker) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
    //todo recover

    //makeContext
    ctx := makeContext(writer, r)
    request := ctx.Request()
    response := ctx.Response()

    defer response.flush()

    //length check

    //static handler
    if w.static != nil {
        file := w.static.match(request.Path())
        if file != "" {
            //todo 是否启用全局middleware?
            response.File(file)
            return
        }
    }

    //router handler
    route := w.match(request.Method(), request.Path())
    if route == nil {
        if w.logFunc != nil {
            w.logFunc("[http] route not found, %v", request.Path())
        }
        var notfound handler = func(context *Context) error {
            context.Response().Error(http.StatusNotFound, "invalid path or method")
            return nil
        }
        fnChain := w.makeHandlers([]appliable{notfound})
        fnChain(ctx)
        return
    }
    if w.logFunc != nil {
        w.logFunc("[http] route found ok, %v", request.Path())
    }
    route.fnChain(ctx)
}

func (w *HTTPWorker) Static(prefix, dir string) error {
    if w.static == nil {
        w.static = &static{}
    }
    return w.static.serve(prefix, dir)
}
