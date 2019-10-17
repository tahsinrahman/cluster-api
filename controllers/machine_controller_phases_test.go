/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func init() {
	externalReadyWait = 1 * time.Second
}

var _ = Describe("Reconcile Machine Phases", func() {
	deletionTimestamp := metav1.Now()

	var defaultKubeconfigSecret *corev1.Secret
	defaultCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: metav1.NamespaceDefault,
		},
	}

	defaultMachine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-test",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.MachineControlPlaneLabelName: "true",
			},
		},
		Spec: clusterv1.MachineSpec{
			ClusterName: defaultCluster.Name,
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
					Kind:       "BootstrapConfig",
					Name:       "bootstrap-config1",
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
				Kind:       "InfrastructureConfig",
				Name:       "infra-config1",
			},
		},
	}

	defaultBootstrap := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "BootstrapConfig",
			"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
			"metadata": map[string]interface{}{
				"name":      "bootstrap-config1",
				"namespace": "default",
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}

	defaultInfra := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "InfrastructureConfig",
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha2",
			"metadata": map[string]interface{}{
				"name":      "infra-config1",
				"namespace": "default",
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}

	BeforeEach(func() {
		defaultKubeconfigSecret = kubeconfig.GenerateSecret(defaultCluster, kubeconfig.FromEnvTestConfig(cfg, defaultCluster))
	})

	It("Should set `Pending` with a new Machine", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhasePending))
	})

	It("Should set `Provisioning` when bootstrap is ready", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		// Set bootstrap ready.
		unstructured.SetNestedField(bootstrapConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(bootstrapConfig.Object, "...", "status", "bootstrapData")

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhaseProvisioning))
	})

	It("Should set `Provisioned` when bootstrap and infra is ready", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		// Set bootstrap ready.
		unstructured.SetNestedField(bootstrapConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(bootstrapConfig.Object, "...", "status", "bootstrapData")

		// Set infra ready.
		unstructured.SetNestedField(infraConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(infraConfig.Object, "test://id-1", "spec", "providerID")
		unstructured.SetNestedField(infraConfig.Object, []interface{}{
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.1",
			},
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.2",
			},
		}, "status", "addresses")

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())
		Expect(machine.Status.Addresses).To(HaveLen(2))

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhaseProvisioned))
	})

	It("Should set `Provisioned` when bootstrap and infra is ready with no Status.Addresses", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		// Set bootstrap ready.
		unstructured.SetNestedField(bootstrapConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(bootstrapConfig.Object, "...", "status", "bootstrapData")

		// Set infra ready.
		unstructured.SetNestedField(infraConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(infraConfig.Object, "test://id-1", "spec", "providerID")

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())
		Expect(machine.Status.Addresses).To(HaveLen(0))

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhaseProvisioned))
	})

	It("Should set `Running` when bootstrap, infra, and NodeRef is ready", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		// Set bootstrap ready.
		unstructured.SetNestedField(bootstrapConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(bootstrapConfig.Object, "...", "status", "bootstrapData")

		// Set infra ready.
		unstructured.SetNestedField(infraConfig.Object, "test://id-1", "spec", "providerID")
		unstructured.SetNestedField(infraConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(infraConfig.Object, []interface{}{
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.1",
			},
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.2",
			},
		}, "addresses")

		// Set NodeRef.
		machine.Status.NodeRef = &corev1.ObjectReference{Kind: "Node", Name: "machine-test-node"}

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhaseRunning))
	})

	It("Should set `Deleting` when Machine is being deleted", func() {
		machine := defaultMachine.DeepCopy()
		bootstrapConfig := defaultBootstrap.DeepCopy()
		infraConfig := defaultInfra.DeepCopy()

		// Set bootstrap ready.
		unstructured.SetNestedField(bootstrapConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(bootstrapConfig.Object, "...", "status", "bootstrapData")

		// Set infra ready.
		unstructured.SetNestedField(infraConfig.Object, "test://id-1", "spec", "providerID")
		unstructured.SetNestedField(infraConfig.Object, true, "status", "ready")
		unstructured.SetNestedField(infraConfig.Object, []interface{}{
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.1",
			},
			map[string]interface{}{
				"type":    "InternalIP",
				"address": "10.0.0.2",
			},
		}, "addresses")

		// Set NodeRef.
		machine.Status.NodeRef = &corev1.ObjectReference{Kind: "Node", Name: "machine-test-node"}

		// Set Deletion Timestamp.
		machine.SetDeletionTimestamp(&deletionTimestamp)

		r := &MachineReconciler{
			Client: fake.NewFakeClient(defaultCluster, defaultKubeconfigSecret, machine, bootstrapConfig, infraConfig),
			Log:    log.Log,
		}

		res, err := r.reconcile(context.Background(), defaultCluster, machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

		r.reconcilePhase(machine)
		Expect(machine.Status.GetTypedPhase()).To(Equal(clusterv1.MachinePhaseDeleting))
	})
})

