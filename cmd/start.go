/*
Copyright 2016 The Kubernetes Authors.

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
	goflag "flag"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/ogre0403/iscsi-provisioner/provisioner"
	"github.com/ogre0403/iscsi-target-client/pkg/model"

	log "github.com/golang/glog"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// start-controllerCmd represents the start-controller command
var startcontrollerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start an iscsi dynamic provisioner",
	Long:  `Start an iscsi dynamic provisioner`,
	Run: func(cmd *cobra.Command, args []string) {
		goflag.Parse()
		log.V(2).Infof("start called")
		var config *rest.Config
		var err error
		master := viper.GetString("master")
		kubeconfig := viper.GetString("kubeconfig")
		// creates the in-cluster config
		log.V(2).Infof("creating in cluster default kube client config")
		if master != "" || kubeconfig != "" {
			config, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
		} else {
			config, err = rest.InClusterConfig()
		}
		if err != nil {
			log.Fatalln(err)
		}
		log.V(2).Infof("kube client config created")

		// creates the clientset
		log.V(2).Infof("creating kube client set")
		kubernetesClientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalln(err)
		}
		log.V(2).Infof("kube client set created")

		// The controller needs to know what the server version is because out-of-tree
		// provisioners aren't officially supported until 1.5
		serverVersion, err := kubernetesClientSet.Discovery().ServerVersion()
		if err != nil {
			log.Fatalf("Error getting server version: %v", err)
		}

		addr := viper.GetString("target-api-address")
		port := viper.GetInt("target-api-port")
		log.V(2).Infof("target api addr: %s:%d", addr, port)

		sc := &model.ServerCfg{
			Port:     port,
			Username: viper.GetString("api-username"),
			Password: viper.GetString("api-password"),
		}

		iscsiProvisioner := provisioner.NewiscsiProvisioner(addr, sc)
		log.V(2).Infof("iscsi provisioner created")

		pc := controller.NewProvisionController(kubernetesClientSet, viper.GetString("provisioner-name"), iscsiProvisioner, serverVersion.GitVersion, controller.Threadiness(1))
		controller.ResyncPeriod(viper.GetDuration("resync-period"))
		controller.ExponentialBackOffOnError(viper.GetBool("exponential-backoff-on-error"))
		controller.FailedProvisionThreshold(viper.GetInt("fail-retry-threshold"))
		controller.FailedDeleteThreshold(viper.GetInt("fail-retry-threshold"))
		controller.LeaseDuration(viper.GetDuration("lease-period"))
		controller.RenewDeadline(viper.GetDuration("renew-deadline"))
		controller.RetryPeriod(viper.GetDuration("retry-period"))
		log.V(2).Infof("iscsi controller created, running forever...")
		pc.Run(wait.NeverStop)
	},
}

func init() {
	RootCmd.AddCommand(startcontrollerCmd)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	// provisioner flag
	startcontrollerCmd.Flags().String("provisioner-name", "iscsi-target-api", "name of this provisioner, must match what is passed in the storage class annotation")
	viper.BindPFlag("provisioner-name", startcontrollerCmd.Flags().Lookup("provisioner-name"))
	startcontrollerCmd.Flags().String("target-api-address", "localhost", "ip or dns of the target api server")
	viper.BindPFlag("target-api-address", startcontrollerCmd.Flags().Lookup("target-api-address"))
	startcontrollerCmd.Flags().Int("target-api-port", 8811, "port on which target api server is listening")
	viper.BindPFlag("target-api-port", startcontrollerCmd.Flags().Lookup("target-api-port"))
	startcontrollerCmd.Flags().String("default-fs", "xfs", "filesystem to use when not specified")
	viper.BindPFlag("default-fs", startcontrollerCmd.Flags().Lookup("default-fs"))
	startcontrollerCmd.Flags().String("api-username", "admin", "username for the iscsi-target-api")
	viper.BindPFlag("api-username", startcontrollerCmd.Flags().Lookup("api-username"))
	startcontrollerCmd.Flags().String("api-password", "", "password for the iscsi-target-api")
	viper.BindPFlag("api-password", startcontrollerCmd.Flags().Lookup("api-password"))
	//startcontrollerCmd.Flags().String("session-chap-credential-file-path", "/var/run/secrets/iscsi-provisioner/session-chap-credential.properties", "path where the credential for session chap authentication can be found")
	//viper.BindPFlag("session-chap-credential-file-path", startcontrollerCmd.Flags().Lookup("session-chap-credential-file-path"))

	// Kubernetes flag
	startcontrollerCmd.Flags().Duration("resync-period", controller.DefaultResyncPeriod, "how often to poll the master API for updates")
	viper.BindPFlag("resync-period", startcontrollerCmd.Flags().Lookup("resync-period"))
	startcontrollerCmd.Flags().Bool("exponential-backoff-on-error", controller.DefaultExponentialBackOffOnError, "exponential-backoff-on-error doubles the retry-period everytime there is an error")
	viper.BindPFlag("exponential-backoff-on-error", startcontrollerCmd.Flags().Lookup("exponential-backoff-on-error"))
	startcontrollerCmd.Flags().Int("fail-retry-threshold", controller.DefaultFailedProvisionThreshold, "Threshold for max number of retries on failure of provisioner")
	viper.BindPFlag("fail-retry-threshold", startcontrollerCmd.Flags().Lookup("fail-retry-threshold"))
	startcontrollerCmd.Flags().Duration("lease-period", controller.DefaultLeaseDuration, "LeaseDuration is the duration that non-leader candidates will wait to force acquire leadership. This is measured against time of last observed ack")
	viper.BindPFlag("lease-period", startcontrollerCmd.Flags().Lookup("lease-period"))
	startcontrollerCmd.Flags().Duration("renew-deadline", controller.DefaultRenewDeadline, "RenewDeadline is the duration that the acting master will retry refreshing leadership before giving up")
	viper.BindPFlag("renew-deadline", startcontrollerCmd.Flags().Lookup("renew-deadline"))
	startcontrollerCmd.Flags().Duration("retry-period", controller.DefaultRetryPeriod, "RetryPeriod is the duration the LeaderElector clients should wait between tries of actions")
	viper.BindPFlag("retry-period", startcontrollerCmd.Flags().Lookup("retry-period"))
	startcontrollerCmd.Flags().String("master", "", "Master URL")
	viper.BindPFlag("master", startcontrollerCmd.Flags().Lookup("master"))
	startcontrollerCmd.Flags().String("kubeconfig", "", "Absolute path to the kubeconfig")
	viper.BindPFlag("kubeconfig", startcontrollerCmd.Flags().Lookup("kubeconfig"))

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// start-controllerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// start-controllerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
