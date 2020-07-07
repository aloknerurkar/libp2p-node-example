package cmd

import (
	"os"
	"time"

	process "github.com/jbenet/goprocess"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the P2P node",
	Long: `This command will be used to initialize the P2P node.
	As a part of the initialization daemon will be started which will
	be listening for RPC on "listen-addr" and "listen-port" as well
	as HTTP on "http-port"
	`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Called init")
		px := process.WithTeardown(func() error {
			log.Info("Tearing down process")
			return nil
		})
		px.Go(func(px process.Process) {
			fp, err := os.Create("tmp1")
			if err != nil {
				panic(err)
			}
			for {
				fp.Write([]byte("Hello\n"))
				time.Sleep(time.Second * 2)
			}
		})
		go func() {
			<-px.Closing()
		}()
		go func() {
			time.Sleep(time.Second * 10)
			px.Close()
		}()
		log.Info("Done")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String("listen-addr", "0.0.0.0", "Address to listen on for P2P")
	initCmd.Flags().String("listen-port", "10000", "Port to listen on for P2P")
	initCmd.Flags().String("http-port", "8080", "Port to start HTTP server on")
}