func TestReconcileBootstrap(t *testing.T) {
	defaultMachine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-test",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: "test-cluster",
			},
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
					Kind:       "BootstrapConfig",
					Name:       "bootstrap-config1",
				},
			},
		},
	}

	testCases := []struct {
		name            string
		bootstrapConfig map[string]interface{}
		machine         *clusterv1.Machine
		expectError     bool
		expected        func(g *gomega.WithT, m *clusterv1.Machine)
	}{
		{
			name: "new machine, bootstrap config ready with data",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"ready":         true,
					"bootstrapData": "#!/bin/bash ... data",
				},
			},
			expectError: false,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeTrue())
				g.Expect(m.Spec.Bootstrap.Data).ToNot(gomega.BeNil())
				g.Expect(*m.Spec.Bootstrap.Data).To(gomega.ContainSubstring("#!/bin/bash"))
			},
		},
		{
			name: "new machine, bootstrap config ready with no data",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"ready": true,
				},
			},
			expectError: true,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeFalse())
				g.Expect(m.Spec.Bootstrap.Data).To(gomega.BeNil())
			},
		},
		{
			name: "new machine, bootstrap config not ready",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
			expectError: true,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeFalse())
			},
		},
		{
			name: "new machine, bootstrap config is not found",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "wrong-namespace",
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
			expectError: true,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeFalse())
			},
		},
		{
			name: "new machine, no bootstrap config or data",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "wrong-namespace",
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name: "existing machine, bootstrap data should not change",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"ready": true,
					"data":  "#!/bin/bash ... data with change",
				},
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bootstrap-test-existing",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
							Kind:       "BootstrapConfig",
							Name:       "bootstrap-config1",
						},
						Data: pointer.StringPtr("#!/bin/bash ... data"),
					},
				},
				Status: clusterv1.MachineStatus{
					BootstrapReady: true,
				},
			},
			expectError: false,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeTrue())
				g.Expect(*m.Spec.Bootstrap.Data).To(gomega.Equal("#!/bin/bash ... data"))
			},
		},
		{
			name: "existing machine, bootstrap provider is to not ready",
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"ready": false,
					"data":  "#!/bin/bash ... data",
				},
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bootstrap-test-existing",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
							Kind:       "BootstrapConfig",
							Name:       "bootstrap-config1",
						},
						Data: pointer.StringPtr("#!/bin/bash ... data"),
					},
				},
				Status: clusterv1.MachineStatus{
					BootstrapReady: true,
				},
			},
			expectError: false,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.BootstrapReady).To(gomega.BeTrue())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			if tc.machine == nil {
				tc.machine = defaultMachine.DeepCopy()
			}

			bootstrapConfig := &unstructured.Unstructured{Object: tc.bootstrapConfig}
			r := &MachineReconciler{
				Client: fake.NewFakeClient(tc.machine, bootstrapConfig),
				Log:    log.Log,
			}

			err := r.reconcileBootstrap(context.Background(), tc.machine)
			if tc.expectError {
				g.Expect(err).ToNot(gomega.BeNil())
			} else {
				g.Expect(err).To(gomega.BeNil())
			}

			if tc.expected != nil {
				tc.expected(g, tc.machine)
			}
		})

	}

}

