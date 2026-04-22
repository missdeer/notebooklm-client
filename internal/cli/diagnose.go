package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/missdeer/notebooklm-client/internal/rpc"
	"github.com/missdeer/notebooklm-client/internal/session"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Show system info and session status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("OS:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Go:       %s\n", runtime.Version())
		fmt.Printf("Home:     %s\n", rpc.HomeDir())
		fmt.Printf("Session:  %s\n", rpc.SessionPath())

		valid, _ := session.HasValid("", 0)
		if valid {
			fmt.Println("Status:   session valid")
		} else {
			fmt.Println("Status:   no valid session")
		}

		overrides := rpc.LoadRpcIDOverrides()
		fmt.Printf("RPC overrides: %d\n", len(overrides))
		return nil
	},
}
