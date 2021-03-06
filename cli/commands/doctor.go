package commands

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/hashicorp/go-version"
	"github.com/phase2/rig/cli/util"
	"github.com/urfave/cli"
)

type Doctor struct {
	BaseCommand
}

func (cmd *Doctor) Commands() cli.Command {
	return cli.Command{
		Name:   "doctor",
		Usage:  "Troubleshoot the Rig environment",
		Before: cmd.Before,
		Action: cmd.Run,
	}
}

func (cmd *Doctor) Run(c *cli.Context) error {

	// 1. Check Docker API Version compatibility
	clientApiVersion := util.GetDockerClientApiVersion()
	serverApiVersion, err := util.GetDockerServerApiVersion(cmd.machine.Name)
	serverMinApiVersion, _ := util.GetDockerServerMinApiVersion(cmd.machine.Name)

	// Older clients can talk to newer servers, and when you ask a newer server
	// it's version in the presence of an older server it will downgrade it's
	// compatability as far as possible. So as long as the client API is not greater
	// than the servers current version or less than the servers minimum api version
	// then we are compatible
	constraintString := fmt.Sprintf("<= %s", serverApiVersion)
	if serverMinApiVersion != nil {
		constraintString = fmt.Sprintf(">= %s", serverMinApiVersion)
	}
	apiConstraint, _ := version.NewConstraint(constraintString)

	if err != nil {
		cmd.out.Error.Println("Could not determine Docker Machine Docker versions: ", err)
	} else if clientApiVersion.Equal(serverApiVersion) {
		cmd.out.Info.Printf("Docker Client (%s) and Server (%s) have equal API Versions", clientApiVersion, serverApiVersion)
	} else if apiConstraint.Check(clientApiVersion) {
		cmd.out.Info.Printf("Docker Client (%s) has Server compatible API version (%s). Server current (%s), Server min compat (%s)", clientApiVersion, constraintString, serverApiVersion, serverMinApiVersion)
	} else {
		cmd.out.Error.Printf("Docker Client (%s) is incompatible with Server. Server current (%s), Server min compat (%s). Use `rig upgrade` to fix this.", clientApiVersion, serverApiVersion, serverMinApiVersion)
	}

	// 2. Pull down the data from DNSDock. This will confirm we can resolve names as well
	//    as route to the appropriate IP addresses via the added route commands
	if cmd.machine.IsRunning() {
		dnsRecords := DnsRecords{BaseCommand{machine: cmd.machine, out: cmd.out}}
		if records, err := dnsRecords.LoadRecords(); err == nil {
			resolved := false
			for _, record := range records {
				if record["Name"] == "dnsdock" {
					resolved = true
					cmd.out.Info.Printf("DNS and routing services are working. DNSDock resolves to %s", record["IPs"])
					break
				}
			}

			if !resolved {
				cmd.out.Error.Println("Unable to verify DNS services are working.")
			}
		} else {
			cmd.out.Error.Println("Unable to verify DNS services and routing are working.")
			cmd.out.Error.Println(err)
		}
	} else {
		cmd.out.Warning.Printf("Docker Machine `%s` is not running. Cannot determine if DNS resolution is working correctly.", cmd.machine.Name)
	}

	// 3. Ensure that docker-machine-nfs script is available for our NFS mounts (Mac ONLY)
	if runtime.GOOS == "darwin" {
		if err := exec.Command("which", "docker-machine-nfs").Run(); err != nil {
			cmd.out.Error.Println("Docker Machine NFS is not installed.")
		} else {
			cmd.out.Info.Println("Docker Machine NFS is installed.")
		}
	}

	return nil
}