func TestReconcileInfrastructure(t *testing.T) {
	defaultMachine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-test",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.MachineClusterLabelName: "test-cluster",
			},
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
					Kind:       "BootstrapConfig",
					Name:       "bootstrap-config1",
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
				Kind:       "InfrastructureConfig",
				Name:       "infra-config1",
			},
		},
	}

	testCases := []struct {
		name               string
		bootstrapConfig    map[string]interface{}
		infraConfig        map[string]interface{}
		machine            *clusterv1.Machine
		expectError        bool
		expectChanged      bool
		expectRequeueAfter bool
		expected           func(g *gomega.WithT, m *clusterv1.Machine)
	}{
		{
			name: "new machine, infrastructure config ready",
			infraConfig: map[string]interface{}{
				"kind":       "InfrastructureConfig",
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "infra-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"providerID": "test://id-1",
				},
				"status": map[string]interface{}{
					"ready": true,
					"addresses": []interface{}{
						map[string]interface{}{
							"type":    "InternalIP",
							"address": "10.0.0.1",
						},
						map[string]interface{}{
							"type":    "InternalIP",
							"address": "10.0.0.2",
						},
					},
				},
			},
			expectError:   false,
			expectChanged: true,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.InfrastructureReady).To(gomega.BeTrue())
			},
		},
		{
			name: "ready bootstrap, infra, and nodeRef, machine is running, infra object is deleted, expect failed",
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "machine-test",
					Namespace: "default",
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha2",
							Kind:       "BootstrapConfig",
							Name:       "bootstrap-config1",
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
						Kind:       "InfrastructureConfig",
						Name:       "infra-config1",
					},
				},
				Status: clusterv1.MachineStatus{
					BootstrapReady:      true,
					InfrastructureReady: true,
					NodeRef:             &corev1.ObjectReference{Kind: "Node", Name: "machine-test-node"},
				},
			},
			bootstrapConfig: map[string]interface{}{
				"kind":       "BootstrapConfig",
				"apiVersion": "bootstrap.cluster.x-k8s.io/v1alpha2",
				"metadata": map[string]interface{}{
					"name":      "bootstrap-config1",
					"namespace": "default",
				},
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"ready":         true,
					"bootstrapData": "...",
				},
			},
			infraConfig: map[string]interface{}{
				"kind":       "InfrastructureConfig",
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha2",
				"metadata":   map[string]interface{}{},
			},
			expectError:        true,
			expectRequeueAfter: true,
			expected: func(g *gomega.WithT, m *clusterv1.Machine) {
				g.Expect(m.Status.InfrastructureReady).To(gomega.BeTrue())
				g.Expect(m.Status.ErrorMessage).ToNot(gomega.BeNil())
				g.Expect(m.Status.ErrorReason).ToNot(gomega.BeNil())
				g.Expect(m.Status.GetTypedPhase()).To(gomega.Equal(clusterv1.MachinePhaseFailed))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			if tc.machine == nil {
				tc.machine = defaultMachine.DeepCopy()
			}

			infraConfig := &unstructured.Unstructured{Object: tc.infraConfig}
			r := &MachineReconciler{
				Client: fake.NewFakeClient(tc.machine, infraConfig),
				Log:    log.Log,
			}

			err := r.reconcileInfrastructure(context.Background(), tc.machine)
			r.reconcilePhase(tc.machine)
			if tc.expectError {
				g.Expect(err).ToNot(gomega.BeNil())
			} else {
				g.Expect(err).To(gomega.BeNil())
			}

			if tc.expected != nil {
				tc.expected(g, tc.machine)
			}
		})

	}

}
