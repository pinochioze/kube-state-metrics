/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

package collectors

import (
	"k8s.io/kube-state-metrics/pkg/metrics"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var (
	descServiceLabelsName          = "kube_service_labels"
	descServiceLabelsHelp          = "Kubernetes labels converted to Prometheus labels."
	descServiceLabelsDefaultLabels = []string{"namespace", "service"}

	serviceMetricFamilies = []metrics.FamilyGenerator{
		metrics.FamilyGenerator{
			Name: "kube_service_info",
			Help: "Information about service.",
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				m := metrics.Metric{
					Name:        "kube_service_info",
					LabelKeys:   []string{"cluster_ip", "external_name", "load_balancer_ip"},
					LabelValues: []string{s.Spec.ClusterIP, s.Spec.ExternalName, s.Spec.LoadBalancerIP},
					Value:       1,
				}
				return metrics.Family{&m}
			}),
		},
		metrics.FamilyGenerator{
			Name: "kube_service_created",
			Help: "Unix creation timestamp",
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				if !s.CreationTimestamp.IsZero() {
					m := metrics.Metric{
						Name:        "kube_service_created",
						LabelKeys:   nil,
						LabelValues: nil,
						Value:       float64(s.CreationTimestamp.Unix()),
					}
					return metrics.Family{&m}
				}
				return nil
			}),
		},
		metrics.FamilyGenerator{
			Name: "kube_service_spec_type",
			Help: "Type about service.",
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				m := metrics.Metric{
					Name:        "kube_service_spec_type",
					LabelKeys:   []string{"type"},
					LabelValues: []string{string(s.Spec.Type)},
					Value:       1,
				}
				return metrics.Family{&m}
			}),
		},
		metrics.FamilyGenerator{
			Name: descServiceLabelsName,
			Help: descServiceLabelsHelp,
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				labelKeys, labelValues := kubeLabelsToPrometheusLabels(s.Labels)
				m := metrics.Metric{
					Name:        descServiceLabelsName,
					LabelKeys:   labelKeys,
					LabelValues: labelValues,
					Value:       1,
				}
				return metrics.Family{&m}
			}),
		},
		// Defined, but not used anywhere. See
		// https://github.com/kubernetes/kube-state-metrics/pull/571#pullrequestreview-176215628.
		// {
		// 	"kube_service_external_name",
		// 	"Service external name",
		// 	// []string{"type"},
		// },
		// {
		// 	"kube_service_load_balancer_ip",
		// 	"Load balancer IP of service",
		// 	// []string{"load_balancer_ip"},
		// },
		{
			Name: "kube_service_spec_external_ip",
			Help: "Service external ips. One series for each ip",
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				family := []metrics.Family{}

				if len(s.Spec.ExternalIPs) > 0 {
					for _, externalIP := range s.Spec.ExternalIPs {
						m := metrics.Metric{
							Name:        "kube_service_spec_external_ip",
							LabelKeys:   []string{"external_ip"},
							LabelValues: []string{externalIP},
							Value:       1,
						}

						family = append(family, m)
					}
				}

				return family
			}),
		},
		{
			Name: "kube_service_status_load_balancer_ingress",
			Help: "Service load balancer ingress status",
			GenerateFunc: wrapSvcFunc(func(s *v1.Service) metrics.Family {
				family := metrics.Family{}

				if len(s.Status.LoadBalancer.Ingress) > 0 {
					for _, ingress := range s.Status.LoadBalancer.Ingress {
						m := metrics.Metric{
							Name:        "kube_service_status_load_balancer_ingress",
							LabelKeys:   []string{"ip", "hostname"},
							LabelValues: []string{ingress.IP, ingress.Hostname},
							Value:       1,
						}

						family = append(family, m)
					}
				}

				return family
			}),
		},
	}
)

func wrapSvcFunc(f func(*v1.Service) metrics.Family) func(interface{}) metrics.Family {
	return func(obj interface{}) metrics.Family {
		svc := obj.(*v1.Service)

		metricFamily := f(svc)

		for _, m := range metricFamily {
			m.LabelKeys = append(descServiceLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{svc.Namespace, svc.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createServiceListWatch(kubeClient clientset.Interface, ns string) cache.ListWatch {
	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Services(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Services(ns).Watch(opts)
		},
	}
}
