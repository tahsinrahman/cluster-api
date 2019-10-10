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

package phases

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/clusterdeployer/clusterclient"
)

func ApplyClusterAPIComponents(client clusterclient.Client, providerComponents string) error {
	klog.Info("Applying Cluster API Provider Components")

	var clientErr error
	waitErr := wait.PollImmediate(providerComponentsIntervalTimeout, providerComponentsRetryTimeout, func() (bool, error) {
		if clientErr = client.Apply(providerComponents); clientErr != nil {
			return false, nil
		}
		return true, nil
	})
	if waitErr != nil {
		return errors.Wrap(clientErr, "timed out waiting for cluster api components to be ready")
	}

	return client.WaitForClusterV1alpha2Ready()
}
