/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

const deprecationMsg string = "NOTICE: clusterctl has been deprecated in v1alpha2 and will be removed in a future version."

var helpTemplate = fmt.Sprintf(`%s
{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`, deprecationMsg)

var RootCmd = &cobra.Command{
	Use:   "clusterctl",
	Short: "cluster management",
	Long:  `Simple kubernetes cluster management`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func exitWithHelp(cmd *cobra.Command, err string) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	_ = cmd.Help()
	os.Exit(1)
}

func init() {
	klog.InitFlags(flag.CommandLine)
	RootCmd.SetGlobalNormalizationFunc(cliflag.WordSepNormalizeFunc)
	RootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	RootCmd.SetHelpTemplate(helpTemplate)
	InitLogs()
}
