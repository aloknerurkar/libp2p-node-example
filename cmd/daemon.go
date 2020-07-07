package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/aloknerurkar/gopcp/core/repo"

	nodeImpl "github.com/aloknerurkar/gopcp/core/node"
	"github.com/aloknerurkar/gopcp/gateway"
	logger "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
)

var log = logger.Logger("daemon")

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the P2P daemon and HTTP Gateway",
	Long: `This command will be used to initialize the P2P node.
	As a part of the initialization daemon will be started which will
	be listening for RPC on "listen-addr" and "listen-port" as well
	as HTTP on "http-port"
	`,
	Run: daemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)

	daemonCmd.Flags().String("listen-addr", "0.0.0.0", "Address to listen on for P2P")
	daemonCmd.Flags().String("listen-port", "10000", "Port to listen on for P2P")
	daemonCmd.Flags().String("http-port", "8080", "Port to start HTTP server on")
}

func daemon(cmd *cobra.Command, args []string) {

	logger.SetLogLevel("*", "Info")

	wg := &sync.WaitGroup{}

	rep, err := repo.InitializeRepo()
	if err != nil {
		fmt.Printf("Failed getting identity. (ERR: %s).\n", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	nodeP := nodeImpl.NewNodeImpl(ctx, rep)
	if nodeP == nil {
		fmt.Printf("Failed creating new P2P node")
		return
	}

	var gatewayErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx = log.Start(ctx, "Gateway service")
		defer log.Finish(ctx)
		gatewayErr = gateway.Start(ctx, nodeP)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		cancel()
		fmt.Print("\nReceived an interrupt, stopping services...\n")
		fmt.Print("\nHit Ctrl-C again to force shutdown\n")
	}()

	wg.Wait()
	if gatewayErr != nil {
		// Case where gateway process exits and we stop waiting
		// we will need to stop the Node in this case
		cancel()
		log.Errorf("Gateway returned err %s", gatewayErr.Error())
	}
	log.Info("Done")
}
