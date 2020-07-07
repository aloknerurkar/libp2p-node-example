package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/aloknerurkar/gopcp/core"
	pb "github.com/aloknerurkar/gopcp/gateway/pb"
	rpc_svc "github.com/aloknerurkar/gopcp/gateway/rpc"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	logger "github.com/ipfs/go-log"
	"google.golang.org/grpc"
)

var log = logger.Logger("Gateway")

// Start the gateway service. This will be a blocking call. It will
// internally start an RPC as well as an HTTP service.
func Start(ctx context.Context, nd core.Node) (retErr error) {

	wg := &sync.WaitGroup{}
	errc := make(chan error, 2)

	cctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		cancel()
		log.Infof("Parent context done")
	}()

	wg.Add(1)
	done := false
	go func() {
		defer wg.Done()
		for {
			select {
			case <-cctx.Done():
				log.Infof("Stop requested")
				done = true
			case retErr = <-errc:
				log.Errorf("One of the services returned error Err:%s", retErr.Error())
				cancel()
				done = true
			}
			if done {
				break
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := startRPC(cctx, nd)
		if err != nil {
			errc <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := startHTTP(cctx, nd)
		if err != nil {
			errc <- err
		}
	}()

	wg.Wait()
	return
}

func startRPC(ctx context.Context, nd core.Node) (retErr error) {

	var port int
	if !nd.Repository().Conf().Get("gatewayRpcPort", &port) {
		log.Error("Gateway RPC port not specified in config")
		retErr = errors.New("Config not provided")
		return
	}

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Errorf("Failed starting TCP listener Err:%s", err.Error())
		retErr = err
		return
	}

	grpcServer := grpc.NewServer()
	pb.RegisterGoPcpServer(grpcServer, rpc_svc.NewGwService(nd))

	errc := make(chan error, 1)
	wg := &sync.WaitGroup{}

	// Make sure server routine is stopped before returning
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("Starting Gateway RPC on %d", port)
		e := grpcServer.Serve(lis)
		if err != nil {
			errc <- e
		}
	}()

	done := false
	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping RPC server")
			grpcServer.Stop()
			done = true
		case retErr = <-errc:
			log.Errorf("RPC service returned error Err:%s", retErr.Error())
			done = true
		}
		if done {
			break
		}
	}

	wg.Wait()
	return
}

func startHTTP(ctx context.Context, nd core.Node) (retErr error) {

	var rpcPort, httpPort int
	if !nd.Repository().Conf().Get("gatewayRpcPort", &rpcPort) ||
		!nd.Repository().Conf().Get("gatewayHttpPort", &httpPort) {
		log.Error("Gateway config missing")
		retErr = errors.New("Config not provided")
		return
	}

	// Emit default values through GRPC gateway
	// Refer : https://github.com/grpc-ecosystem/grpc-gateway/issues/233
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard,
		&runtime.JSONPb{OrigName: true, EmitDefaults: true}))

	opts := []grpc.DialOption{grpc.WithInsecure()}

	retErr = pb.RegisterGoPcpHandlerFromEndpoint(ctx, mux,
		"localhost:"+strconv.Itoa(rpcPort), opts)
	if retErr != nil {
		log.Errorf("Failed while registering Grpc gateway Err:%s", retErr.Error())
		return
	}

	httpServer := &http.Server{Addr: ":" + strconv.Itoa(httpPort), Handler: mux}

	errc := make(chan error, 1)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Infof("Starting Gateway HTTP on %d", httpPort)
		e := httpServer.ListenAndServe()
		if e != nil {
			errc <- e
		}
	}()

	done := false
	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping HTTP server")
			_ = httpServer.Shutdown(nil)
			done = true
		case retErr = <-errc:
			log.Errorf("HTTP service returned error Err:%s", retErr.Error())
			done = true
		}
		if done {
			break
		}
	}

	wg.Wait()
	return
}
